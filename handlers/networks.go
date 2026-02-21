package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

// NetworkInfo is the API representation of a VyOS interface with IPv4 addresses.
type NetworkInfo struct {
	Interface   string   `json:"interface"`
	Type        string   `json:"type"`
	Addresses   []string `json:"addresses"`
	Description string   `json:"description,omitempty"`
}

// CreateNetworkRequest is the JSON body for POST /devices/{device_id}/networks.
type CreateNetworkRequest struct {
	Interface   string `json:"interface"`
	Type        string `json:"type"`
	Address     string `json:"address"`
	Description string `json:"description,omitempty"`
}

// UpdateNetworkRequest is the JSON body for PUT /devices/{device_id}/networks/{interface}.
type UpdateNetworkRequest struct {
	Type        string `json:"type"`
	Address     string `json:"address"`
	Description string `json:"description,omitempty"`
}

// toStringSlice normalises a VyOS config value that may be a single string or
// a JSON array ([]interface{}) into a []string. VyOS returns a bare string
// when there is only one value and an array when there are multiple.
// Always returns a non-nil slice so that JSON responses encode as [] not null.
func toStringSlice(v interface{}) []string {
	switch val := v.(type) {
	case string:
		return []string{val}
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return []string{}
	}
}

// ListNetworks handles GET /devices/{device_id}/networks.
func (h *Handler) ListNetworks(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	out, _, err := c.Conf.Get(r.Context(), "interfaces", nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+fmt.Sprint(out.Error))
		return
	}

	ifaceMap, _ := out.Data.(map[string]interface{})
	result := make([]NetworkInfo, 0)

	for ifType, ifData := range ifaceMap {
		ifaces, _ := ifData.(map[string]interface{})
		for ifName, ifCfg := range ifaces {
			cfg, _ := ifCfg.(map[string]interface{})
			addrs := toStringSlice(cfg["address"])
			desc, _ := cfg["description"].(string)
			result = append(result, NetworkInfo{
				Interface:   ifName,
				Type:        ifType,
				Addresses:   addrs,
				Description: desc,
			})
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateNetwork handles POST /devices/{device_id}/networks.
func (h *Handler) CreateNetwork(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	var req CreateNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Interface == "" || req.Type == "" || req.Address == "" {
		writeError(w, http.StatusBadRequest, "interface, type, and address are required")
		return
	}

	path := fmt.Sprintf("interfaces %s %s address %s", req.Type, req.Interface, req.Address)
	out, _, err := c.Conf.Set(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+fmt.Sprint(out.Error))
		return
	}

	if req.Description != "" {
		descPath := fmt.Sprintf("interfaces %s %s description %s", req.Type, req.Interface, req.Description)
		if out2, _, err2 := c.Conf.Set(r.Context(), descPath); err2 != nil || !out2.Success {
			// non-fatal: address was set successfully
		}
	}

	writeJSON(w, http.StatusCreated, NetworkInfo{
		Interface:   req.Interface,
		Type:        req.Type,
		Addresses:   []string{req.Address},
		Description: req.Description,
	})
}

// GetNetwork handles GET /devices/{device_id}/networks/{interface}.
func (h *Handler) GetNetwork(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	iface := vars["interface"]
	ifType := r.URL.Query().Get("type")
	if ifType == "" {
		ifType = "ethernet"
	}

	path := fmt.Sprintf("interfaces %s %s", ifType, iface)
	out, _, err := c.Conf.Get(r.Context(), path, nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusNotFound, "interface not found")
		return
	}

	cfg, _ := out.Data.(map[string]interface{})
	addrs := toStringSlice(cfg["address"])
	desc, _ := cfg["description"].(string)

	writeJSON(w, http.StatusOK, NetworkInfo{
		Interface:   iface,
		Type:        ifType,
		Addresses:   addrs,
		Description: desc,
	})
}

// UpdateNetwork handles PUT /devices/{device_id}/networks/{interface}.
func (h *Handler) UpdateNetwork(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	iface := vars["interface"]

	var req UpdateNetworkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Type == "" || req.Address == "" {
		writeError(w, http.StatusBadRequest, "type and address are required")
		return
	}

	// Delete existing address block then set the new one.
	delPath := fmt.Sprintf("interfaces %s %s address", req.Type, iface)
	if _, _, err := c.Conf.Delete(r.Context(), delPath); err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}

	setPath := fmt.Sprintf("interfaces %s %s address %s", req.Type, iface, req.Address)
	out, _, err := c.Conf.Set(r.Context(), setPath)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+fmt.Sprint(out.Error))
		return
	}

	if req.Description != "" {
		descPath := fmt.Sprintf("interfaces %s %s description %s", req.Type, iface, req.Description)
		c.Conf.Set(r.Context(), descPath) //nolint:errcheck
	}

	writeJSON(w, http.StatusOK, NetworkInfo{
		Interface:   iface,
		Type:        req.Type,
		Addresses:   []string{req.Address},
		Description: req.Description,
	})
}

// DeleteNetwork handles DELETE /devices/{device_id}/networks/{interface}?type=ethernet.
func (h *Handler) DeleteNetwork(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	iface := vars["interface"]
	ifType := r.URL.Query().Get("type")
	if ifType == "" {
		ifType = "ethernet"
	}

	path := fmt.Sprintf("interfaces %s %s", ifType, iface)
	out, _, err := c.Conf.Delete(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+fmt.Sprint(out.Error))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
