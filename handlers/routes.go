package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// RouteInfo is the API representation of a VyOS static route.
type RouteInfo struct {
	Network     string `json:"network"`
	NextHop     string `json:"next_hop"`
	Distance    string `json:"distance,omitempty"`
	Description string `json:"description,omitempty"`
}

// CreateRouteRequest is the JSON body for POST /devices/{device_id}/routes.
type CreateRouteRequest struct {
	Network     string `json:"network"`
	NextHop     string `json:"next_hop"`
	Distance    string `json:"distance,omitempty"`
	Description string `json:"description,omitempty"`
}

// UpdateRouteRequest is the JSON body for PUT /devices/{device_id}/routes/{prefix}/{mask}.
type UpdateRouteRequest struct {
	NextHop     string `json:"next_hop,omitempty"`
	Distance    string `json:"distance,omitempty"`
	Description string `json:"description,omitempty"`
}

func routeNetwork(vars map[string]string) string {
	return vars["prefix"] + "/" + vars["mask"]
}

func routeBasePath(network string) string {
	return fmt.Sprintf("protocols static route %s", network)
}

// ListRoutes handles GET /devices/{device_id}/routes.
func (h *Handler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	out, _, err := c.Conf.Get(r.Context(), "protocols static route", nil)
	if err != nil {
		if strings.Contains(err.Error(), "unexpected status 400") {
			writeJSON(w, http.StatusOK, []RouteInfo{})
			return
		}
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}

	result := []RouteInfo{}
	if out.Success {
		rawMap, _ := out.Data.(map[string]interface{})
		routeMap := rawMap
		if inner, ok := rawMap["route"].(map[string]interface{}); ok {
			routeMap = inner
		}
		for network, rData := range routeMap {
			result = append(result, parseRouteData(network, rData))
		}
	} else if !strings.Contains(errMsg(out.Error), "empty") {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateRoute handles POST /devices/{device_id}/routes.
func (h *Handler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	var req CreateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Network == "" || req.NextHop == "" {
		writeError(w, http.StatusBadRequest, "network and next_hop are required")
		return
	}

	base := routeBasePath(req.Network)
	nhPath := fmt.Sprintf("%s next-hop %s", base, req.NextHop)

	out, _, err := c.Conf.Set(r.Context(), nhPath)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}

	if req.Distance != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s distance %s", nhPath, req.Distance)) //nolint:errcheck
	}
	if req.Description != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s description %s", base, req.Description)) //nolint:errcheck
	}

	writeJSON(w, http.StatusCreated, RouteInfo{
		Network:     req.Network,
		NextHop:     req.NextHop,
		Distance:    req.Distance,
		Description: req.Description,
	})
}

// GetRoute handles GET /devices/{device_id}/routes/{prefix}/{mask}.
func (h *Handler) GetRoute(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	network := routeNetwork(mux.Vars(r))
	out, _, err := c.Conf.Get(r.Context(), routeBasePath(network), nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}

	writeJSON(w, http.StatusOK, parseRouteData(network, out.Data))
}

// UpdateRoute handles PUT /devices/{device_id}/routes/{prefix}/{mask}.
func (h *Handler) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	network := routeNetwork(mux.Vars(r))
	base := routeBasePath(network)

	var req UpdateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.NextHop != "" {
		nhPath := fmt.Sprintf("%s next-hop %s", base, req.NextHop)
		out, _, err := c.Conf.Set(r.Context(), nhPath)
		if err != nil {
			writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
			return
		}
		if !out.Success {
			writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
			return
		}
		if req.Distance != "" {
			c.Conf.Set(r.Context(), fmt.Sprintf("%s distance %s", nhPath, req.Distance)) //nolint:errcheck
		}
	}
	if req.Description != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s description %s", base, req.Description)) //nolint:errcheck
	}

	out, _, err := c.Conf.Get(r.Context(), base, nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, parseRouteData(network, out.Data))
}

// DeleteRoute handles DELETE /devices/{device_id}/routes/{prefix}/{mask}.
func (h *Handler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	network := routeNetwork(mux.Vars(r))
	out, _, err := c.Conf.Delete(r.Context(), routeBasePath(network))
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

// parseRouteData converts raw VyOS config data into a RouteInfo.
func parseRouteData(network string, data interface{}) RouteInfo {
	cfg, _ := data.(map[string]interface{})
	desc, _ := cfg["description"].(string)

	var nextHop, distance string
	if nhMap, ok := cfg["next-hop"].(map[string]interface{}); ok {
		for addr, nhData := range nhMap {
			nextHop = addr
			if nhCfg, ok := nhData.(map[string]interface{}); ok {
				if d, ok := nhCfg["distance"].(string); ok {
					distance = d
				}
			}
			break // use first next-hop
		}
	}

	return RouteInfo{
		Network:     network,
		NextHop:     nextHop,
		Distance:    distance,
		Description: desc,
	}
}
