package handlers_test

import (
	"net/http"
	"testing"
)

// CRUD tests run full create-read-update-delete flows per resource using the mock VyOS.
// Each test runs List → Create → Get → Update → Delete (and for firewall: AddRule, DeleteRule) in order.
//
// Run all CRUD tests:
//   go test -v -run CRUD ./handlers
//
// Run one resource at a time (troubleshoot step by step):
//   go test -v -run 'TestCRUD_Networks' ./handlers
//   go test -v -run 'TestCRUD_VRFs' ./handlers
//   go test -v -run 'TestCRUD_VLANs' ./handlers
//   go test -v -run 'TestCRUD_FirewallPolicies' ./handlers
//   go test -v -run 'TestCRUD_AddressGroups' ./handlers
//
// Run a single step (e.g. just Update for VRFs):
//   go test -v -run 'TestCRUD_VRFs/Update' ./handlers
//
// If a step fails, the mock may be out of sync with the number of VyOS API calls
// the handler makes (e.g. CreateVRF sends 2 calls when description is set).
// Adjust the newMockVyOS response queue in that test accordingly.

func TestCRUD_Networks(t *testing.T) {
	// Queue: List(Get interfaces), Create(Set), Get(Get iface), Update(Delete+Set), Delete(Delete)
	listData := map[string]interface{}{
		"ethernet": map[string]interface{}{
			"eth0": map[string]interface{}{"address": "192.168.1.1/24", "description": "LAN"},
		},
	}
	getCfg := map[string]interface{}{"address": "192.168.1.1/24", "description": "LAN"}
	_, _, client := newMockVyOS(t,
		dataResp(listData),   // ListNetworks
		successResp(),        // CreateNetwork (Set address)
		dataResp(getCfg),     // GetNetwork
		successResp(),        // UpdateNetwork (Delete address)
		successResp(),        // UpdateNetwork (Set new address)
		successResp(),        // DeleteNetwork
	)
	h := newHandler(client)

	// Step 1: List
	t.Run("List", func(t *testing.T) {
		w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListNetworks)
		assertStatus(t, w, http.StatusOK)
		var list []map[string]interface{}
		decodeJSON(t, w, &list)
		if len(list) != 1 {
			t.Fatalf("list: got %d interfaces, want 1", len(list))
		}
		if list[0]["interface"] != "eth0" {
			t.Errorf("list[0].interface = %v, want eth0", list[0]["interface"])
		}
	})

	// Step 2: Create
	t.Run("Create", func(t *testing.T) {
		body := map[string]string{"interface": "eth1", "type": "ethernet", "address": "10.0.0.1/24"}
		w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateNetwork)
		assertStatus(t, w, http.StatusCreated)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["interface"] != "eth1" {
			t.Errorf("interface = %v, want eth1", out["interface"])
		}
		if out["addresses"].([]interface{})[0] != "10.0.0.1/24" {
			t.Errorf("addresses = %v", out["addresses"])
		}
	})

	// Step 3: Get
	t.Run("Get", func(t *testing.T) {
		w := do(t, http.MethodGet, "/", nil, deviceVars("interface", "eth0"), h.GetNetwork)
		assertStatus(t, w, http.StatusOK)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["interface"] != "eth0" {
			t.Errorf("interface = %v, want eth0", out["interface"])
		}
	})

	// Step 4: Update
	t.Run("Update", func(t *testing.T) {
		body := map[string]string{"type": "ethernet", "address": "10.0.0.2/24", "description": "uplink"}
		w := do(t, http.MethodPut, "/", body, deviceVars("interface", "eth0"), h.UpdateNetwork)
		assertStatus(t, w, http.StatusOK)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["addresses"].([]interface{})[0] != "10.0.0.2/24" {
			t.Errorf("addresses = %v", out["addresses"])
		}
	})

	// Step 5: Delete
	t.Run("Delete", func(t *testing.T) {
		w := do(t, http.MethodDelete, "/?type=ethernet", nil, deviceVars("interface", "eth0"), h.DeleteNetwork)
		assertStatus(t, w, http.StatusNoContent)
	})
}

func TestCRUD_VRFs(t *testing.T) {
	listData := map[string]interface{}{
		"MGMT": map[string]interface{}{"table": "100", "description": "management"},
	}
	getCfg := map[string]interface{}{"table": "100", "description": "management"}
	updatedCfg := map[string]interface{}{"table": "101", "description": "updated-desc"}
	_, _, client := newMockVyOS(t,
		dataResp(listData),   // ListVRFs
		successResp(),        // CreateVRF (Set table)
		successResp(),        // CreateVRF (Set description)
		dataResp(getCfg),     // GetVRF
		successResp(),        // UpdateVRF (Set table)
		successResp(),        // UpdateVRF (Set description)
		dataResp(updatedCfg), // UpdateVRF (Get for response)
		successResp(),        // DeleteVRF
	)
	h := newHandler(client)

	t.Run("List", func(t *testing.T) {
		w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListVRFs)
		assertStatus(t, w, http.StatusOK)
		var list []map[string]interface{}
		decodeJSON(t, w, &list)
		if len(list) != 1 || list[0]["name"] != "MGMT" {
			t.Fatalf("list: got %v", list)
		}
	})

	t.Run("Create", func(t *testing.T) {
		body := map[string]string{"name": "TEST", "table": "200", "description": "test-vrf"}
		w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateVRF)
		assertStatus(t, w, http.StatusCreated)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["name"] != "TEST" || out["table"] != "200" {
			t.Errorf("got %v", out)
		}
	})

	t.Run("Get", func(t *testing.T) {
		w := do(t, http.MethodGet, "/", nil, deviceVars("vrf", "MGMT"), h.GetVRF)
		assertStatus(t, w, http.StatusOK)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["name"] != "MGMT" {
			t.Errorf("name = %v", out["name"])
		}
	})

	t.Run("Update", func(t *testing.T) {
		body := map[string]string{"table": "101", "description": "updated-desc"}
		w := do(t, http.MethodPut, "/", body, deviceVars("vrf", "MGMT"), h.UpdateVRF)
		assertStatus(t, w, http.StatusOK)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["table"] != "101" {
			t.Errorf("table = %v, want 101", out["table"])
		}
	})

	t.Run("Delete", func(t *testing.T) {
		w := do(t, http.MethodDelete, "/", nil, deviceVars("vrf", "MGMT"), h.DeleteVRF)
		assertStatus(t, w, http.StatusNoContent)
	})
}

func TestCRUD_VLANs(t *testing.T) {
	listData := map[string]interface{}{
		"ethernet": map[string]interface{}{
			"eth0": map[string]interface{}{
				"address": "10.0.0.1/24",
				"vif": map[string]interface{}{
					"100": map[string]interface{}{"address": "10.100.0.1/24", "description": "vlan100"},
				},
			},
		},
	}
	getVif := map[string]interface{}{"address": "10.100.0.1/24", "description": "vlan100"}
	_, _, client := newMockVyOS(t,
		dataResp(listData),   // ListVLANs
		successResp(),        // CreateVLAN (Set vif with address)
		dataResp(getVif),     // GetVLAN
		successResp(),        // UpdateVLAN (Delete address)
		successResp(),        // UpdateVLAN (Set new address)
		successResp(),        // DeleteVLAN
	)
	h := newHandler(client)

	t.Run("List", func(t *testing.T) {
		w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListVLANs)
		assertStatus(t, w, http.StatusOK)
		var list []map[string]interface{}
		decodeJSON(t, w, &list)
		if len(list) != 1 || list[0]["vlan_id"] != float64(100) {
			t.Fatalf("list: got %v", list)
		}
	})

	t.Run("Create", func(t *testing.T) {
		body := map[string]interface{}{"interface": "eth0", "type": "ethernet", "vlan_id": 200, "address": "10.200.0.1/24"}
		w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateVLAN)
		assertStatus(t, w, http.StatusCreated)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["vlan_id"] != float64(200) {
			t.Errorf("vlan_id = %v", out["vlan_id"])
		}
	})

	t.Run("Get", func(t *testing.T) {
		w := do(t, http.MethodGet, "/?type=ethernet", nil, deviceVars("interface", "eth0", "vlan_id", "100"), h.GetVLAN)
		assertStatus(t, w, http.StatusOK)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["vlan_id"] != float64(100) || out["interface"] != "eth0" {
			t.Errorf("got %v", out)
		}
	})

	t.Run("Update", func(t *testing.T) {
		body := map[string]string{"type": "ethernet", "address": "10.100.0.2/24", "description": "updated"}
		w := do(t, http.MethodPut, "/", body, deviceVars("interface", "eth0", "vlan_id", "100"), h.UpdateVLAN)
		assertStatus(t, w, http.StatusOK)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["addresses"].([]interface{})[0] != "10.100.0.2/24" {
			t.Errorf("addresses = %v", out["addresses"])
		}
	})

	t.Run("Delete", func(t *testing.T) {
		w := do(t, http.MethodDelete, "/?type=ethernet", nil, deviceVars("interface", "eth0", "vlan_id", "100"), h.DeleteVLAN)
		assertStatus(t, w, http.StatusNoContent)
	})
}

func TestCRUD_FirewallPolicies(t *testing.T) {
	listData := map[string]interface{}{
		"LAN-IN": map[string]interface{}{"default-action": "drop", "description": "inbound"},
	}
	getPolicy := map[string]interface{}{
		"default-action": "drop",
		"description":   "inbound",
		"rule": map[string]interface{}{
			"10": map[string]interface{}{"action": "accept", "source": map[string]interface{}{"address": "10.0.0.0/8"}},
		},
	}
	updatedPolicy := map[string]interface{}{"default-action": "accept", "description": "updated-desc"}
	_, _, client := newMockVyOS(t,
		dataResp(listData),     // ListPolicies (named policies)
		successResp(),          // ListPolicies base chain: forward (no config)
		successResp(),          // ListPolicies base chain: input (no config)
		successResp(),          // ListPolicies base chain: output (no config)
		successResp(),          // CreatePolicy
		dataResp(getPolicy),     // GetPolicy
		successResp(),          // UpdatePolicy (Set default-action)
		successResp(),          // UpdatePolicy (Set description)
		dataResp(updatedPolicy), // UpdatePolicy (Get for response)
		successResp(),          // AddRule (Set action)
		successResp(),          // AddRule (Set source address)
		successResp(),          // DeleteRule
		successResp(),          // DeletePolicy
	)
	h := newHandler(client)

	t.Run("List", func(t *testing.T) {
		w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListPolicies)
		assertStatus(t, w, http.StatusOK)
		var list []map[string]interface{}
		decodeJSON(t, w, &list)
		if len(list) != 1 || list[0]["name"] != "LAN-IN" {
			t.Fatalf("list: got %v", list)
		}
	})

	t.Run("Create", func(t *testing.T) {
		body := map[string]string{"name": "TEST-IN", "default_action": "drop"}
		w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreatePolicy)
		assertStatus(t, w, http.StatusCreated)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["name"] != "TEST-IN" {
			t.Errorf("name = %v", out["name"])
		}
	})

	t.Run("Get", func(t *testing.T) {
		w := do(t, http.MethodGet, "/", nil, deviceVars("policy", "LAN-IN"), h.GetPolicy)
		assertStatus(t, w, http.StatusOK)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["name"] != "LAN-IN" {
			t.Errorf("name = %v", out["name"])
		}
		rules, _ := out["rules"].(map[string]interface{})
		if len(rules) != 1 {
			t.Errorf("rules len = %d", len(rules))
		}
	})

	t.Run("Update", func(t *testing.T) {
		body := map[string]string{"default_action": "accept", "description": "updated-desc"}
		w := do(t, http.MethodPut, "/", body, deviceVars("policy", "LAN-IN"), h.UpdatePolicy)
		assertStatus(t, w, http.StatusOK)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["default_action"] != "accept" {
			t.Errorf("default_action = %v", out["default_action"])
		}
	})

	t.Run("AddRule", func(t *testing.T) {
		body := map[string]interface{}{"rule_id": 10, "action": "accept", "source": "10.0.0.0/8"}
		w := do(t, http.MethodPost, "/", body, deviceVars("policy", "LAN-IN"), h.AddRule)
		assertStatus(t, w, http.StatusCreated)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["rule_id"] != float64(10) || out["action"] != "accept" {
			t.Errorf("got %v", out)
		}
	})

	t.Run("DeleteRule", func(t *testing.T) {
		w := do(t, http.MethodDelete, "/", nil, deviceVars("policy", "LAN-IN", "rule_id", "10"), h.DeleteRule)
		assertStatus(t, w, http.StatusNoContent)
	})

	t.Run("Delete", func(t *testing.T) {
		w := do(t, http.MethodDelete, "/", nil, deviceVars("policy", "LAN-IN"), h.DeletePolicy)
		assertStatus(t, w, http.StatusNoContent)
	})
}

func TestCRUD_AddressGroups(t *testing.T) {
	listData := map[string]interface{}{
		"RFC1918": map[string]interface{}{
			"address":     []interface{}{"10.0.0.0/8", "192.168.0.0/16"},
			"description": "private",
		},
	}
	getCfg := map[string]interface{}{
		"address":     []interface{}{"10.0.0.0/8", "192.168.0.0/16"},
		"description": "private",
	}
	_, _, client := newMockVyOS(t,
		dataResp(listData),   // ListAddressGroups
		successResp(),        // CreateAddressGroup (Set address 1)
		successResp(),        // CreateAddressGroup (Set address 2)
		dataResp(getCfg),     // GetAddressGroup
		successResp(),        // UpdateAddressGroup (Delete address)
		successResp(),        // UpdateAddressGroup (Set addr 1)
		successResp(),        // UpdateAddressGroup (Set addr 2)
		successResp(),        // DeleteAddressGroup
	)
	h := newHandler(client)

	t.Run("List", func(t *testing.T) {
		w := do(t, http.MethodGet, "/", nil, deviceVars(), h.ListAddressGroups)
		assertStatus(t, w, http.StatusOK)
		var list []map[string]interface{}
		decodeJSON(t, w, &list)
		if len(list) != 1 || list[0]["name"] != "RFC1918" {
			t.Fatalf("list: got %v", list)
		}
	})

	t.Run("Create", func(t *testing.T) {
		body := map[string]interface{}{"name": "TEST-GRP", "addresses": []string{"10.0.0.0/8", "172.16.0.0/12"}}
		w := do(t, http.MethodPost, "/", body, deviceVars(), h.CreateAddressGroup)
		assertStatus(t, w, http.StatusCreated)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["name"] != "TEST-GRP" {
			t.Errorf("name = %v", out["name"])
		}
		addrs, _ := out["addresses"].([]interface{})
		if len(addrs) != 2 {
			t.Errorf("addresses len = %d", len(addrs))
		}
	})

	t.Run("Get", func(t *testing.T) {
		w := do(t, http.MethodGet, "/", nil, deviceVars("group", "RFC1918"), h.GetAddressGroup)
		assertStatus(t, w, http.StatusOK)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		if out["name"] != "RFC1918" {
			t.Errorf("name = %v", out["name"])
		}
	})

	t.Run("Update", func(t *testing.T) {
		body := map[string]interface{}{"addresses": []string{"192.168.0.0/24", "192.168.1.0/24"}}
		w := do(t, http.MethodPut, "/", body, deviceVars("group", "RFC1918"), h.UpdateAddressGroup)
		assertStatus(t, w, http.StatusOK)
		var out map[string]interface{}
		decodeJSON(t, w, &out)
		addrs, _ := out["addresses"].([]interface{})
		if len(addrs) != 2 {
			t.Errorf("addresses = %v", addrs)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		w := do(t, http.MethodDelete, "/", nil, deviceVars("group", "RFC1918"), h.DeleteAddressGroup)
		assertStatus(t, w, http.StatusNoContent)
	})
}
