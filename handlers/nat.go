package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

// NATRuleInfo is the API representation of a VyOS NAT rule.
type NATRuleInfo struct {
	RuleID          int    `json:"rule_id"`
	Type            string `json:"type"`
	Description     string `json:"description,omitempty"`
	OutboundIface   string `json:"outbound_interface,omitempty"`
	InboundIface    string `json:"inbound_interface,omitempty"`
	Protocol        string `json:"protocol,omitempty"`
	SourceAddress   string `json:"source_address,omitempty"`
	SourcePort      string `json:"source_port,omitempty"`
	DestAddress     string `json:"destination_address,omitempty"`
	DestPort        string `json:"destination_port,omitempty"`
	TranslationAddr string `json:"translation_address,omitempty"`
	TranslationPort string `json:"translation_port,omitempty"`
	Disabled        bool   `json:"disabled,omitempty"`
}

// CreateNATRuleRequest is the JSON body for POST /devices/{device_id}/nat/{nat_type}/rules.
type CreateNATRuleRequest struct {
	RuleID          int    `json:"rule_id"`
	Description     string `json:"description,omitempty"`
	OutboundIface   string `json:"outbound_interface,omitempty"`
	InboundIface    string `json:"inbound_interface,omitempty"`
	Protocol        string `json:"protocol,omitempty"`
	SourceAddress   string `json:"source_address,omitempty"`
	SourcePort      string `json:"source_port,omitempty"`
	DestAddress     string `json:"destination_address,omitempty"`
	DestPort        string `json:"destination_port,omitempty"`
	TranslationAddr string `json:"translation_address,omitempty"`
	TranslationPort string `json:"translation_port,omitempty"`
}

// UpdateNATRuleRequest is the JSON body for PUT /devices/{device_id}/nat/{nat_type}/rules/{rule_id}.
type UpdateNATRuleRequest struct {
	Description     string `json:"description,omitempty"`
	OutboundIface   string `json:"outbound_interface,omitempty"`
	InboundIface    string `json:"inbound_interface,omitempty"`
	Protocol        string `json:"protocol,omitempty"`
	SourceAddress   string `json:"source_address,omitempty"`
	SourcePort      string `json:"source_port,omitempty"`
	DestAddress     string `json:"destination_address,omitempty"`
	DestPort        string `json:"destination_port,omitempty"`
	TranslationAddr string `json:"translation_address,omitempty"`
	TranslationPort string `json:"translation_port,omitempty"`
}

func validNATType(natType string) bool {
	return natType == "source" || natType == "destination"
}

func natRulePath(natType string, ruleID int) string {
	return fmt.Sprintf("nat %s rule %d", natType, ruleID)
}

// ListNATRules handles GET /devices/{device_id}/nat/{nat_type}/rules.
func (h *Handler) ListNATRules(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}
	natType := mux.Vars(r)["nat_type"]
	if !validNATType(natType) {
		writeError(w, http.StatusBadRequest, "nat_type must be 'source' or 'destination'")
		return
	}

	out, _, err := c.Conf.Get(r.Context(), fmt.Sprintf("nat %s rule", natType), nil)
	if err != nil {
		// VyOS returns HTTP 400 when a config path doesn't exist at all (NAT not yet configured).
		// Treat it the same as an empty result rather than an error.
		if strings.Contains(err.Error(), "unexpected status 400") {
			writeJSON(w, http.StatusOK, []NATRuleInfo{})
			return
		}
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}

	result := []NATRuleInfo{}
	if out.Success {
		rawMap, _ := out.Data.(map[string]interface{})
		ruleMap := rawMap
		if inner, ok := rawMap["rule"].(map[string]interface{}); ok {
			ruleMap = inner
		}
		for idStr, ruleData := range ruleMap {
			ruleID, err := strconv.Atoi(idStr)
			if err != nil {
				continue
			}
			result = append(result, parseNATRuleData(natType, ruleID, ruleData))
		}
	} else if !strings.Contains(errMsg(out.Error), "empty") {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateNATRule handles POST /devices/{device_id}/nat/{nat_type}/rules.
func (h *Handler) CreateNATRule(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}
	natType := mux.Vars(r)["nat_type"]
	if !validNATType(natType) {
		writeError(w, http.StatusBadRequest, "nat_type must be 'source' or 'destination'")
		return
	}

	var req CreateNATRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.RuleID == 0 {
		writeError(w, http.StatusBadRequest, "rule_id is required")
		return
	}
	if req.TranslationAddr == "" {
		writeError(w, http.StatusBadRequest, "translation_address is required")
		return
	}

	base := natRulePath(natType, req.RuleID)

	// translation address is required — use it as the anchor set call.
	out, _, err := c.Conf.Set(r.Context(), fmt.Sprintf("%s translation address %s", base, req.TranslationAddr))
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
		return
	}

	if req.TranslationPort != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s translation port %s", base, req.TranslationPort)) //nolint:errcheck
	}
	if req.Description != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s description %s", base, req.Description)) //nolint:errcheck
	}
	if req.Protocol != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s protocol %s", base, req.Protocol)) //nolint:errcheck
	}
	if req.OutboundIface != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s outbound-interface name %s", base, req.OutboundIface)) //nolint:errcheck
	}
	if req.InboundIface != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s inbound-interface name %s", base, req.InboundIface)) //nolint:errcheck
	}
	if req.SourceAddress != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s source address %s", base, req.SourceAddress)) //nolint:errcheck
	}
	if req.SourcePort != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s source port %s", base, req.SourcePort)) //nolint:errcheck
	}
	if req.DestAddress != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s destination address %s", base, req.DestAddress)) //nolint:errcheck
	}
	if req.DestPort != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s destination port %s", base, req.DestPort)) //nolint:errcheck
	}

	writeJSON(w, http.StatusCreated, NATRuleInfo{
		RuleID:          req.RuleID,
		Type:            natType,
		Description:     req.Description,
		OutboundIface:   req.OutboundIface,
		InboundIface:    req.InboundIface,
		Protocol:        req.Protocol,
		SourceAddress:   req.SourceAddress,
		SourcePort:      req.SourcePort,
		DestAddress:     req.DestAddress,
		DestPort:        req.DestPort,
		TranslationAddr: req.TranslationAddr,
		TranslationPort: req.TranslationPort,
	})
}

// GetNATRule handles GET /devices/{device_id}/nat/{nat_type}/rules/{rule_id}.
func (h *Handler) GetNATRule(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}
	vars := mux.Vars(r)
	natType := vars["nat_type"]
	if !validNATType(natType) {
		writeError(w, http.StatusBadRequest, "nat_type must be 'source' or 'destination'")
		return
	}
	ruleID, err := strconv.Atoi(vars["rule_id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "rule_id must be an integer")
		return
	}

	out, _, err := c.Conf.Get(r.Context(), natRulePath(natType, ruleID), nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	if !out.Success {
		writeError(w, http.StatusNotFound, "NAT rule not found")
		return
	}

	writeJSON(w, http.StatusOK, parseNATRuleData(natType, ruleID, out.Data))
}

// UpdateNATRule handles PUT /devices/{device_id}/nat/{nat_type}/rules/{rule_id}.
func (h *Handler) UpdateNATRule(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}
	vars := mux.Vars(r)
	natType := vars["nat_type"]
	if !validNATType(natType) {
		writeError(w, http.StatusBadRequest, "nat_type must be 'source' or 'destination'")
		return
	}
	ruleID, err := strconv.Atoi(vars["rule_id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "rule_id must be an integer")
		return
	}

	var req UpdateNATRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	base := natRulePath(natType, ruleID)

	if req.TranslationAddr != "" {
		out, _, err := c.Conf.Set(r.Context(), fmt.Sprintf("%s translation address %s", base, req.TranslationAddr))
		if err != nil {
			writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
			return
		}
		if !out.Success {
			writeError(w, http.StatusUnprocessableEntity, "device rejected operation: "+errMsg(out.Error))
			return
		}
	}
	if req.TranslationPort != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s translation port %s", base, req.TranslationPort)) //nolint:errcheck
	}
	if req.Description != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s description %s", base, req.Description)) //nolint:errcheck
	}
	if req.Protocol != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s protocol %s", base, req.Protocol)) //nolint:errcheck
	}
	if req.OutboundIface != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s outbound-interface name %s", base, req.OutboundIface)) //nolint:errcheck
	}
	if req.InboundIface != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s inbound-interface name %s", base, req.InboundIface)) //nolint:errcheck
	}
	if req.SourceAddress != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s source address %s", base, req.SourceAddress)) //nolint:errcheck
	}
	if req.SourcePort != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s source port %s", base, req.SourcePort)) //nolint:errcheck
	}
	if req.DestAddress != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s destination address %s", base, req.DestAddress)) //nolint:errcheck
	}
	if req.DestPort != "" {
		c.Conf.Set(r.Context(), fmt.Sprintf("%s destination port %s", base, req.DestPort)) //nolint:errcheck
	}

	// Return updated state.
	out, _, err := c.Conf.Get(r.Context(), base, nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "device communication error: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, parseNATRuleData(natType, ruleID, out.Data))
}

// DeleteNATRule handles DELETE /devices/{device_id}/nat/{nat_type}/rules/{rule_id}.
func (h *Handler) DeleteNATRule(w http.ResponseWriter, r *http.Request) {
	c, ok := h.getClient(w, r)
	if !ok {
		return
	}
	vars := mux.Vars(r)
	natType := vars["nat_type"]
	if !validNATType(natType) {
		writeError(w, http.StatusBadRequest, "nat_type must be 'source' or 'destination'")
		return
	}
	ruleID, err := strconv.Atoi(vars["rule_id"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "rule_id must be an integer")
		return
	}

	out, _, err := c.Conf.Delete(r.Context(), natRulePath(natType, ruleID))
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

// parseNATRuleData converts raw VyOS config data into a NATRuleInfo.
func parseNATRuleData(natType string, ruleID int, data interface{}) NATRuleInfo {
	cfg, _ := data.(map[string]interface{})
	desc, _ := cfg["description"].(string)
	protocol, _ := cfg["protocol"].(string)
	_, disabled := cfg["disable"]

	var outboundIface, inboundIface string
	if ob, ok := cfg["outbound-interface"].(map[string]interface{}); ok {
		outboundIface, _ = ob["name"].(string)
	}
	if ib, ok := cfg["inbound-interface"].(map[string]interface{}); ok {
		inboundIface, _ = ib["name"].(string)
	}

	var srcAddr, srcPort string
	if src, ok := cfg["source"].(map[string]interface{}); ok {
		srcAddr, _ = src["address"].(string)
		srcPort, _ = src["port"].(string)
	}

	var dstAddr, dstPort string
	if dst, ok := cfg["destination"].(map[string]interface{}); ok {
		dstAddr, _ = dst["address"].(string)
		dstPort, _ = dst["port"].(string)
	}

	var transAddr, transPort string
	if trans, ok := cfg["translation"].(map[string]interface{}); ok {
		transAddr, _ = trans["address"].(string)
		transPort, _ = trans["port"].(string)
	}

	return NATRuleInfo{
		RuleID:          ruleID,
		Type:            natType,
		Description:     desc,
		OutboundIface:   outboundIface,
		InboundIface:    inboundIface,
		Protocol:        protocol,
		SourceAddress:   srcAddr,
		SourcePort:      srcPort,
		DestAddress:     dstAddr,
		DestPort:        dstPort,
		TranslationAddr: transAddr,
		TranslationPort: transPort,
		Disabled:        disabled,
	}
}
