package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/valueiron/vyos-api/handlers"
	"github.com/valueiron/vyos-api/vyos"
	"github.com/gorilla/mux"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		runHealthCheck()
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	deviceMap := parseHosts(os.Getenv("VYOS_HOSTS"))
	h := handlers.New(deviceMap)

	r := mux.NewRouter()
	r.Use(loggingMiddleware)

	// Service endpoints.
	r.HandleFunc("/health", h.Health).Methods(http.MethodGet)
	r.HandleFunc("/devices", h.ListDevices).Methods(http.MethodGet)

	// Networks (interfaces with IPv4).
	r.HandleFunc("/devices/{device_id}/networks", h.ListNetworks).Methods(http.MethodGet)
	r.HandleFunc("/devices/{device_id}/networks", h.CreateNetwork).Methods(http.MethodPost)
	r.HandleFunc("/devices/{device_id}/networks/{interface}", h.GetNetwork).Methods(http.MethodGet)
	r.HandleFunc("/devices/{device_id}/networks/{interface}", h.UpdateNetwork).Methods(http.MethodPut)
	r.HandleFunc("/devices/{device_id}/networks/{interface}", h.DeleteNetwork).Methods(http.MethodDelete)

	// VRFs.
	r.HandleFunc("/devices/{device_id}/vrfs", h.ListVRFs).Methods(http.MethodGet)
	r.HandleFunc("/devices/{device_id}/vrfs", h.CreateVRF).Methods(http.MethodPost)
	r.HandleFunc("/devices/{device_id}/vrfs/{vrf}", h.GetVRF).Methods(http.MethodGet)
	r.HandleFunc("/devices/{device_id}/vrfs/{vrf}", h.UpdateVRF).Methods(http.MethodPut)
	r.HandleFunc("/devices/{device_id}/vrfs/{vrf}", h.DeleteVRF).Methods(http.MethodDelete)

	// VLANs (802.1Q vif subinterfaces).
	r.HandleFunc("/devices/{device_id}/vlans", h.ListVLANs).Methods(http.MethodGet)
	r.HandleFunc("/devices/{device_id}/vlans", h.CreateVLAN).Methods(http.MethodPost)
	r.HandleFunc("/devices/{device_id}/vlans/{interface}/{vlan_id}", h.GetVLAN).Methods(http.MethodGet)
	r.HandleFunc("/devices/{device_id}/vlans/{interface}/{vlan_id}", h.UpdateVLAN).Methods(http.MethodPut)
	r.HandleFunc("/devices/{device_id}/vlans/{interface}/{vlan_id}", h.DeleteVLAN).Methods(http.MethodDelete)

	// Firewall policies and rules.
	r.HandleFunc("/devices/{device_id}/firewall/policies", h.ListPolicies).Methods(http.MethodGet)
	r.HandleFunc("/devices/{device_id}/firewall/policies", h.CreatePolicy).Methods(http.MethodPost)
	r.HandleFunc("/devices/{device_id}/firewall/policies/{policy}", h.GetPolicy).Methods(http.MethodGet)
	r.HandleFunc("/devices/{device_id}/firewall/policies/{policy}", h.UpdatePolicy).Methods(http.MethodPut)
	r.HandleFunc("/devices/{device_id}/firewall/policies/{policy}", h.DeletePolicy).Methods(http.MethodDelete)
	r.HandleFunc("/devices/{device_id}/firewall/policies/{policy}/rules", h.AddRule).Methods(http.MethodPost)
	r.HandleFunc("/devices/{device_id}/firewall/policies/{policy}/rules/{rule_id}", h.DeleteRule).Methods(http.MethodDelete)
	r.HandleFunc("/devices/{device_id}/firewall/policies/{policy}/disable", h.DisablePolicy).Methods(http.MethodPut)
	r.HandleFunc("/devices/{device_id}/firewall/policies/{policy}/enable", h.EnablePolicy).Methods(http.MethodPut)
	r.HandleFunc("/devices/{device_id}/firewall/policies/{policy}/rules/{rule_id}/disable", h.DisableRule).Methods(http.MethodPut)
	r.HandleFunc("/devices/{device_id}/firewall/policies/{policy}/rules/{rule_id}/enable", h.EnableRule).Methods(http.MethodPut)

	// Firewall address groups.
	r.HandleFunc("/devices/{device_id}/firewall/address-groups", h.ListAddressGroups).Methods(http.MethodGet)
	r.HandleFunc("/devices/{device_id}/firewall/address-groups", h.CreateAddressGroup).Methods(http.MethodPost)
	r.HandleFunc("/devices/{device_id}/firewall/address-groups/{group}", h.GetAddressGroup).Methods(http.MethodGet)
	r.HandleFunc("/devices/{device_id}/firewall/address-groups/{group}", h.UpdateAddressGroup).Methods(http.MethodPut)
	r.HandleFunc("/devices/{device_id}/firewall/address-groups/{group}", h.DeleteAddressGroup).Methods(http.MethodDelete)

	addr := ":8082"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("shutdown signal received", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}

// parseHosts parses the VYOS_HOSTS environment variable and returns a device
// map keyed by device ID for use with handlers.New.
//
// Format: name:scheme://host:port:apikey (comma-separated for multiple devices)
// Example: router1:https://192.168.1.1:443:key1,router2:https://10.0.0.1:8443:key2
func parseHosts(hostsEnv string) map[string]*handlers.Device {
	devices := make(map[string]*handlers.Device)
	if hostsEnv == "" {
		return devices
	}

	for _, entry := range strings.Split(hostsEnv, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// SplitN on ":" with n=5 yields:
		//   "router1:https://192.168.1.1:443:key1"
		//   → ["router1", "https", "//192.168.1.1", "443", "key1"]
		parts := strings.SplitN(entry, ":", 5)
		if len(parts) != 5 {
			slog.Warn("skipping invalid VYOS_HOSTS entry (expected name:scheme://host:port:key)", "entry", entry)
			continue
		}
		name := parts[0]
		baseURL := parts[1] + ":" + parts[2] + ":" + parts[3] // e.g. "https://192.168.1.1:443"
		apiKey := parts[4]

		client := vyos.NewClient(nil).WithURL(baseURL).WithToken(apiKey).Insecure()
		devices[name] = &handlers.Device{
			ID:     name,
			URL:    baseURL,
			Client: client,
		}

		slog.Info("registered VyOS device", "name", name, "url", baseURL)
	}

	return devices
}

// responseWriter wraps http.ResponseWriter to capture the status code for logging.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := newResponseWriter(w)
		next.ServeHTTP(rw, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// runHealthCheck performs an HTTP GET against the /health endpoint and exits
// with a non-zero code on failure. Used as the container health probe so that
// the distroless runtime image does not need curl or wget.
func runHealthCheck() {
	port := "8082"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get("http://localhost:" + port + "/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: %v\n", err)
		os.Exit(1)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: unexpected status %d\n", resp.StatusCode)
		os.Exit(1)
	}
}
