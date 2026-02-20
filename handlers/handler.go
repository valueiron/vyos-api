package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/ganawaj/go-vyos/vyos"
	"github.com/gorilla/mux"
)

// Device groups a VyOS client with its registration metadata.
type Device struct {
	ID     string
	URL    string
	Client *vyos.Client
}

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	devices map[string]*Device
}

// New returns a Handler backed by the given device map (keyed by device ID).
func New(devices map[string]*Device) *Handler {
	return &Handler{devices: devices}
}

// getClient extracts the device_id path variable, looks up the client, and
// writes a 404 if not found. Returns (client, true) on success.
func (h *Handler) getClient(w http.ResponseWriter, r *http.Request) (*vyos.Client, bool) {
	id := mux.Vars(r)["device_id"]
	d, ok := h.devices[id]
	if !ok {
		writeError(w, http.StatusNotFound, "device not found: "+id)
		return nil, false
	}
	return d.Client, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
