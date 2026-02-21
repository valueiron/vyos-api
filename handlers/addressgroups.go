package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// AddressGroupInfo is the API representation of a VyOS firewall address group.
type AddressGroupInfo struct {
	Name        string   `json:"name"`
	Addresses   []string `json:"addresses"`
	Description string   `json:"description,omitempty"`
}

// CreateAddressGroupRequest is the JSON body for POST /devices/{device_id}/firewall/address-groups.
type CreateAddressGroupRequest struct {
	Name        string   `json:"name"`
	Addresses   []string `json:"addresses"`
	Description string   `json:"description,omitempty"`
}

// UpdateAddressGroupRequest is the JSON body for PUT /devices/{device_id}/firewall/address-groups/{group}.
// Performs a full replacement of the address list.
type UpdateAddressGroupRequest struct {
	Addresses   []string `json:"addresses"`
	Description string   `json:"description,omitempty"`
}

// ListAddressGroups handles GET /devices/{device_id}/firewall/address-groups.
func (h *Handler) ListAddressGroups(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	out, _, err := c.Conf.Get(r.Context(), "firewall group address-group", nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		// No address groups configured: VyOS returns "Configuration under specified path is empty"
		if strings.Contains(fmt.Sprint(out.Error), "empty") {
			writeJSON(w, http.StatusOK, []AddressGroupInfo{})
			return
		}
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+fmt.Sprint(out.Error))
		return
	}

	// VyOS returns data under the path component key: {"address-group": {"DEMO-NET": {...}, ...}}
	rawMap, _ := out.Data.(map[string]interface{})
	groupMap := rawMap
	if inner, ok := rawMap["address-group"].(map[string]interface{}); ok {
		groupMap = inner
	}
	result := make([]AddressGroupInfo, 0, len(groupMap))
	for name, data := range groupMap {
		result = append(result, parseAddressGroupData(name, data))
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateAddressGroup handles POST /devices/{device_id}/firewall/address-groups.
func (h *Handler) CreateAddressGroup(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	var req CreateAddressGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Add each address member.
	for _, addr := range req.Addresses {
		path := fmt.Sprintf("firewall group address-group %s address %s", req.Name, addr)
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

	// If no addresses provided, create an empty group.
	if len(req.Addresses) == 0 {
		path := fmt.Sprintf("firewall group address-group %s", req.Name)
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
		descPath := fmt.Sprintf("firewall group address-group %s description %s", req.Name, req.Description)
		c.Conf.Set(r.Context(), descPath) //nolint:errcheck
	}

	writeJSON(w, http.StatusCreated, AddressGroupInfo{
		Name:        req.Name,
		Addresses:   req.Addresses,
		Description: req.Description,
	})
}

// GetAddressGroup handles GET /devices/{device_id}/firewall/address-groups/{group}.
func (h *Handler) GetAddressGroup(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	group := mux.Vars(r)["group"]

	out, _, err := c.Conf.Get(r.Context(), fmt.Sprintf("firewall group address-group %s", group), nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusNotFound, "address group not found")
		return
	}

	writeJSON(w, http.StatusOK, parseAddressGroupData(group, out.Data))
}

// UpdateAddressGroup handles PUT /devices/{device_id}/firewall/address-groups/{group}.
// Performs a full replacement of the address list.
func (h *Handler) UpdateAddressGroup(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	group := mux.Vars(r)["group"]

	var req UpdateAddressGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Full replace: delete existing address list then re-add.
	delPath := fmt.Sprintf("firewall group address-group %s address", group)
	c.Conf.Delete(r.Context(), delPath) //nolint:errcheck

	for _, addr := range req.Addresses {
		path := fmt.Sprintf("firewall group address-group %s address %s", group, addr)
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
		descPath := fmt.Sprintf("firewall group address-group %s description %s", group, req.Description)
		c.Conf.Set(r.Context(), descPath) //nolint:errcheck
	}

	writeJSON(w, http.StatusOK, AddressGroupInfo{
		Name:        group,
		Addresses:   req.Addresses,
		Description: req.Description,
	})
}

// DeleteAddressGroup handles DELETE /devices/{device_id}/firewall/address-groups/{group}.
func (h *Handler) DeleteAddressGroup(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	group := mux.Vars(r)["group"]

	out, _, err := c.Conf.Delete(r.Context(), fmt.Sprintf("firewall group address-group %s", group))
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

// parseAddressGroupData converts raw VyOS config data into an AddressGroupInfo.
func parseAddressGroupData(name string, data interface{}) AddressGroupInfo {
	cfg, _ := data.(map[string]interface{})
	addrs := toStringSlice(cfg["address"])
	desc, _ := cfg["description"].(string)
	if addrs == nil {
		addrs = []string{}
	}
	return AddressGroupInfo{
		Name:        name,
		Addresses:   addrs,
		Description: desc,
	}
}
