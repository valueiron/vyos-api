package handlers_test

import (
	"net/http"
	"testing"
)

func TestListVLANs_OK(t *testing.T) {
	ifaceData := map[string]interface{}{
		"ethernet": map[string]interface{}{
			"eth0": map[string]interface{}{
				"address": "192.168.1.1/24",
				"vif": map[string]interface{}{
					"10": map[string]interface{}{
						"address":     "10.10.0.1/24",
						"description": "vlan-10",
					},
					"20": map[string]interface{}{
						"address": []interface{}{"10.20.0.1/24", "10.20.0.2/24"},
					},
				},
			},
		},
		"loopback": map[string]interface{}{
			"lo": map[string]interface{}{}, // no vif — should not appear
		},
	}
	_, _, client := newMockVyOS(t, dataResp(ifaceData))
	h := newHandler(client)

	w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListVLANs)
	assertStatus(t, w, http.StatusOK)

	var result []map[string]interface{}
	decodeJSON(t, w, &result)
	if len(result) != 2 {
		t.Fatalf("got %d VLANs, want 2", len(result))
	}

	for _, vlan := range result {
		addrs, ok := vlan["addresses"]
		if !ok {
			t.Errorf("VLAN %v missing addresses field", vlan["vlan_id"])
			continue
		}
		if addrs == nil {
			t.Errorf("VLAN %v has null addresses, want []", vlan["vlan_id"])
		}
	}
}

func TestListVLANs_NoVLANs(t *testing.T) {
	ifaceData := map[string]interface{}{
		"ethernet": map[string]interface{}{
			"eth0": map[string]interface{}{"address": "10.0.0.1/24"},
		},
	}
	_, _, client := newMockVyOS(t, dataResp(ifaceData))
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListVLANs)
	assertStatus(t, w, http.StatusOK)

	var result []map[string]interface{}
	decodeJSON(t, w, &result)
	if len(result) != 0 {
		t.Errorf("got %d VLANs, want 0", len(result))
	}
}

func TestListVLANs_NoAddress_NeverNull(t *testing.T) {
	// A vif with no address should still have addresses: [].
	ifaceData := map[string]interface{}{
		"ethernet": map[string]interface{}{
			"eth0": map[string]interface{}{
				"vif": map[string]interface{}{
					"100": map[string]interface{}{}, // no address
				},
			},
		},
	}
	_, _, client := newMockVyOS(t, dataResp(ifaceData))
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListVLANs)
	assertStatus(t, w, http.StatusOK)

	var result []map[string]interface{}
	decodeJSON(t, w, &result)
	if len(result) != 1 {
		t.Fatalf("want 1 VLAN, got %d", len(result))
	}
	if result[0]["addresses"] == nil {
		t.Error("addresses is null, want []")
	}
}

func TestCreateVLAN_OK(t *testing.T) {
	_, _, client := newMockVyOS(t, successResp())
	h := newHandler(client)

	body := map[string]interface{}{
		"interface": "eth0",
		"type":      "ethernet",
		"vlan_id":   100,
		"address":   "10.100.0.1/24",
	}
	w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateVLAN)
	assertStatus(t, w, http.StatusCreated)

	var result map[string]interface{}
	decodeJSON(t, w, &result)
	if result["vlan_id"] != float64(100) {
		t.Errorf("vlan_id = %v, want 100", result["vlan_id"])
	}
}

func TestCreateVLAN_MissingVLANID(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	body := map[string]interface{}{"interface": "eth0", "type": "ethernet"}
	w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateVLAN)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestGetVLAN_OK(t *testing.T) {
	vifCfg := map[string]interface{}{"address": "10.100.0.1/24"}
	_, _, client := newMockVyOS(t, dataResp(vifCfg))
	h := newHandler(client)

	w := do(t, http.MethodGet, "/?type=ethernet", nil,
		deviceVars("interface", "eth0", "vlan_id", "100"),
		h.GetVLAN)
	assertStatus(t, w, http.StatusOK)

	var result map[string]interface{}
	decodeJSON(t, w, &result)
	if result["vlan_id"] != float64(100) {
		t.Errorf("vlan_id = %v, want 100", result["vlan_id"])
	}
}

func TestGetVLAN_InvalidVLANID(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil,
		deviceVars("interface", "eth0", "vlan_id", "notanumber"),
		h.GetVLAN)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestGetVLAN_NotFound(t *testing.T) {
	_, _, client := newMockVyOS(t, failResp("vif not found"))
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil,
		deviceVars("interface", "eth0", "vlan_id", "999"),
		h.GetVLAN)
	assertStatus(t, w, http.StatusNotFound)
}

func TestDeleteVLAN_OK(t *testing.T) {
	_, _, client := newMockVyOS(t, successResp())
	h := newHandler(client)
	w := do(t, http.MethodDelete, "/?type=ethernet", nil,
		deviceVars("interface", "eth0", "vlan_id", "100"),
		h.DeleteVLAN)
	assertStatus(t, w, http.StatusNoContent)
}

func TestDeleteVLAN_InvalidVLANID(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	w := do(t, http.MethodDelete, "/", nil,
		deviceVars("interface", "eth0", "vlan_id", "bad"),
		h.DeleteVLAN)
	assertStatus(t, w, http.StatusBadRequest)
}
