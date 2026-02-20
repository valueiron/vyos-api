package handlers_test

import (
	"net/http"
	"testing"
)

func TestListAddressGroups_OK(t *testing.T) {
	groupData := map[string]interface{}{
		"RFC1918": map[string]interface{}{
			"address":     []interface{}{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
			"description": "private-ranges",
		},
		"MGMT": map[string]interface{}{
			"address": "192.168.1.0/24",
		},
	}
	_, _, client := newMockVyOS(t, dataResp(groupData))
	h := newHandler(client)

	w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListAddressGroups)
	assertStatus(t, w, http.StatusOK)

	var result []map[string]interface{}
	decodeJSON(t, w, &result)
	if len(result) != 2 {
		t.Fatalf("got %d groups, want 2", len(result))
	}
	for _, g := range result {
		if g["addresses"] == nil {
			t.Errorf("group %v has null addresses", g["name"])
		}
	}
}

func TestListAddressGroups_DeviceNotFound(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil, unknownDeviceVars(), h.ListAddressGroups)
	assertStatus(t, w, http.StatusNotFound)
}

func TestCreateAddressGroup_OK(t *testing.T) {
	// Two Set calls (one per address), success for each.
	_, _, client := newMockVyOS(t, successResp(), successResp())
	h := newHandler(client)

	body := map[string]interface{}{
		"name":      "RFC1918",
		"addresses": []string{"10.0.0.0/8", "192.168.0.0/16"},
	}
	w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateAddressGroup)
	assertStatus(t, w, http.StatusCreated)

	var result map[string]interface{}
	decodeJSON(t, w, &result)
	if result["name"] != "RFC1918" {
		t.Errorf("name = %v, want RFC1918", result["name"])
	}
	addrs, _ := result["addresses"].([]interface{})
	if len(addrs) != 2 {
		t.Errorf("got %d addresses, want 2", len(addrs))
	}
}

func TestCreateAddressGroup_Empty(t *testing.T) {
	// No addresses — sends a single Set to create the empty group node.
	_, _, client := newMockVyOS(t, successResp())
	h := newHandler(client)

	body := map[string]interface{}{"name": "EMPTY", "addresses": []string{}}
	w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateAddressGroup)
	assertStatus(t, w, http.StatusCreated)
}

func TestCreateAddressGroup_MissingName(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	body := map[string]interface{}{"addresses": []string{"10.0.0.1"}}
	w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateAddressGroup)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestGetAddressGroup_OK(t *testing.T) {
	groupCfg := map[string]interface{}{
		"address":     []interface{}{"10.0.0.0/8", "192.168.0.0/16"},
		"description": "private",
	}
	_, _, client := newMockVyOS(t, dataResp(groupCfg))
	h := newHandler(client)

	w := do(t, http.MethodGet, "/", nil, deviceVars("group", "RFC1918"), h.GetAddressGroup)
	assertStatus(t, w, http.StatusOK)

	var result map[string]interface{}
	decodeJSON(t, w, &result)
	if result["name"] != "RFC1918" {
		t.Errorf("name = %v, want RFC1918", result["name"])
	}
	addrs, _ := result["addresses"].([]interface{})
	if len(addrs) != 2 {
		t.Errorf("got %d addresses, want 2", len(addrs))
	}
}

func TestGetAddressGroup_EmptyGroup_NeverNull(t *testing.T) {
	// A group with no members should return addresses: [], not null.
	groupCfg := map[string]interface{}{} // no address key
	_, _, client := newMockVyOS(t, dataResp(groupCfg))
	h := newHandler(client)

	w := do(t, http.MethodGet, "/", nil, deviceVars("group", "EMPTY"), h.GetAddressGroup)
	assertStatus(t, w, http.StatusOK)

	var result map[string]interface{}
	decodeJSON(t, w, &result)
	if result["addresses"] == nil {
		t.Error("addresses is null, want []")
	}
}

func TestGetAddressGroup_NotFound(t *testing.T) {
	_, _, client := newMockVyOS(t, failResp("group not found"))
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil, deviceVars("group", "NOPE"), h.GetAddressGroup)
	assertStatus(t, w, http.StatusNotFound)
}

func TestUpdateAddressGroup_OK(t *testing.T) {
	// Delete existing addresses, then two Set calls (one per new address).
	_, _, client := newMockVyOS(t, successResp(), successResp(), successResp())
	h := newHandler(client)

	body := map[string]interface{}{"addresses": []string{"10.0.0.1", "10.0.0.2"}}
	w := do(t, http.MethodPut, "/", body, deviceVars("group", "RFC1918"), h.UpdateAddressGroup)
	assertStatus(t, w, http.StatusOK)

	var result map[string]interface{}
	decodeJSON(t, w, &result)
	addrs, _ := result["addresses"].([]interface{})
	if len(addrs) != 2 {
		t.Errorf("got %d addresses, want 2", len(addrs))
	}
}

func TestDeleteAddressGroup_OK(t *testing.T) {
	_, _, client := newMockVyOS(t, successResp())
	h := newHandler(client)
	w := do(t, http.MethodDelete, "/", nil, deviceVars("group", "RFC1918"), h.DeleteAddressGroup)
	assertStatus(t, w, http.StatusNoContent)
}

func TestDeleteAddressGroup_Rejected(t *testing.T) {
	_, _, client := newMockVyOS(t, failResp("group not found"))
	h := newHandler(client)
	w := do(t, http.MethodDelete, "/", nil, deviceVars("group", "NOPE"), h.DeleteAddressGroup)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}
