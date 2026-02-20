package handlers_test

import (
	"net/http"
	"testing"
)

func TestListPolicies_OK(t *testing.T) {
	policyData := map[string]interface{}{
		"LAN-IN": map[string]interface{}{
			"default-action": "drop",
			"description":    "inbound",
			"rule": map[string]interface{}{
				"10": map[string]interface{}{
					"action": "accept",
					"source": map[string]interface{}{"address": "10.0.0.0/8"},
				},
			},
		},
		"WAN-IN": map[string]interface{}{
			"default-action": "accept",
		},
	}
	_, _, client := newMockVyOS(t, dataResp(policyData))
	h := newHandler(client)

	w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListPolicies)
	assertStatus(t, w, http.StatusOK)

	var result []map[string]interface{}
	decodeJSON(t, w, &result)
	if len(result) != 2 {
		t.Fatalf("got %d policies, want 2", len(result))
	}
}

func TestListPolicies_DeviceNotFound(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil, unknownDeviceVars(), h.ListPolicies)
	assertStatus(t, w, http.StatusNotFound)
}

func TestCreatePolicy_OK(t *testing.T) {
	_, _, client := newMockVyOS(t, successResp())
	h := newHandler(client)

	body := map[string]string{"name": "LAN-IN", "default_action": "drop"}
	w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreatePolicy)
	assertStatus(t, w, http.StatusCreated)

	var result map[string]interface{}
	decodeJSON(t, w, &result)
	if result["name"] != "LAN-IN" {
		t.Errorf("name = %v, want LAN-IN", result["name"])
	}
	if result["default_action"] != "drop" {
		t.Errorf("default_action = %v, want drop", result["default_action"])
	}
}

func TestCreatePolicy_MissingDefaultAction(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	body := map[string]string{"name": "LAN-IN"}
	w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreatePolicy)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestGetPolicy_OK(t *testing.T) {
	policyData := map[string]interface{}{
		"default-action": "drop",
		"description":    "inbound",
		"rule": map[string]interface{}{
			"10": map[string]interface{}{"action": "accept"},
			"20": map[string]interface{}{
				"action":      "drop",
				"source":      map[string]interface{}{"address": "1.2.3.4"},
				"destination": map[string]interface{}{"address": "5.6.7.8"},
			},
		},
	}
	_, _, client := newMockVyOS(t, dataResp(policyData))
	h := newHandler(client)

	w := do(t, http.MethodGet, "/", nil, deviceVars("policy", "LAN-IN"), h.GetPolicy)
	assertStatus(t, w, http.StatusOK)

	var result map[string]interface{}
	decodeJSON(t, w, &result)
	if result["name"] != "LAN-IN" {
		t.Errorf("name = %v, want LAN-IN", result["name"])
	}
	rules, ok := result["rules"].(map[string]interface{})
	if !ok {
		t.Fatalf("rules is %T, want map", result["rules"])
	}
	if len(rules) != 2 {
		t.Errorf("got %d rules, want 2", len(rules))
	}
	rule20, ok := rules["20"].(map[string]interface{})
	if !ok {
		t.Fatal("rule 20 not found or wrong type")
	}
	if rule20["source"] != "1.2.3.4" {
		t.Errorf("rule 20 source = %v, want 1.2.3.4", rule20["source"])
	}
	if rule20["destination"] != "5.6.7.8" {
		t.Errorf("rule 20 destination = %v, want 5.6.7.8", rule20["destination"])
	}
}

func TestGetPolicy_NotFound(t *testing.T) {
	_, _, client := newMockVyOS(t, failResp("policy not found"))
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil, deviceVars("policy", "NOPE"), h.GetPolicy)
	assertStatus(t, w, http.StatusNotFound)
}

func TestUpdatePolicy_OK(t *testing.T) {
	updatedData := map[string]interface{}{"default-action": "accept"}
	// One Set call, then a Get to return the updated state.
	_, _, client := newMockVyOS(t, successResp(), dataResp(updatedData))
	h := newHandler(client)

	body := map[string]string{"default_action": "accept"}
	w := do(t, http.MethodPut, "/", body, deviceVars("policy", "LAN-IN"), h.UpdatePolicy)
	assertStatus(t, w, http.StatusOK)
}

func TestDeletePolicy_OK(t *testing.T) {
	_, _, client := newMockVyOS(t, successResp())
	h := newHandler(client)
	w := do(t, http.MethodDelete, "/", nil, deviceVars("policy", "LAN-IN"), h.DeletePolicy)
	assertStatus(t, w, http.StatusNoContent)
}

func TestAddRule_OK(t *testing.T) {
	_, _, client := newMockVyOS(t, successResp())
	h := newHandler(client)

	body := map[string]interface{}{
		"rule_id": 10,
		"action":  "accept",
		"source":  "10.0.0.0/8",
	}
	w := do(t, http.MethodPost, "/", body, deviceVars("policy", "LAN-IN"), h.AddRule)
	assertStatus(t, w, http.StatusCreated)

	var result map[string]interface{}
	decodeJSON(t, w, &result)
	if result["rule_id"] != float64(10) {
		t.Errorf("rule_id = %v, want 10", result["rule_id"])
	}
	if result["action"] != "accept" {
		t.Errorf("action = %v, want accept", result["action"])
	}
}

func TestAddRule_MissingRuleID(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	body := map[string]interface{}{"action": "accept"}
	w := do(t, http.MethodPost, "/", body, deviceVars("policy", "LAN-IN"), h.AddRule)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestAddRule_MissingAction(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	body := map[string]interface{}{"rule_id": 10}
	w := do(t, http.MethodPost, "/", body, deviceVars("policy", "LAN-IN"), h.AddRule)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestDeleteRule_OK(t *testing.T) {
	_, _, client := newMockVyOS(t, successResp())
	h := newHandler(client)
	w := do(t, http.MethodDelete, "/", nil,
		deviceVars("policy", "LAN-IN", "rule_id", "10"),
		h.DeleteRule)
	assertStatus(t, w, http.StatusNoContent)
}

func TestDeleteRule_InvalidRuleID(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	w := do(t, http.MethodDelete, "/", nil,
		deviceVars("policy", "LAN-IN", "rule_id", "notanumber"),
		h.DeleteRule)
	assertStatus(t, w, http.StatusBadRequest)
}
