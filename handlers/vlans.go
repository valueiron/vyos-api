package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// VLANInfo is the API representation of a VyOS 802.1Q vif subinterface.
type VLANInfo struct {
	Interface   string   `json:"interface"`
	Type        string   `json:"type"`
	VLANID      int      `json:"vlan_id"`
	Addresses   []string `json:"addresses"`
	Description string   `json:"description,omitempty"`
}

// CreateVLANRequest is the JSON body for POST /devices/{device_id}/vlans.
type CreateVLANRequest struct {
	Interface   string `json:"interface"`
	Type        string `json:"type"`
	VLANID      int    `json:"vlan_id"`
	Address     string `json:"address,omitempty"`
	Description string `json:"description,omitempty"`
}

// UpdateVLANRequest is the JSON body for PUT /devices/{device_id}/vlans/{interface}/{vlan_id}.
type UpdateVLANRequest struct {
	Type        string `json:"type"`
	Address     string `json:"address,omitempty"`
	Description string `json:"description,omitempty"`
}

// ListVLANs handles GET /devices/{device_id}/vlans.
func (h *Handler) ListVLANs(w http.ResponseWriter, r *http.Request) {
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
	result := make([]VLANInfo, 0)

	for ifType, ifData := range ifaceMap {
		ifaces, _ := ifData.(map[string]interface{})
		for ifName, ifCfg := range ifaces {
			cfg, _ := ifCfg.(map[string]interface{})
			vifMap, ok := cfg["vif"].(map[string]interface{})
			if !ok {
				continue
			}
			for vlanIDStr, vifData := range vifMap {
				vlanID, err := strconv.Atoi(vlanIDStr)
				if err != nil {
					continue
				}
				vifCfg, _ := vifData.(map[string]interface{})
				addrs := toStringSlice(vifCfg["address"])
				desc, _ := vifCfg["description"].(string)
				result = append(result, VLANInfo{
					Interface:   ifName,
					Type:        ifType,
					VLANID:      vlanID,
					Addresses:   addrs,
					Description: desc,
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateVLAN handles POST /devices/{device_id}/vlans.
func (h *Handler) CreateVLAN(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	var req CreateVLANRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Interface == "" || req.Type == "" || req.VLANID == 0 {
		writeError(w, http.StatusBadRequest, "interface, type, and vlan_id are required")
		return
	}

	// Create the vif subinterface.
	if req.Address != "" {
		path := fmt.Sprintf("interfaces %s %s vif %d address %s", req.Type, req.Interface, req.VLANID, req.Address)
		out, _, err := c.Conf.Set(r.Context(), path)
		if err != nil {
			writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
			return
		}
		if !out.Success {
			writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+fmt.Sprint(out.Error))
			return
		}
	} else {
		// Create vif without address.
		path := fmt.Sprintf("interfaces %s %s vif %d", req.Type, req.Interface, req.VLANID)
		out, _, err := c.Conf.Set(r.Context(), path)
		if err != nil {
			writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
			return
		}
		if !out.Success {
			writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+fmt.Sprint(out.Error))
			return
		}
	}

	if req.Description != "" {
		descPath := fmt.Sprintf("interfaces %s %s vif %d description %s", req.Type, req.Interface, req.VLANID, req.Description)
		c.Conf.Set(r.Context(), descPath) //nolint:errcheck
	}

	addrs := []string{}
	if req.Address != "" {
		addrs = []string{req.Address}
	}

	writeJSON(w, http.StatusCreated, VLANInfo{
		Interface:   req.Interface,
		Type:        req.Type,
		VLANID:      req.VLANID,
		Addresses:   addrs,
		Description: req.Description,
	})
}

// GetVLAN handles GET /devices/{device_id}/vlans/{interface}/{vlan_id}.
func (h *Handler) GetVLAN(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	iface := vars["interface"]
	vlanIDStr := vars["vlan_id"]
	vlanID, err := strconv.Atoi(vlanIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "vlan_id must be an integer")
		return
	}
	ifType := r.URL.Query().Get("type")
	if ifType == "" {
		ifType = "ethernet"
	}

	path := fmt.Sprintf("interfaces %s %s vif %d", ifType, iface, vlanID)
	out, _, err := c.Conf.Get(r.Context(), path, nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusNotFound, "VLAN not found")
		return
	}

	cfg, _ := out.Data.(map[string]interface{})
	addrs := toStringSlice(cfg["address"])
	desc, _ := cfg["description"].(string)

	writeJSON(w, http.StatusOK, VLANInfo{
		Interface:   iface,
		Type:        ifType,
		VLANID:      vlanID,
		Addresses:   addrs,
		Description: desc,
	})
}

// UpdateVLAN handles PUT /devices/{device_id}/vlans/{interface}/{vlan_id}.
func (h *Handler) UpdateVLAN(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	iface := vars["interface"]
	vlanIDStr := vars["vlan_id"]
	vlanID, err := strconv.Atoi(vlanIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "vlan_id must be an integer")
		return
	}

	var req UpdateVLANRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Type == "" {
		req.Type = "ethernet"
	}

	if req.Address != "" {
		// Replace existing addresses.
		delPath := fmt.Sprintf("interfaces %s %s vif %d address", req.Type, iface, vlanID)
		c.Conf.Delete(r.Context(), delPath) //nolint:errcheck

		setPath := fmt.Sprintf("interfaces %s %s vif %d address %s", req.Type, iface, vlanID, req.Address)
		out, _, err := c.Conf.Set(r.Context(), setPath)
		if err != nil {
			writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
			return
		}
		if !out.Success {
			writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+fmt.Sprint(out.Error))
			return
		}
	}

	if req.Description != "" {
		descPath := fmt.Sprintf("interfaces %s %s vif %d description %s", req.Type, iface, vlanID, req.Description)
		c.Conf.Set(r.Context(), descPath) //nolint:errcheck
	}

	addrs := []string{}
	if req.Address != "" {
		addrs = []string{req.Address}
	}

	writeJSON(w, http.StatusOK, VLANInfo{
		Interface:   iface,
		Type:        req.Type,
		VLANID:      vlanID,
		Addresses:   addrs,
		Description: req.Description,
	})
}

// DeleteVLAN handles DELETE /devices/{device_id}/vlans/{interface}/{vlan_id}?type=ethernet.
func (h *Handler) DeleteVLAN(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	iface := vars["interface"]
	vlanIDStr := vars["vlan_id"]
	vlanID, err := strconv.Atoi(vlanIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "vlan_id must be an integer")
		return
	}
	ifType := r.URL.Query().Get("type")
	if ifType == "" {
		ifType = "ethernet"
	}

	path := fmt.Sprintf("interfaces %s %s vif %d", ifType, iface, vlanID)
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
