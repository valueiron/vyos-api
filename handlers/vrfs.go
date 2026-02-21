package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

// VRFInfo is the API representation of a VyOS VRF.
type VRFInfo struct {
	Name        string `json:"name"`
	Table       string `json:"table"`
	Description string `json:"description,omitempty"`
}

// CreateVRFRequest is the JSON body for POST /devices/{device_id}/vrfs.
type CreateVRFRequest struct {
	Name        string `json:"name"`
	Table       string `json:"table"`
	Description string `json:"description,omitempty"`
}

// UpdateVRFRequest is the JSON body for PUT /devices/{device_id}/vrfs/{vrf}.
type UpdateVRFRequest struct {
	Table       string `json:"table,omitempty"`
	Description string `json:"description,omitempty"`
}

// ListVRFs handles GET /devices/{device_id}/vrfs.
func (h *Handler) ListVRFs(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	out, _, err := c.Conf.Get(r.Context(), "vrf name", nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+fmt.Sprint(out.Error))
		return
	}

	// VyOS returns data under the path component key: {"name": {"vrf-BLUE": {"table": "100"}, ...}}
	rawMap, _ := out.Data.(map[string]interface{})
	vrfMap := rawMap
	if inner, ok := rawMap["name"].(map[string]interface{}); ok {
		vrfMap = inner
	}
	result := make([]VRFInfo, 0, len(vrfMap))
	for name, data := range vrfMap {
		cfg, _ := data.(map[string]interface{})
		table, _ := cfg["table"].(string)
		desc, _ := cfg["description"].(string)
		result = append(result, VRFInfo{
			Name:        name,
			Table:       table,
			Description: desc,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateVRF handles POST /devices/{device_id}/vrfs.
func (h *Handler) CreateVRF(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	var req CreateVRFRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" || req.Table == "" {
		writeError(w, http.StatusBadRequest, "name and table are required")
		return
	}

	path := fmt.Sprintf("vrf name %s table %s", req.Name, req.Table)
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
		descPath := fmt.Sprintf("vrf name %s description %s", req.Name, req.Description)
		c.Conf.Set(r.Context(), descPath) //nolint:errcheck
	}

	writeJSON(w, http.StatusCreated, VRFInfo{
		Name:        req.Name,
		Table:       req.Table,
		Description: req.Description,
	})
}

// GetVRF handles GET /devices/{device_id}/vrfs/{vrf}.
func (h *Handler) GetVRF(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	vrfName := mux.Vars(r)["vrf"]

	out, _, err := c.Conf.Get(r.Context(), fmt.Sprintf("vrf name %s", vrfName), nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusNotFound, "VRF not found")
		return
	}

	cfg, _ := out.Data.(map[string]interface{})
	table, _ := cfg["table"].(string)
	desc, _ := cfg["description"].(string)

	writeJSON(w, http.StatusOK, VRFInfo{
		Name:        vrfName,
		Table:       table,
		Description: desc,
	})
}

// UpdateVRF handles PUT /devices/{device_id}/vrfs/{vrf}.
func (h *Handler) UpdateVRF(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	vrfName := mux.Vars(r)["vrf"]

	var req UpdateVRFRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Table != "" {
		path := fmt.Sprintf("vrf name %s table %s", vrfName, req.Table)
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
		descPath := fmt.Sprintf("vrf name %s description %s", vrfName, req.Description)
		out, _, err := c.Conf.Set(r.Context(), descPath)
		if err != nil {
			writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
			return
		}
		if !out.Success {
			writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+fmt.Sprint(out.Error))
			return
		}
	}

	// Return updated state.
	out, _, err := c.Conf.Get(r.Context(), fmt.Sprintf("vrf name %s", vrfName), nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	cfg, _ := out.Data.(map[string]interface{})
	table, _ := cfg["table"].(string)
	desc, _ := cfg["description"].(string)

	writeJSON(w, http.StatusOK, VRFInfo{
		Name:        vrfName,
		Table:       table,
		Description: desc,
	})
}

// DeleteVRF handles DELETE /devices/{device_id}/vrfs/{vrf}.
func (h *Handler) DeleteVRF(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	vrfName := mux.Vars(r)["vrf"]

	out, _, err := c.Conf.Delete(r.Context(), fmt.Sprintf("vrf name %s", vrfName))
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
