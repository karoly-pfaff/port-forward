package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"portier/cli/sources/version"
)

const defaultTimeout = 10 * time.Second

// Client is an HTTP client for the Portier management API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a Client for the given management API base URL.
func New(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// ConnectionError indicates that the management API was unreachable.
type ConnectionError struct {
	URL string
	Err error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("could not connect to Portier at %s", e.URL)
}

func (e *ConnectionError) Unwrap() error { return e.Err }

// APIError indicates a non-2xx response from the management API.
type APIError struct {
	StatusCode int
	Messages   []string
}

func (e *APIError) Error() string {
	if len(e.Messages) > 0 {
		return fmt.Sprintf("API error %d: %s", e.StatusCode, strings.Join(e.Messages, "; "))
	}
	return fmt.Sprintf("API error %d", e.StatusCode)
}

type apiErrorBody struct {
	Errors []string `json:"errors"`
}

func (c *Client) get(path string, out any) error {
	reqURL := c.baseURL + path
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("invalid request URL: %w", err)
	}
	req.Header.Set("User-Agent", "PortierCLI/"+version.Version)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &ConnectionError{URL: c.baseURL, Err: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody apiErrorBody
		_ = json.Unmarshal(body, &errBody)
		return &APIError{StatusCode: resp.StatusCode, Messages: errBody.Errors}
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parsing response JSON: %w", err)
	}
	return nil
}

// RuntimeInfo mirrors the response from GET /api/runtime.
type RuntimeInfo struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	Runtime        string `json:"runtime"`
	Platform       string `json:"platform"`
	Arch           string `json:"arch"`
	UptimeSeconds  int64  `json:"uptimeSeconds"`
	StartedAt      string `json:"startedAt"`
	ManagementHost string `json:"managementHost"`
	ManagementPort int    `json:"managementPort"`
	ConfigPath     string `json:"configPath"`
	StaticDir      string `json:"staticDir"`
	ServiceMode    bool   `json:"serviceMode"`
	PID            int    `json:"pid"`
}

// GetRuntime calls GET /api/runtime and returns the runtime info.
func (c *Client) GetRuntime() (*RuntimeInfo, error) {
	var info RuntimeInfo
	if err := c.get("/api/runtime", &info); err != nil {
		return nil, err
	}
	return &info, nil
}
