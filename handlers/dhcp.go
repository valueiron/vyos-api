package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/valueiron/vyos-api/vyos"
)

// DHCPSubnetInfo describes a subnet within a DHCP shared network.
type DHCPSubnetInfo struct {
	Subnet        string   `json:"subnet"`
	DefaultRouter string   `json:"default_router,omitempty"`
	DNSServers    []string `json:"dns_servers,omitempty"`
	RangeStart    string   `json:"range_start,omitempty"`
	RangeStop     string   `json:"range_stop,omitempty"`
	Lease         string   `json:"lease,omitempty"`
}

// DHCPServerInfo is the API representation of a VyOS DHCP shared-network.
type DHCPServerInfo struct {
	Name    string           `json:"name"`
	Subnets []DHCPSubnetInfo `json:"subnets"`
}

// CreateDHCPServerRequest is the JSON body for POST /devices/{device_id}/dhcp/servers.
type CreateDHCPServerRequest struct {
	Name          string   `json:"name"`
	Subnet        string   `json:"subnet"`
	DefaultRouter string   `json:"default_router,omitempty"`
	DNSServers    []string `json:"dns_servers,omitempty"`
	RangeStart    string   `json:"range_start,omitempty"`
	RangeStop     string   `json:"range_stop,omitempty"`
	Lease         string   `json:"lease,omitempty"`
}

// UpdateDHCPServerRequest is the JSON body for PUT /devices/{device_id}/dhcp/servers/{name}.
type UpdateDHCPServerRequest struct {
	Subnet        string   `json:"subnet"`
	DefaultRouter string   `json:"default_router,omitempty"`
	DNSServers    []string `json:"dns_servers,omitempty"`
	RangeStart    string   `json:"range_start,omitempty"`
	RangeStop     string   `json:"range_stop,omitempty"`
	Lease         string   `json:"lease,omitempty"`
}

func dhcpBasePath(name string) string {
	return fmt.Sprintf("service dhcp-server shared-network-name %s", name)
}

func dhcpSubnetPath(name, subnet string) string {
	return fmt.Sprintf("%s subnet %s", dhcpBasePath(name), subnet)
}

// setDHCPSubnetFields applies optional DHCP subnet fields after the subnet node exists.
func setDHCPSubnetFields(ctx context.Context, c *vyos.Client, subnetPath, defaultRouter string, dnsServers []string, rangeStart, rangeStop, lease string) {
	if defaultRouter != "" {
		c.Conf.Set(ctx, fmt.Sprintf("%s default-router %s", subnetPath, defaultRouter)) //nolint:errcheck
	}
	for _, ns := range dnsServers {
		ns = strings.TrimSpace(ns)
		if ns != "" {
			c.Conf.Set(ctx, fmt.Sprintf("%s name-server %s", subnetPath, ns)) //nolint:errcheck
		}
	}
	if rangeStart != "" {
		c.Conf.Set(ctx, fmt.Sprintf("%s range 0 start %s", subnetPath, rangeStart)) //nolint:errcheck
	}
	if rangeStop != "" {
		c.Conf.Set(ctx, fmt.Sprintf("%s range 0 stop %s", subnetPath, rangeStop)) //nolint:errcheck
	}
	if lease != "" {
		c.Conf.Set(ctx, fmt.Sprintf("%s lease %s", subnetPath, lease)) //nolint:errcheck
	}
}

// ListDHCPServers handles GET /devices/{device_id}/dhcp/servers.
func (h *Handler) ListDHCPServers(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	out, _, err := c.Conf.Get(r.Context(), "service dhcp-server shared-network-name", nil)
	if err != nil {
		if strings.Contains(err.Error(), "unexpected status 400") {
			writeJSON(w, http.StatusOK, []DHCPServerInfo{})
			return
		}
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}

	result := []DHCPServerInfo{}
	if out.Success {
		rawMap, _ := out.Data.(map[string]interface{})
		netMap := rawMap
		if inner, ok := rawMap["shared-network-name"].(map[string]interface{}); ok {
			netMap = inner
		}
		for name, nData := range netMap {
			result = append(result, parseDHCPServerData(name, nData))
		}
	} else if !strings.Contains(errMsg(out.Error), "empty") {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateDHCPServer handles POST /devices/{device_id}/dhcp/servers.
func (h *Handler) CreateDHCPServer(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	var req CreateDHCPServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" || req.Subnet == "" {
		writeError(w, http.StatusBadRequest, "name and subnet are required")
		return
	}

	subnetPath := dhcpSubnetPath(req.Name, req.Subnet)

	out, _, err := c.Conf.Set(r.Context(), subnetPath)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}

	setDHCPSubnetFields(r.Context(), c, subnetPath, req.DefaultRouter, req.DNSServers, req.RangeStart, req.RangeStop, req.Lease)

	writeJSON(w, http.StatusCreated, DHCPServerInfo{
		Name: req.Name,
		Subnets: []DHCPSubnetInfo{{
			Subnet:        req.Subnet,
			DefaultRouter: req.DefaultRouter,
			DNSServers:    req.DNSServers,
			RangeStart:    req.RangeStart,
			RangeStop:     req.RangeStop,
			Lease:         req.Lease,
		}},
	})
}

// GetDHCPServer handles GET /devices/{device_id}/dhcp/servers/{name}.
func (h *Handler) GetDHCPServer(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	name := mux.Vars(r)["name"]
	out, _, err := c.Conf.Get(r.Context(), dhcpBasePath(name), nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusNotFound, "DHCP server not found")
		return
	}

	writeJSON(w, http.StatusOK, parseDHCPServerData(name, out.Data))
}

// UpdateDHCPServer handles PUT /devices/{device_id}/dhcp/servers/{name}.
func (h *Handler) UpdateDHCPServer(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	name := mux.Vars(r)["name"]

	var req UpdateDHCPServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Subnet == "" {
		writeError(w, http.StatusBadRequest, "subnet is required")
		return
	}

	subnetPath := dhcpSubnetPath(name, req.Subnet)

	out, _, err := c.Conf.Set(r.Context(), subnetPath)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}

	setDHCPSubnetFields(r.Context(), c, subnetPath, req.DefaultRouter, req.DNSServers, req.RangeStart, req.RangeStop, req.Lease)

	getOut, _, err := c.Conf.Get(r.Context(), dhcpBasePath(name), nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, parseDHCPServerData(name, getOut.Data))
}

// DeleteDHCPServer handles DELETE /devices/{device_id}/dhcp/servers/{name}.
func (h *Handler) DeleteDHCPServer(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	name := mux.Vars(r)["name"]
	out, _, err := c.Conf.Delete(r.Context(), dhcpBasePath(name))
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// parseDHCPServerData converts raw VyOS config data into a DHCPServerInfo.
func parseDHCPServerData(name string, data interface{}) DHCPServerInfo {
	cfg, _ := data.(map[string]interface{})
	subnets := []DHCPSubnetInfo{}

	subnetMap, _ := cfg["subnet"].(map[string]interface{})
	for subnet, sData := range subnetMap {
		sCfg, _ := sData.(map[string]interface{})
		info := DHCPSubnetInfo{Subnet: subnet}

		info.DefaultRouter, _ = sCfg["default-router"].(string)
		info.Lease, _ = sCfg["lease"].(string)

		// DNS servers: VyOS may return a single string or a list.
		switch v := sCfg["name-server"].(type) {
		case string:
			info.DNSServers = []string{v}
		case []interface{}:
			for _, ns := range v {
				if s, ok := ns.(string); ok {
					info.DNSServers = append(info.DNSServers, s)
				}
			}
		}

		// Range: VyOS stores as {"0": {"start": "...", "stop": "..."}}
		if rangeMap, ok := sCfg["range"].(map[string]interface{}); ok {
			for _, rData := range rangeMap {
				if rCfg, ok := rData.(map[string]interface{}); ok {
					info.RangeStart, _ = rCfg["start"].(string)
					info.RangeStop, _ = rCfg["stop"].(string)
				}
				break // use first range entry
			}
		}

		subnets = append(subnets, info)
	}

	return DHCPServerInfo{Name: name, Subnets: subnets}
}
