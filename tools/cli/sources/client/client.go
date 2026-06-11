package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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

// do executes an HTTP request and unmarshals the JSON response into out.
// If out is nil the response body is read and checked for errors but not parsed.
func (c *Client) do(method, path string, out any) error {
	reqURL := c.baseURL + path
	req, err := http.NewRequest(method, reqURL, nil)
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

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("parsing response JSON: %w", err)
		}
	}
	return nil
}

func (c *Client) get(path string, out any) error {
	return c.do(http.MethodGet, path, out)
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

// PortAdvisory mirrors a port advisory from the API.
type PortAdvisory struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// ForwardRuleResponse mirrors the response from GET /api/forwards.
type ForwardRuleResponse struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Protocol   string         `json:"protocol"`
	ListenHost string         `json:"listenHost"`
	ListenPort int            `json:"listenPort"`
	TargetHost string         `json:"targetHost"`
	TargetPort int            `json:"targetPort"`
	Enabled    bool           `json:"enabled"`
	UDPMode    string         `json:"udpMode,omitempty"`
	Group      string         `json:"group,omitempty"`
	Advisories []PortAdvisory `json:"advisories"`
}

// ForwardStatus mirrors the response from GET /api/status.
type ForwardStatus struct {
	RuleID            string `json:"ruleId"`
	Running           bool   `json:"running"`
	ActiveConnections *int   `json:"activeConnections,omitempty"`
	BytesIn           int64  `json:"bytesIn"`
	BytesOut          int64  `json:"bytesOut"`
	PacketsIn         *int64 `json:"packetsIn,omitempty"`
	PacketsOut        *int64 `json:"packetsOut,omitempty"`
	ActiveUDPSessions *int   `json:"activeUdpSessions,omitempty"`
	LastError         string `json:"lastError,omitempty"`
	StartedAt         string `json:"startedAt,omitempty"`
}

// ActivityEvent mirrors an event from GET /api/activity.
type ActivityEvent struct {
	ID        string         `json:"id"`
	Timestamp string         `json:"timestamp"`
	Type      string         `json:"type"`
	Severity  string         `json:"severity"`
	RuleID    string         `json:"ruleId,omitempty"`
	RuleName  string         `json:"ruleName,omitempty"`
	Protocol  string         `json:"protocol,omitempty"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
}

// ActivityQuery holds optional filters for GetActivity.
type ActivityQuery struct {
	Limit    int
	RuleID   string
	Type     string
	Severity string
}

// GetForwards calls GET /api/forwards and returns the list of rules.
func (c *Client) GetForwards() ([]ForwardRuleResponse, error) {
	var rules []ForwardRuleResponse
	if err := c.get("/api/forwards", &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// GetStatus calls GET /api/status and returns runtime status for all rules.
func (c *Client) GetStatus() ([]ForwardStatus, error) {
	var statuses []ForwardStatus
	if err := c.get("/api/status", &statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

// DiagnosticCheck mirrors a single check result from POST /api/forwards/:id/diagnose.
type DiagnosticCheck struct {
	ID      string         `json:"id"`
	Label   string         `json:"label"`
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// DiagnosticSummary mirrors the summary field from POST /api/forwards/:id/diagnose.
type DiagnosticSummary struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// RuleDiagnosticsResult mirrors the response from POST /api/forwards/:id/diagnose.
type RuleDiagnosticsResult struct {
	RuleID      string            `json:"ruleId"`
	RuleName    string            `json:"ruleName"`
	Protocol    string            `json:"protocol"`
	Summary     DiagnosticSummary `json:"summary"`
	Checks      []DiagnosticCheck `json:"checks"`
	DiagnosedAt string            `json:"diagnosedAt"`
}

// ConfigRule is a forwarding rule as stored in a Portier config file.
type ConfigRule struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	ListenHost string `json:"listenHost"`
	ListenPort int    `json:"listenPort"`
	TargetHost string `json:"targetHost"`
	TargetPort int    `json:"targetPort"`
	Enabled    bool   `json:"enabled"`
	UDPMode    string `json:"udpMode,omitempty"`
	Group      string `json:"group,omitempty"`
}

// ConfigExportResponse mirrors the response from GET /api/config/export.
type ConfigExportResponse struct {
	Version    string       `json:"version"`
	ExportedAt string       `json:"exportedAt"`
	Rules      []ConfigRule `json:"rules"`
}

// ConfigImportRequest is the request body for POST /api/config/import.
type ConfigImportRequest struct {
	Mode   string               `json:"mode"`
	Config ConfigExportResponse `json:"config"`
}

// ImportResult mirrors the result field from a successful config import response.
type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors"`
}

// ConfigImportResponse mirrors the response from POST /api/config/import.
type ConfigImportResponse struct {
	Result ImportResult          `json:"result"`
	Rules  []ForwardRuleResponse `json:"rules"`
}

// ConfigPlanRuleSnapshot mirrors a rule snapshot in a plan response.
type ConfigPlanRuleSnapshot struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	ListenHost string `json:"listenHost"`
	ListenPort int    `json:"listenPort"`
	TargetHost string `json:"targetHost"`
	TargetPort int    `json:"targetPort"`
	Enabled    bool   `json:"enabled"`
	UDPMode    string `json:"udpMode,omitempty"`
	Group      string `json:"group,omitempty"`
}

// ConfigPlanChange describes a field-level change in a plan update operation.
type ConfigPlanChange struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

// ConfigPlanOperation describes one add/update/remove/unchanged operation.
type ConfigPlanOperation struct {
	Type        string                  `json:"type"`
	RuleID      string                  `json:"ruleId,omitempty"`
	RuleName    string                  `json:"ruleName"`
	Protocol    string                  `json:"protocol"`
	Current     *ConfigPlanRuleSnapshot `json:"current,omitempty"`
	Desired     *ConfigPlanRuleSnapshot `json:"desired,omitempty"`
	Changes     []ConfigPlanChange      `json:"changes,omitempty"`
	Destructive bool                    `json:"destructive"`
}

// ConfigPlanSummary contains operation counts for the plan response.
type ConfigPlanSummary struct {
	Add         int  `json:"add"`
	Update      int  `json:"update"`
	Remove      int  `json:"remove"`
	Unchanged   int  `json:"unchanged"`
	Destructive int  `json:"destructive"`
	HasDrift    bool `json:"hasDrift"`
	HasErrors   bool `json:"hasErrors"`
}

// ConfigPlanError is a structured error in a plan response.
type ConfigPlanError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// ConfigPlanWarning is a warning in a plan response.
type ConfigPlanWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ConfigPlanResponse mirrors the response from POST /api/config/plan.
type ConfigPlanResponse struct {
	GeneratedAt string                `json:"generatedAt"`
	Mode        string                `json:"mode"`
	Summary     ConfigPlanSummary     `json:"summary"`
	Operations  []ConfigPlanOperation `json:"operations"`
	Errors      []ConfigPlanError     `json:"errors"`
	Warnings    []ConfigPlanWarning   `json:"warnings"`
}

// ConfigPlanDesired is the desired config sent to POST /api/config/plan.
type ConfigPlanDesired struct {
	Rules []ConfigRule `json:"rules"`
}

// ConfigPlanRequest is the request body for POST /api/config/plan.
type ConfigPlanRequest struct {
	Desired ConfigPlanDesired `json:"desired"`
}

// doWithBody executes an HTTP request with a JSON body and unmarshals the response into out.
// If out is nil the response body is checked for errors but not parsed.
func (c *Client) doWithBody(method, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encoding request body: %w", err)
	}
	reqURL := c.baseURL + path
	req, err := http.NewRequest(method, reqURL, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("invalid request URL: %w", err)
	}
	req.Header.Set("User-Agent", "PortierCLI/"+version.Version)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &ConnectionError{URL: c.baseURL, Err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody apiErrorBody
		_ = json.Unmarshal(respBody, &errBody)
		return &APIError{StatusCode: resp.StatusCode, Messages: errBody.Errors}
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("parsing response JSON: %w", err)
		}
	}
	return nil
}

// ExportConfig calls GET /api/config/export and returns the current config.
func (c *Client) ExportConfig() (*ConfigExportResponse, error) {
	var cfg ConfigExportResponse
	if err := c.get("/api/config/export", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ImportConfig calls POST /api/config/import with the given request body.
func (c *Client) ImportConfig(req ConfigImportRequest) (*ConfigImportResponse, error) {
	var resp ConfigImportResponse
	if err := c.doWithBody(http.MethodPost, "/api/config/import", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PlanConfig calls POST /api/config/plan and returns the plan response.
func (c *Client) PlanConfig(req ConfigPlanRequest) (*ConfigPlanResponse, error) {
	var resp ConfigPlanResponse
	if err := c.doWithBody(http.MethodPost, "/api/config/plan", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ConfigAppliedCounts holds the count of each operation type from an apply response.
type ConfigAppliedCounts struct {
	Add       int `json:"add"`
	Update    int `json:"update"`
	Remove    int `json:"remove"`
	Unchanged int `json:"unchanged"`
}

// ConfigApplyRequest is the request body for POST /api/config/apply.
type ConfigApplyRequest struct {
	Desired ConfigPlanDesired `json:"desired"`
	Yes     bool              `json:"yes"`
	DryRun  bool              `json:"dryRun,omitempty"`
}

// ConfigApplyResponse mirrors the response from POST /api/config/apply.
type ConfigApplyResponse struct {
	Ok        bool                `json:"ok"`
	DryRun    bool                `json:"dryRun"`
	AppliedAt string              `json:"appliedAt"`
	Plan      ConfigPlanResponse  `json:"plan"`
	Applied   ConfigAppliedCounts `json:"applied"`
}

// ApplyConfig calls POST /api/config/apply and returns the apply response.
// Returns an APIError for 400 responses (e.g. destructive without yes).
func (c *Client) ApplyConfig(req ConfigApplyRequest) (*ConfigApplyResponse, error) {
	var resp ConfigApplyResponse
	if err := c.doWithBody(http.MethodPost, "/api/config/apply", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// StartForward calls POST /api/forwards/:id/start to start a forwarding rule.
func (c *Client) StartForward(id string) error {
	return c.do(http.MethodPost, "/api/forwards/"+id+"/start", nil)
}

// StopForward calls POST /api/forwards/:id/stop to stop a forwarding rule.
func (c *Client) StopForward(id string) error {
	return c.do(http.MethodPost, "/api/forwards/"+id+"/stop", nil)
}

// DiagnoseForward calls POST /api/forwards/:id/diagnose and returns the result.
func (c *Client) DiagnoseForward(id string) (*RuleDiagnosticsResult, error) {
	var result RuleDiagnosticsResult
	if err := c.do(http.MethodPost, "/api/forwards/"+id+"/diagnose", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BaseURL returns the management API base URL used by this client.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// GetActivity calls GET /api/activity with optional filters and returns events.
func (c *Client) GetActivity(q ActivityQuery) ([]ActivityEvent, error) {
	params := url.Values{}
	if q.Limit > 0 {
		params.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.RuleID != "" {
		params.Set("ruleId", q.RuleID)
	}
	if q.Type != "" {
		params.Set("type", q.Type)
	}
	if q.Severity != "" {
		params.Set("severity", q.Severity)
	}
	path := "/api/activity"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var resp struct {
		Events []ActivityEvent `json:"events"`
	}
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return resp.Events, nil
}
