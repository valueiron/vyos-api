package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/vyos-api/handlers"
)

func TestListDevices_Empty(t *testing.T) {
	h := handlers.New(map[string]*handlers.Device{})
	r := httptest.NewRequest(http.MethodGet, "/devices", nil)
	w := httptest.NewRecorder()
	h.ListDevices(w, r)
	assertStatus(t, w, http.StatusOK)

	var result []map[string]interface{}
	decodeJSON(t, w, &result)
	if len(result) != 0 {
		t.Errorf("got %d devices, want 0", len(result))
	}
}

func TestListDevices_Healthy(t *testing.T) {
	_, _, client := newMockVyOS(t, successResp())
	h := newHandler(client)

	r := httptest.NewRequest(http.MethodGet, "/devices", nil)
	w := httptest.NewRecorder()
	h.ListDevices(w, r)
	assertStatus(t, w, http.StatusOK)

	var result []map[string]interface{}
	decodeJSON(t, w, &result)
	if len(result) != 1 {
		t.Fatalf("got %d devices, want 1", len(result))
	}
	if result[0]["id"] != "router1" {
		t.Errorf("id = %v, want router1", result[0]["id"])
	}
	if result[0]["healthy"] != true {
		t.Errorf("healthy = %v, want true", result[0]["healthy"])
	}
}

func TestListDevices_Unhealthy(t *testing.T) {
	_, srv, client := newMockVyOS(t)
	srv.Close()
	h := newHandler(client)

	r := httptest.NewRequest(http.MethodGet, "/devices", nil)
	w := httptest.NewRecorder()
	h.ListDevices(w, r)
	assertStatus(t, w, http.StatusOK)

	var result []map[string]interface{}
	decodeJSON(t, w, &result)
	if len(result) != 1 {
		t.Fatalf("got %d devices, want 1", len(result))
	}
	if result[0]["healthy"] != false {
		t.Errorf("healthy = %v, want false", result[0]["healthy"])
	}
}
