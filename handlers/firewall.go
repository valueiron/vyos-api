package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

// errMsg returns a string form of the VyOS API error field (which may be string or other).
func errMsg(e interface{}) string { return fmt.Sprint(e) }

// PolicyInfo is the API representation of a VyOS IPv4 firewall policy.
type PolicyInfo struct {
	Name          string              `json:"name"`
	DefaultAction string              `json:"default_action"`
	Description   string              `json:"description,omitempty"`
	Disabled      bool                `json:"disabled,omitempty"`
	Rules         map[string]RuleInfo `json:"rules,omitempty"`
}

// RuleInfo is the API representation of a firewall rule.
type RuleInfo struct {
	Action           string `json:"action"`
	Source           string `json:"source,omitempty"`
	SourceGroup      string `json:"source_group,omitempty"`
	Destination      string `json:"destination,omitempty"`
	DestinationGroup string `json:"destination_group,omitempty"`
	Description      string `json:"description,omitempty"`
	Disabled         bool   `json:"disabled,omitempty"`
}

// CreatePolicyRequest is the JSON body for POST /devices/{device_id}/firewall/policies.
type CreatePolicyRequest struct {
	Name          string `json:"name"`
	DefaultAction string `json:"default_action"`
	Description   string `json:"description,omitempty"`
}

// UpdatePolicyRequest is the JSON body for PUT /devices/{device_id}/firewall/policies/{policy}.
type UpdatePolicyRequest struct {
	DefaultAction string `json:"default_action,omitempty"`
	Description   string `json:"description,omitempty"`
}

// AddRuleRequest is the JSON body for POST /devices/{device_id}/firewall/policies/{policy}/rules.
type AddRuleRequest struct {
	RuleID           int    `json:"rule_id"`
	Action           string `json:"action"`
	Source           string `json:"source,omitempty"`
	SourceGroup      string `json:"source_group,omitempty"`
	Destination      string `json:"destination,omitempty"`
	DestinationGroup string `json:"destination_group,omitempty"`
	Description      string `json:"description,omitempty"`
}

// Base chain path suffixes (firewall ipv4 <dir> filter).
var baseChainPaths = []struct {
	name string
	path string
}{
	{"forward", "firewall ipv4 forward filter"},
	{"input", "firewall ipv4 input filter"},
	{"output", "firewall ipv4 output filter"},
}

// ListPolicies handles GET /devices/{device_id}/firewall/policies.
// Returns named policies (firewall ipv4 name X) plus base chains (forward, input, output) that have rules.
func (h *Handler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	var result []PolicyInfo

	// Named policies under firewall ipv4 name
	out, _, err := c.Conf.Get(r.Context(), "firewall ipv4 name", nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if out.Success {
		rawMap, _ := out.Data.(map[string]interface{})
		policyMap := rawMap
		if inner, ok := rawMap["name"].(map[string]interface{}); ok {
			policyMap = inner
		}
		for name, data := range policyMap {
			result = append(result, parsePolicyData(name, data))
		}
	} else if !strings.Contains(errMsg(out.Error), "empty") {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}

	// Base chains (forward, input, output) — include if they have config
	for _, bc := range baseChainPaths {
		out2, _, err2 := c.Conf.Get(r.Context(), bc.path, nil)
		if err2 != nil || !out2.Success {
			continue
		}
		rawMap, _ := out2.Data.(map[string]interface{})
		data := rawMap
		if inner, ok := rawMap["filter"].(map[string]interface{}); ok {
			data = inner
		}
		if hasPolicyContent(data) {
			result = append(result, parsePolicyData(bc.name, data))
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// hasPolicyContent returns true if the map has rule or default-action (so we treat it as a policy).
func hasPolicyContent(cfg map[string]interface{}) bool {
	if cfg == nil {
		return false
	}
	if _, ok := cfg["rule"].(map[string]interface{}); ok {
		return true
	}
	if _, ok := cfg["default-action"].(string); ok {
		return true
	}
	return false
}

// CreatePolicy handles POST /devices/{device_id}/firewall/policies.
func (h *Handler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	var req CreatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" || req.DefaultAction == "" {
		writeError(w, http.StatusBadRequest, "name and default_action are required")
		return
	}

	path := fmt.Sprintf("firewall ipv4 name %s default-action %s", req.Name, req.DefaultAction)
	out, _, err := c.Conf.Set(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}

	if req.Description != "" {
		descPath := fmt.Sprintf("firewall ipv4 name %s description %s", req.Name, req.Description)
		c.Conf.Set(r.Context(), descPath) //nolint:errcheck
	}

	writeJSON(w, http.StatusCreated, PolicyInfo{
		Name:          req.Name,
		DefaultAction: req.DefaultAction,
		Description:   req.Description,
	})
}

// GetPolicy handles GET /devices/{device_id}/firewall/policies/{policy}.
// The response includes all rules for the policy. Supports named policies and base chains (forward, input, output).
func (h *Handler) GetPolicy(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	policy := mux.Vars(r)["policy"]

	var path string
	for _, bc := range baseChainPaths {
		if bc.name == policy {
			path = bc.path
			break
		}
	}
	if path == "" {
		path = fmt.Sprintf("firewall ipv4 name %s", policy)
	}

	out, _, err := c.Conf.Get(r.Context(), path, nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusNotFound, "policy not found")
		return
	}

	data := out.Data
	if rawMap, ok := out.Data.(map[string]interface{}); ok {
		if inner, ok := rawMap["filter"].(map[string]interface{}); ok {
			data = inner
		} else if inner, ok := rawMap["name"].(map[string]interface{}); ok {
			// single named policy get might return {"name": {"POLICY": {...}}} in some versions
			if policyData, ok := inner[policy].(map[string]interface{}); ok {
				data = policyData
			}
		}
	}

	writeJSON(w, http.StatusOK, parsePolicyData(policy, data))
}

// UpdatePolicy handles PUT /devices/{device_id}/firewall/policies/{policy}.
func (h *Handler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	policy := mux.Vars(r)["policy"]

	var req UpdatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.DefaultAction != "" {
		path := fmt.Sprintf("firewall ipv4 name %s default-action %s", policy, req.DefaultAction)
		out, _, err := c.Conf.Set(r.Context(), path)
		if err != nil {
			writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
			return
		}
		if !out.Success {
			writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
			return
		}
	}

	if req.Description != "" {
		descPath := fmt.Sprintf("firewall ipv4 name %s description %s", policy, req.Description)
		c.Conf.Set(r.Context(), descPath) //nolint:errcheck
	}

	// Return updated state.
	out, _, err := c.Conf.Get(r.Context(), fmt.Sprintf("firewall ipv4 name %s", policy), nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, parsePolicyData(policy, out.Data))
}

// DeletePolicy handles DELETE /devices/{device_id}/firewall/policies/{policy}.
func (h *Handler) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	policy := mux.Vars(r)["policy"]

	out, _, err := c.Conf.Delete(r.Context(), fmt.Sprintf("firewall ipv4 name %s", policy))
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

// AddRule handles POST /devices/{device_id}/firewall/policies/{policy}/rules.
func (h *Handler) AddRule(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	policy := mux.Vars(r)["policy"]

	var req AddRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.RuleID == 0 || req.Action == "" {
		writeError(w, http.StatusBadRequest, "rule_id and action are required")
		return
	}

	// Set action.
	path := fmt.Sprintf("firewall ipv4 name %s rule %d action %s", policy, req.RuleID, req.Action)
	out, _, err := c.Conf.Set(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}

	if req.Source != "" {
		srcPath := fmt.Sprintf("firewall ipv4 name %s rule %d source address %s", policy, req.RuleID, req.Source)
		c.Conf.Set(r.Context(), srcPath) //nolint:errcheck
	} else if req.SourceGroup != "" {
		srcPath := fmt.Sprintf("firewall ipv4 name %s rule %d source group address-group %s", policy, req.RuleID, req.SourceGroup)
		c.Conf.Set(r.Context(), srcPath) //nolint:errcheck
	}

	if req.Destination != "" {
		dstPath := fmt.Sprintf("firewall ipv4 name %s rule %d destination address %s", policy, req.RuleID, req.Destination)
		c.Conf.Set(r.Context(), dstPath) //nolint:errcheck
	} else if req.DestinationGroup != "" {
		dstPath := fmt.Sprintf("firewall ipv4 name %s rule %d destination group address-group %s", policy, req.RuleID, req.DestinationGroup)
		c.Conf.Set(r.Context(), dstPath) //nolint:errcheck
	}

	if req.Description != "" {
		descPath := fmt.Sprintf("firewall ipv4 name %s rule %d description %s", policy, req.RuleID, req.Description)
		c.Conf.Set(r.Context(), descPath) //nolint:errcheck
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"policy":            policy,
		"rule_id":           req.RuleID,
		"action":            req.Action,
		"source":            req.Source,
		"source_group":      req.SourceGroup,
		"destination":       req.Destination,
		"destination_group": req.DestinationGroup,
		"description":       req.Description,
	})
}

// DeleteRule handles DELETE /devices/{device_id}/firewall/policies/{policy}/rules/{rule_id}.
func (h *Handler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}

	vars := mux.Vars(r)
	policy := vars["policy"]
	ruleIDStr := vars["rule_id"]
	ruleID, err := strconv.Atoi(ruleIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "rule_id must be an integer")
		return
	}

	path := fmt.Sprintf("firewall ipv4 name %s rule %d", policy, ruleID)
	out, _, err := c.Conf.Delete(r.Context(), path)
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

// parsePolicyData converts raw VyOS config data into a PolicyInfo.
func parsePolicyData(name string, data interface{}) PolicyInfo {
	cfg, _ := data.(map[string]interface{})
	defaultAction, _ := cfg["default-action"].(string)
	desc, _ := cfg["description"].(string)
	_, policyDisabled := cfg["disable"]

	rules := make(map[string]RuleInfo)
	if ruleMap, ok := cfg["rule"].(map[string]interface{}); ok {
		for ruleID, ruleData := range ruleMap {
			ruleCfg, _ := ruleData.(map[string]interface{})
			action, _ := ruleCfg["action"].(string)
			ruleDesc, _ := ruleCfg["description"].(string)
			_, ruleDisabled := ruleCfg["disable"]

			var srcAddr, srcGroup, dstAddr, dstGroup string
			if src, ok := ruleCfg["source"].(map[string]interface{}); ok {
				srcAddr, _ = src["address"].(string)
				if grp, ok := src["group"].(map[string]interface{}); ok {
					srcGroup, _ = grp["address-group"].(string)
				}
			}
			if dst, ok := ruleCfg["destination"].(map[string]interface{}); ok {
				dstAddr, _ = dst["address"].(string)
				if grp, ok := dst["group"].(map[string]interface{}); ok {
					dstGroup, _ = grp["address-group"].(string)
				}
			}

			rules[ruleID] = RuleInfo{
				Action:           action,
				Source:           srcAddr,
				SourceGroup:      srcGroup,
				Destination:      dstAddr,
				DestinationGroup: dstGroup,
				Description:      ruleDesc,
				Disabled:         ruleDisabled,
			}
		}
	}

	return PolicyInfo{
		Name:          name,
		DefaultAction: defaultAction,
		Description:   desc,
		Disabled:      policyDisabled,
		Rules:         rules,
	}
}

// DisablePolicy handles PUT /devices/{device_id}/firewall/policies/{policy}/disable.
func (h *Handler) DisablePolicy(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}
	policy := mux.Vars(r)["policy"]
	path := fmt.Sprintf("firewall ipv4 name %s disable", policy)
	out, _, err := c.Conf.Set(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"disabled": true})
}

// EnablePolicy handles PUT /devices/{device_id}/firewall/policies/{policy}/enable.
func (h *Handler) EnablePolicy(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}
	policy := mux.Vars(r)["policy"]
	path := fmt.Sprintf("firewall ipv4 name %s disable", policy)
	out, _, err := c.Conf.Delete(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"disabled": false})
}

// DisableRule handles PUT /devices/{device_id}/firewall/policies/{policy}/rules/{rule_id}/disable.
func (h *Handler) DisableRule(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}
	vars := mux.Vars(r)
	policy := vars["policy"]
	ruleID, err := strconv.Atoi(vars["rule_id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "rule_id must be an integer")
		return
	}
	path := fmt.Sprintf("firewall ipv4 name %s rule %d disable", policy, ruleID)
	out, _, err := c.Conf.Set(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"disabled": true})
}

// EnableRule handles PUT /devices/{device_id}/firewall/policies/{policy}/rules/{rule_id}/enable.
func (h *Handler) EnableRule(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}
	vars := mux.Vars(r)
	policy := vars["policy"]
	ruleID, err := strconv.Atoi(vars["rule_id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "rule_id must be an integer")
		return
	}
	path := fmt.Sprintf("firewall ipv4 name %s rule %d disable", policy, ruleID)
	out, _, err := c.Conf.Delete(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"disabled": false})
}
