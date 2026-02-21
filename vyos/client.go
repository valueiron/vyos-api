package vyos

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Response is the VyOS API response envelope.
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   interface{} `json:"error"`
}

// Client talks to the VyOS HTTP API.
type Client struct {
	baseURL string
	key     string
	http    *http.Client
	Conf    *Conf
}

// Conf exposes configuration operations (Get, Set, Delete).
type Conf struct {
	client *Client
}

// NewClient returns a Client. If httpClient is nil, http.DefaultClient is used.
func NewClient(httpClient *http.Client) *Client {
	c := &Client{http: httpClient}
	if c.http == nil {
		c.http = http.DefaultClient
	}
	c.Conf = &Conf{client: c}
	return c
}

// WithURL sets the base URL (e.g. "https://192.168.1.1:443").
func (c *Client) WithURL(baseURL string) *Client {
	c.baseURL = strings.TrimSuffix(baseURL, "/")
	return c
}

// WithToken sets the API key sent as the "key" form field.
func (c *Client) WithToken(key string) *Client {
	c.key = key
	return c
}

// Insecure configures the HTTP client to skip TLS verification.
func (c *Client) Insecure() *Client {
	c.http = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	return c
}

// pathToArr converts a space-separated path to the array format expected by the VyOS API.
func pathToArr(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Fields(path)
}

func (c *Client) post(ctx context.Context, endpoint string, payload interface{}) (*Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("data", string(body))
	form.Set("key", c.key)
	reqBody := form.Encode()

	u := c.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader([]byte(reqBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return &out, &httpStatusError{code: resp.StatusCode}
	}
	return &out, nil
}

type httpStatusError struct{ code int }

func (e *httpStatusError) Error() string {
	return "vyos api: unexpected status " + strconv.Itoa(e.code)
}

// Get retrieves configuration at the given space-separated path.
// The third argument is ignored (for API compatibility).
func (conf *Conf) Get(ctx context.Context, path string, _ interface{}) (*Response, interface{}, error) {
	pathArr := pathToArr(path)
	out, err := conf.client.post(ctx, "/retrieve", map[string]interface{}{
		"op":   "showConfig",
		"path": pathArr,
	})
	if err != nil {
		return nil, nil, err
	}
	return out, nil, nil
}

// Set applies the given space-separated path (including value as path segments).
func (conf *Conf) Set(ctx context.Context, path string) (*Response, interface{}, error) {
	pathArr := pathToArr(path)
	out, err := conf.client.post(ctx, "/configure", map[string]interface{}{
		"op":   "set",
		"path": pathArr,
	})
	if err != nil {
		return nil, nil, err
	}
	return out, nil, nil
}

// Delete removes the node at the given space-separated path.
func (conf *Conf) Delete(ctx context.Context, path string) (*Response, interface{}, error) {
	pathArr := pathToArr(path)
	out, err := conf.client.post(ctx, "/configure", map[string]interface{}{
		"op":   "delete",
		"path": pathArr,
	})
	if err != nil {
		return nil, nil, err
	}
	return out, nil, nil
}
