package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/valueiron/vyos-api/handlers"
	"github.com/valueiron/vyos-api/vyos"
	"github.com/gorilla/mux"
)

// --------------------------------------------------------------------------
// VyOS mock server
// --------------------------------------------------------------------------

// vyosResp mirrors the VyOS API response envelope.
type vyosResp struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   interface{} `json:"error"`
}

// vyosReq is the parsed body of a single VyOS API operation.
type vyosReq struct {
	Op   string   `json:"op"`
	Path []string `json:"path"`
}

// mockVyOS is a sequenced-response test double for the VyOS HTTP API.
// Each call to ServeHTTP consumes the next response from the queue; when the
// queue is exhausted it falls back to a generic success response.
type mockVyOS struct {
	mu        sync.Mutex
	responses []vyosResp
	// Requests received, for optional inspection by tests.
	Received []vyosReq
}

func (m *mockVyOS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// ParseForm handles both application/x-www-form-urlencoded and multipart/form-data.
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form: "+err.Error(), http.StatusBadRequest)
		return
	}
	data := r.FormValue("data")

	// Set sends a JSON array; Get/Delete send a single object.
	var reqs []vyosReq
	if strings.HasPrefix(strings.TrimSpace(data), "[") {
		json.Unmarshal([]byte(data), &reqs) //nolint:errcheck
	} else {
		var single vyosReq
		json.Unmarshal([]byte(data), &single) //nolint:errcheck
		reqs = []vyosReq{single}
	}

	m.mu.Lock()
	m.Received = append(m.Received, reqs...)
	var resp vyosResp
	if len(m.responses) > 0 {
		resp = m.responses[0]
		m.responses = m.responses[1:]
	} else {
		resp = vyosResp{Success: true}
	}
	m.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// newMockVyOS creates an httptest.Server backed by a mockVyOS with the given
// queued responses, and a Client pointing at it.
func newMockVyOS(t *testing.T, responses ...vyosResp) (*mockVyOS, *httptest.Server, *vyos.Client) {
	t.Helper()
	m := &mockVyOS{responses: responses}
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	client := vyos.NewClient(nil).WithURL(srv.URL).WithToken("testkey")
	return m, srv, client
}

// successResp returns a generic success response with no data.
func successResp() vyosResp { return vyosResp{Success: true} }

// dataResp returns a success response with the given data payload.
func dataResp(data interface{}) vyosResp { return vyosResp{Success: true, Data: data} }

// failResp returns a VyOS-level rejection response.
func failResp(msg string) vyosResp { return vyosResp{Success: false, Error: msg} }

// --------------------------------------------------------------------------
// Handler factory
// --------------------------------------------------------------------------

// newHandler creates a Handler with a single registered device "router1".
func newHandler(client *vyos.Client) *handlers.Handler {
	return handlers.New(map[string]*handlers.Device{
		"router1": {ID: "router1", URL: "http://test-device", Client: client},
	})
}

// --------------------------------------------------------------------------
// HTTP test helpers
// --------------------------------------------------------------------------

// do calls fn with a synthetic request.  path is used only for the URL
// (query params included); mux path variables must be supplied via vars.
func do(t *testing.T, method, path string, body interface{}, vars map[string]string, fn http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	var rb io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rb = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, path, rb)
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if vars != nil {
		r = mux.SetURLVars(r, vars)
	}
	w := httptest.NewRecorder()
	fn(w, r)
	return w
}

// deviceVars returns mux vars with just device_id set to "router1".
func deviceVars(extra ...string) map[string]string {
	m := map[string]string{"device_id": "router1"}
	for i := 0; i+1 < len(extra); i += 2 {
		m[extra[i]] = extra[i+1]
	}
	return m
}

// decodeJSON unmarshals the response body into v and fails the test on error.
func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("decode response body: %v\nbody: %s", err, w.Body.String())
	}
}

// assertStatus fails the test if the recorded status code doesn't match want.
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status = %d, want %d\nbody: %s", w.Code, want, w.Body.String())
	}
}

// unknownDeviceVars returns vars with a device_id not in the handler.
func unknownDeviceVars() map[string]string {
	return map[string]string{"device_id": "does-not-exist"}
}
