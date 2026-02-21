package handlers_test

import (
	"net/http"
	"testing"

	"github.com/valueiron/vyos-api/handlers"
)

func TestHealth_OK(t *testing.T) {
	h := handlers.New(nil)

	w := do(t, http.MethodGet, "/health", nil, nil, h.Health)
	assertStatus(t, w, http.StatusOK)

	var result map[string]string
	decodeJSON(t, w, &result)
	if result["status"] != "ok" {
		t.Errorf("status = %q, want ok", result["status"])
	}
}
