package handlers

import (
	"context"
	"net/http"
	"time"
)

// DeviceInfo is the API representation of a registered VyOS device.
type DeviceInfo struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Healthy bool   `json:"healthy"`
}

// ListDevices handles GET /devices.
// Returns all registered devices with a connectivity probe result.
func (h *Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	result := make([]DeviceInfo, 0, len(h.devices))
	for _, d := range h.devices {
		healthy := probe(r.Context(), d)
		result = append(result, DeviceInfo{
			ID:      d.ID,
			URL:     d.URL,
			Healthy: healthy,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

// probe attempts a lightweight retrieve against the device to check connectivity.
func probe(ctx context.Context, d *Device) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, _, err := d.Client.Conf.Get(ctx, "system host-name", nil)
	if err != nil {
		return false
	}
	return out.Success
}
