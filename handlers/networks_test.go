package handlers_test

import (
	"net/http"
	"testing"
)

// --------------------------------------------------------------------------
// ListNetworks
// --------------------------------------------------------------------------

func TestListNetworks_OK(t *testing.T) {
	// VyOS returns interfaces with a mix of single-value and multi-value addresses.
	ifaceData := map[string]interface{}{
		"ethernet": map[string]interface{}{
			"eth0": map[string]interface{}{
				"address":     "192.168.1.1/24",
				"description": "LAN",
			},
			"eth1": map[string]interface{}{
				"address": []interface{}{"10.0.0.1/24", "10.0.0.2/24"},
			},
		},
		"loopback": map[string]interface{}{
			"lo": map[string]interface{}{},
		},
	}
	_, _, client := newMockVyOS(t, dataResp(ifaceData))
	h := newHandler(client)

	w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListNetworks)
	assertStatus(t, w, http.StatusOK)

	var result []map[string]interface{}
	decodeJSON(t, w, &result)

	if len(result) != 3 {
		t.Fatalf("got %d interfaces, want 3", len(result))
	}

	for _, iface := range result {
		addrs, ok := iface["addresses"]
		if !ok {
			t.Errorf("interface %v missing 'addresses' field", iface["interface"])
			continue
		}
		// addresses must be a JSON array, never null.
		if addrs == nil {
			t.Errorf("interface %v has null addresses, want []", iface["interface"])
		}
	}
}

func TestListNetworks_NoAddress_NeverNull(t *testing.T) {
	// An interface with no address configured (e.g. fresh loopback).
	ifaceData := map[string]interface{}{
		"loopback": map[string]interface{}{
			"lo": map[string]interface{}{}, // no address key
		},
	}
	_, _, client := newMockVyOS(t, dataResp(ifaceData))
	h := newHandler(client)

	w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListNetworks)
	assertStatus(t, w, http.StatusOK)

	var result []map[string]interface{}
	decodeJSON(t, w, &result)
	if len(result) != 1 {
		t.Fatalf("want 1 interface, got %d", len(result))
	}

	addrs := result[0]["addresses"]
	if addrs == nil {
		t.Error("addresses is null, want empty array []")
	}
	// JSON decodes a JSON array as []interface{}, even if empty.
	if arr, ok := addrs.([]interface{}); ok {
		if len(arr) != 0 {
			t.Errorf("expected empty addresses, got %v", arr)
		}
	} else {
		t.Errorf("addresses is %T, want []interface{}", addrs)
	}
}

func TestListNetworks_DeviceNotFound(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil, unknownDeviceVars(), h.ListNetworks)
	assertStatus(t, w, http.StatusNotFound)
}

func TestListNetworks_DeviceCommunicationError(t *testing.T) {
	// No mock server — create a client pointing at a closed server.
	_, srv, client := newMockVyOS(t)
	srv.Close() // close before the call
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListNetworks)
	assertStatus(t, w, http.StatusBadGateway)
}

func TestListNetworks_DeviceRejected(t *testing.T) {
	_, _, client := newMockVyOS(t, failResp("internal VyOS error"))
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListNetworks)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

// --------------------------------------------------------------------------
// CreateNetwork
// --------------------------------------------------------------------------

func TestCreateNetwork_OK(t *testing.T) {
	_, _, client := newMockVyOS(t, successResp())
	h := newHandler(client)

	body := map[string]string{
		"interface": "eth0",
		"type":      "ethernet",
		"address":   "192.168.1.1/24",
	}
	w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateNetwork)
	assertStatus(t, w, http.StatusCreated)

	var result map[string]interface{}
	decodeJSON(t, w, &result)
	if result["interface"] != "eth0" {
		t.Errorf("interface = %v, want eth0", result["interface"])
	}
}

func TestCreateNetwork_MissingFields(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)

	// Missing address.
	body := map[string]string{"interface": "eth0", "type": "ethernet"}
	w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateNetwork)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateNetwork_DeviceRejected(t *testing.T) {
	_, _, client := newMockVyOS(t, failResp("address already exists"))
	h := newHandler(client)
	body := map[string]string{
		"interface": "eth0", "type": "ethernet", "address": "10.0.0.1/24",
	}
	w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateNetwork)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}

// --------------------------------------------------------------------------
// GetNetwork
// --------------------------------------------------------------------------

func TestGetNetwork_OK(t *testing.T) {
	ifaceCfg := map[string]interface{}{
		"address":     "192.168.1.1/24",
		"description": "LAN",
	}
	_, _, client := newMockVyOS(t, dataResp(ifaceCfg))
	h := newHandler(client)

	w := do(t, http.MethodGet, "/", nil,
		deviceVars("interface", "eth0"),
		h.GetNetwork)
	assertStatus(t, w, http.StatusOK)

	var result map[string]interface{}
	decodeJSON(t, w, &result)
	if result["interface"] != "eth0" {
		t.Errorf("interface = %v, want eth0", result["interface"])
	}
	addrs, _ := result["addresses"].([]interface{})
	if len(addrs) != 1 || addrs[0] != "192.168.1.1/24" {
		t.Errorf("addresses = %v, want [192.168.1.1/24]", addrs)
	}
}

func TestGetNetwork_NoAddress_NeverNull(t *testing.T) {
	// VyOS returns the interface config but no address is set.
	_, _, client := newMockVyOS(t, dataResp(map[string]interface{}{}))
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil, deviceVars("interface", "lo"), h.GetNetwork)
	assertStatus(t, w, http.StatusOK)

	var result map[string]interface{}
	decodeJSON(t, w, &result)
	if result["addresses"] == nil {
		t.Error("addresses is null, want []")
	}
}

func TestGetNetwork_NotFound(t *testing.T) {
	_, _, client := newMockVyOS(t, failResp("not found"))
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil, deviceVars("interface", "eth99"), h.GetNetwork)
	assertStatus(t, w, http.StatusNotFound)
}

// --------------------------------------------------------------------------
// UpdateNetwork
// --------------------------------------------------------------------------

func TestUpdateNetwork_OK(t *testing.T) {
	// Two VyOS calls: Delete (old address) then Set (new address).
	_, _, client := newMockVyOS(t, successResp(), successResp())
	h := newHandler(client)

	body := map[string]string{"type": "ethernet", "address": "10.0.0.1/24"}
	w := do(t, http.MethodPut, "/", body,
		deviceVars("interface", "eth0"),
		h.UpdateNetwork)
	assertStatus(t, w, http.StatusOK)
}

func TestUpdateNetwork_MissingFields(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	// Missing type.
	body := map[string]string{"address": "10.0.0.1/24"}
	w := do(t, http.MethodPut, "/", body, deviceVars("interface", "eth0"), h.UpdateNetwork)
	assertStatus(t, w, http.StatusBadRequest)
}

// --------------------------------------------------------------------------
// DeleteNetwork
// --------------------------------------------------------------------------

func TestDeleteNetwork_OK(t *testing.T) {
	_, _, client := newMockVyOS(t, successResp())
	h := newHandler(client)
	w := do(t, http.MethodDelete, "/?type=ethernet", nil,
		deviceVars("interface", "eth0"),
		h.DeleteNetwork)
	assertStatus(t, w, http.StatusNoContent)
}

func TestDeleteNetwork_DeviceRejected(t *testing.T) {
	_, _, client := newMockVyOS(t, failResp("interface does not exist"))
	h := newHandler(client)
	w := do(t, http.MethodDelete, "/?type=ethernet", nil,
		deviceVars("interface", "eth99"),
		h.DeleteNetwork)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}
