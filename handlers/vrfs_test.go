package handlers_test

import (
	"net/http"
	"testing"
)

func TestListVRFs_OK(t *testing.T) {
	vrfData := map[string]interface{}{
		"MGMT": map[string]interface{}{"table": "100", "description": "management"},
		"PROD": map[string]interface{}{"table": "200"},
	}
	_, _, client := newMockVyOS(t, dataResp(vrfData))
	h := newHandler(client)

	w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListVRFs)
	assertStatus(t, w, http.StatusOK)

	var result []map[string]interface{}
	decodeJSON(t, w, &result)
	if len(result) != 2 {
		t.Fatalf("got %d VRFs, want 2", len(result))
	}
}

func TestListVRFs_Empty(t *testing.T) {
	// VyOS returns null data when no VRFs are configured.
	_, _, client := newMockVyOS(t, dataResp(nil))
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListVRFs)
	assertStatus(t, w, http.StatusOK)

	var result []map[string]interface{}
	decodeJSON(t, w, &result)
	if len(result) != 0 {
		t.Errorf("got %d VRFs, want 0", len(result))
	}
}

func TestListVRFs_DeviceNotFound(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil, unknownDeviceVars(), h.ListVRFs)
	assertStatus(t, w, http.StatusNotFound)
}

func TestCreateVRF_OK(t *testing.T) {
	_, _, client := newMockVyOS(t, successResp())
	h := newHandler(client)

	body := map[string]string{"name": "MGMT", "table": "100"}
	w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateVRF)
	assertStatus(t, w, http.StatusCreated)

	var result map[string]interface{}
	decodeJSON(t, w, &result)
	if result["name"] != "MGMT" {
		t.Errorf("name = %v, want MGMT", result["name"])
	}
	if result["table"] != "100" {
		t.Errorf("table = %v, want 100", result["table"])
	}
}

func TestCreateVRF_MissingName(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	body := map[string]string{"table": "100"}
	w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateVRF)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestCreateVRF_MissingTable(t *testing.T) {
	_, _, client := newMockVyOS(t)
	h := newHandler(client)
	body := map[string]string{"name": "MGMT"}
	w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateVRF)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestGetVRF_OK(t *testing.T) {
	vrfCfg := map[string]interface{}{"table": "100", "description": "management"}
	_, _, client := newMockVyOS(t, dataResp(vrfCfg))
	h := newHandler(client)

	w := do(t, http.MethodGet, "/", nil, deviceVars("vrf", "MGMT"), h.GetVRF)
	assertStatus(t, w, http.StatusOK)

	var result map[string]interface{}
	decodeJSON(t, w, &result)
	if result["name"] != "MGMT" {
		t.Errorf("name = %v, want MGMT", result["name"])
	}
}

func TestGetVRF_NotFound(t *testing.T) {
	_, _, client := newMockVyOS(t, failResp("VRF does not exist"))
	h := newHandler(client)
	w := do(t, http.MethodGet, "/", nil, deviceVars("vrf", "NOPE"), h.GetVRF)
	assertStatus(t, w, http.StatusNotFound)
}

func TestUpdateVRF_OK(t *testing.T) {
	vrfCfg := map[string]interface{}{"table": "101", "description": "updated"}
	// One Set call for table, one for description, then one Get call.
	_, _, client := newMockVyOS(t, successResp(), successResp(), dataResp(vrfCfg))
	h := newHandler(client)

	body := map[string]string{"table": "101", "description": "updated"}
	w := do(t, http.MethodPut, "/", body, deviceVars("vrf", "MGMT"), h.UpdateVRF)
	assertStatus(t, w, http.StatusOK)
}

func TestDeleteVRF_OK(t *testing.T) {
	_, _, client := newMockVyOS(t, successResp())
	h := newHandler(client)
	w := do(t, http.MethodDelete, "/", nil, deviceVars("vrf", "MGMT"), h.DeleteVRF)
	assertStatus(t, w, http.StatusNoContent)
}

func TestDeleteVRF_Rejected(t *testing.T) {
	_, _, client := newMockVyOS(t, failResp("VRF not found"))
	h := newHandler(client)
	w := do(t, http.MethodDelete, "/", nil, deviceVars("vrf", "NOPE"), h.DeleteVRF)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}
