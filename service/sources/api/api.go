package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"portier/service/sources/activity"
	"portier/service/sources/advisory"
	"portier/service/sources/configplan"
	"portier/service/sources/connections"
	"portier/service/sources/domain"
	"portier/service/sources/manager"
	"portier/service/sources/options"
	"portier/service/sources/static"
	"portier/service/sources/validation"
	"portier/service/sources/version"
)

const notFoundMessage = "API route was not found."

type Options struct {
	StaticDir      string
	Manager        *manager.Manager
	StartedAt      time.Time
	ServiceOptions options.Options
	Version        string
}

type Handler struct {
	staticDir       string
	staticAvailable bool
	manager         *manager.Manager
	startedAt       time.Time
	serviceOpts     options.Options
	version         string
}

func NewHandler(opts Options) *Handler {
	requestManager := opts.Manager
	if requestManager == nil {
		requestManager, _ = manager.New(nil)
	}
	startedAt := opts.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	ver := opts.Version
	if ver == "" {
		ver = version.Version
	}

	return &Handler{
		staticDir:       opts.StaticDir,
		staticAvailable: static.HasClient(opts.StaticDir),
		manager:         requestManager,
		startedAt:       startedAt,
		serviceOpts:     opts.ServiceOptions,
		version:         ver,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if isAPIPath(r.URL.Path) {
		h.serveAPI(w, r)
		return
	}

	if h.staticAvailable {
		static.ServeClient(w, r, h.staticDir)
		return
	}

	http.NotFound(w, r)
}

func isAPIPath(route string) bool {
	return route == "/api" || strings.HasPrefix(route, "/api/")
}

func (h *Handler) serveAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/api/health" {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"server": "go",
			"name":   "Portier",
		})
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/api/runtime" {
		h.serveRuntimeInfo(w)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/api/forwards" {
		writeJSON(w, http.StatusOK, rulesToResponses(h.manager.ListRules()))
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/api/forwards" {
		h.createForward(w, r)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/api/forwards/reorder" {
		h.reorderForwards(w, r)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/api/forwards/") {
		h.serveForwardByID(w, r)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/api/status" {
		writeJSON(w, http.StatusOK, h.manager.ListStatus())
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/api/ports/advisory" {
		servePortAdvisory(w, r)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/api/connections" {
		h.serveConnections(w)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/api/config/export" {
		writeJSON(w, http.StatusOK, h.manager.ExportConfig())
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/api/config/import" {
		h.importConfig(w, r)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/api/config/plan" {
		h.configPlan(w, r)
		return
	}

	if r.URL.Path == "/api/activity" {
		if r.Method == http.MethodGet {
			h.listActivity(w, r)
			return
		}
		if r.Method == http.MethodDelete {
			h.clearActivity(w)
			return
		}
	}

	writeJSON(w, http.StatusNotFound, map[string][]string{
		"errors": {notFoundMessage},
	})
}

func (h *Handler) listActivity(w http.ResponseWriter, r *http.Request) {
	params := activity.ListParams{}
	q := r.URL.Query()

	if limitStr := q.Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			params.Limit = n
		}
	}
	if ruleID := q.Get("ruleId"); ruleID != "" {
		params.RuleID = &ruleID
	}
	if eventType := q.Get("type"); eventType != "" {
		params.Type = &eventType
	}
	if severity := q.Get("severity"); severity != "" {
		params.Severity = &severity
	}

	events := h.manager.ListActivity(params)
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (h *Handler) clearActivity(w http.ResponseWriter) {
	h.manager.ClearActivity()
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createForward(w http.ResponseWriter, r *http.Request) {
	var input validation.ForwardRuleInput
	if !decodeRequest(w, r, &input) {
		return
	}

	rule, err := h.manager.CreateRule(input)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toRuleResponse(rule))
}

func (h *Handler) serveForwardByID(w http.ResponseWriter, r *http.Request) {
	remainder := strings.TrimPrefix(r.URL.Path, "/api/forwards/")
	if remainder == "" {
		writeJSON(w, http.StatusNotFound, map[string][]string{"errors": {notFoundMessage}})
		return
	}
	parts := strings.Split(remainder, "/")
	ruleID := parts[0]

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodPatch:
			h.updateForward(w, r, ruleID)
		case http.MethodDelete:
			h.deleteForward(w, ruleID)
		default:
			writeJSON(w, http.StatusNotFound, map[string][]string{"errors": {notFoundMessage}})
		}
		return
	}

	if len(parts) == 2 && r.Method == http.MethodPost {
		switch parts[1] {
		case "start":
			h.startForward(w, ruleID)
			return
		case "stop":
			h.stopForward(w, ruleID)
			return
		case "diagnose":
			h.diagnoseForward(w, ruleID)
			return
		}
	}

	writeJSON(w, http.StatusNotFound, map[string][]string{"errors": {notFoundMessage}})
}

func (h *Handler) updateForward(w http.ResponseWriter, r *http.Request, ruleID string) {
	raw, err := readBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {err.Error()}})
		return
	}
	patch, errors := validation.DecodeAndValidateForwardRulePatch(raw)
	if len(errors) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": errors})
		return
	}

	rule, err := h.manager.UpdateRule(ruleID, patch)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toRuleResponse(rule))
}

func (h *Handler) deleteForward(w http.ResponseWriter, ruleID string) {
	if err := h.manager.DeleteRule(ruleID); err != nil {
		writeManagerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) startForward(w http.ResponseWriter, ruleID string) {
	status, err := h.manager.StartRule(ruleID)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) stopForward(w http.ResponseWriter, ruleID string) {
	status, err := h.manager.StopRule(ruleID)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) diagnoseForward(w http.ResponseWriter, ruleID string) {
	var targetRule *domain.ForwardRule
	for _, r := range h.manager.ListRules() {
		if r.ID == ruleID {
			rule := r
			targetRule = &rule
			break
		}
	}
	if targetRule == nil {
		writeJSON(w, http.StatusNotFound, map[string][]string{
			"errors": {notFoundMessage},
		})
		return
	}

	var isRunning bool
	for _, s := range h.manager.ListStatus() {
		if s.RuleID == ruleID {
			isRunning = s.Running
			break
		}
	}

	result := diagnoseRule(*targetRule, isRunning)
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) reorderForwards(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.IDs == nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {"ids must be an array of strings."}})
		return
	}

	if err := h.manager.ReorderRules(body.IDs); err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rulesToResponses(h.manager.ListRules()))
}

func (h *Handler) configPlan(w http.ResponseWriter, r *http.Request) {
	raw, err := readBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {err.Error()}})
		return
	}

	var body struct {
		Desired *json.RawMessage `json:"desired"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {err.Error()}})
		return
	}
	if body.Desired == nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {"desired is required."}})
		return
	}

	plan := configplan.BuildConfigPlan(configplan.Input{
		CurrentRules: h.manager.ListRules(),
		DesiredRaw:   *body.Desired,
	})
	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) importConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode   string          `json:"mode"`
		Config json.RawMessage `json:"config"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.Mode != "replace" && body.Mode != "merge" {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {"mode must be replace or merge."}})
		return
	}

	var importedConfig struct {
		Version    string               `json:"version"`
		ExportedAt string               `json:"exportedAt"`
		Rules      []domain.ForwardRule `json:"rules"`
	}
	if len(body.Config) == 0 || !json.Valid(body.Config) {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {"config must be a valid Portier config object with version 1 and a rules array."}})
		return
	}
	if err := json.Unmarshal(body.Config, &importedConfig); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {"config must be a valid Portier config object with version 1 and a rules array."}})
		return
	}
	if importedConfig.Version != "1" || importedConfig.Rules == nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {"config must be a valid Portier config object with version 1 and a rules array."}})
		return
	}

	result, err := h.manager.ImportConfig(domain.ExportedConfig{
		Version:    importedConfig.Version,
		ExportedAt: importedConfig.ExportedAt,
		Rules:      importedConfig.Rules,
	}, body.Mode)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	if len(result.Errors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errors": result.Errors, "result": result})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result, "rules": rulesToResponses(h.manager.ListRules())})
}

func rulesToResponses(rules []domain.ForwardRule) []domain.ForwardRuleResponse {
	responses := make([]domain.ForwardRuleResponse, 0, len(rules))
	for _, rule := range rules {
		responses = append(responses, toRuleResponse(rule))
	}
	return responses
}

func servePortAdvisory(w http.ResponseWriter, r *http.Request) {
	port, err := strconv.Atoi(r.URL.Query().Get("port"))
	if err != nil || port < 1 || port > 65535 {
		writeJSON(w, http.StatusBadRequest, map[string][]string{
			"errors": {"port must be an integer from 1 to 65535."},
		})
		return
	}

	purpose := r.URL.Query().Get("purpose")
	if purpose != advisory.PurposeManagement && purpose != advisory.PurposeForward {
		writeJSON(w, http.StatusBadRequest, map[string][]string{
			"errors": {"purpose must be management or forward."},
		})
		return
	}

	writeJSON(w, http.StatusOK, advisory.GetPortAdvisories(advisory.Input{
		Port:       port,
		ListenHost: r.URL.Query().Get("listenHost"),
		Purpose:    purpose,
	}))
}

func (h *Handler) serveConnections(w http.ResponseWriter) {
	tcpConns := h.manager.GetLiveTCPConnections()
	if tcpConns == nil {
		tcpConns = make([]connections.TcpConnectionInfo, 0)
	}
	udpSessions := h.manager.GetLiveUDPSessions()
	if udpSessions == nil {
		udpSessions = make([]connections.UdpSessionInfo, 0)
	}

	rules := h.manager.ListRules()
	summaries := make([]connections.RuleLiveSummary, 0, len(rules))
	for _, rule := range rules {
		summaries = append(summaries, buildRuleLiveSummary(rule, tcpConns, udpSessions))
	}

	writeJSON(w, http.StatusOK, connections.LiveConnectionsResponse{
		GeneratedAt:    time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		TCPConnections: tcpConns,
		UDPSessions:    udpSessions,
		RuleSummaries:  summaries,
	})
}

func buildRuleLiveSummary(
	rule domain.ForwardRule,
	tcpConns []connections.TcpConnectionInfo,
	udpSessions []connections.UdpSessionInfo,
) connections.RuleLiveSummary {
	var bytesIn, bytesOut, packetsIn, packetsOut int64
	activeTCP := 0
	activeUDP := 0
	var lastTrafficAt *string

	for _, conn := range tcpConns {
		if conn.RuleID == rule.ID {
			activeTCP++
			bytesIn += conn.BytesIn
			bytesOut += conn.BytesOut
			if lastTrafficAt == nil || conn.StartedAt > *lastTrafficAt {
				t := conn.StartedAt
				lastTrafficAt = &t
			}
		}
	}

	for _, sess := range udpSessions {
		if sess.RuleID == rule.ID {
			activeUDP++
			bytesIn += sess.BytesIn
			bytesOut += sess.BytesOut
			packetsIn += sess.PacketsIn
			packetsOut += sess.PacketsOut
			if lastTrafficAt == nil || sess.LastSeenAt > *lastTrafficAt {
				t := sess.LastSeenAt
				lastTrafficAt = &t
			}
		}
	}

	name := rule.Name
	if name == "" {
		name = ""
	}

	return connections.RuleLiveSummary{
		RuleID:               rule.ID,
		RuleName:             name,
		Protocol:             string(rule.Protocol),
		ActiveTCPConnections: activeTCP,
		ActiveUDPSessions:    activeUDP,
		BytesIn:              bytesIn,
		BytesOut:             bytesOut,
		PacketsIn:            packetsIn,
		PacketsOut:           packetsOut,
		LastTrafficAt:        lastTrafficAt,
	}
}

func toRuleResponse(rule domain.ForwardRule) domain.ForwardRuleResponse {
	return domain.ForwardRuleResponse{
		ID:         rule.ID,
		Name:       rule.Name,
		Protocol:   rule.Protocol,
		ListenHost: rule.ListenHost,
		ListenPort: rule.ListenPort,
		TargetHost: rule.TargetHost,
		TargetPort: rule.TargetPort,
		Enabled:    rule.Enabled,
		UdpMode:    rule.UdpMode,
		Advisories: advisory.GetPortAdvisories(advisory.Input{
			Port:       rule.ListenPort,
			ListenHost: rule.ListenHost,
			Purpose:    advisory.PurposeForward,
		}),
	}
}

func (h *Handler) serveRuntimeInfo(w http.ResponseWriter) {
	uptimeSeconds := int64(time.Since(h.startedAt).Seconds())
	writeJSON(w, http.StatusOK, map[string]any{
		"name":           "Portier",
		"version":        h.version,
		"runtime":        "go",
		"platform":       normalizePlatform(),
		"arch":           normalizeArch(),
		"uptimeSeconds":  uptimeSeconds,
		"startedAt":      h.startedAt.UTC().Format(time.RFC3339),
		"managementHost": h.serviceOpts.Host,
		"managementPort": h.serviceOpts.Port,
		"configPath":     h.serviceOpts.ConfigPath,
		"staticDir":      h.serviceOpts.StaticDir,
		"serviceMode":    h.serviceOpts.Service,
		"pid":            os.Getpid(),
	})
}

func normalizePlatform() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "macos"
	case "linux":
		return "linux"
	default:
		return "unknown"
	}
}

func normalizeArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	default:
		return "unknown"
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	raw, err := readBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {err.Error()}})
		return false
	}
	if err := json.Unmarshal(raw, target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": {err.Error()}})
		return false
	}
	return true
}

func readBody(r *http.Request) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1_000_000))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, errors.New("request body must be JSON.")
	}
	return raw, nil
}

func writeManagerError(w http.ResponseWriter, err error) {
	var validationError manager.ValidationError
	if errors.As(err, &validationError) {
		writeJSON(w, http.StatusBadRequest, map[string][]string{"errors": validationError.Errors})
		return
	}

	var conflictError manager.ConflictError
	if errors.As(err, &conflictError) {
		writeJSON(w, http.StatusConflict, map[string][]string{"errors": {conflictError.Message}})
		return
	}

	var notFoundError manager.NotFoundError
	if errors.As(err, &notFoundError) {
		writeJSON(w, http.StatusNotFound, map[string][]string{"errors": {notFoundError.Message}})
		return
	}

	writeJSON(w, http.StatusInternalServerError, map[string][]string{"errors": {err.Error()}})
}
