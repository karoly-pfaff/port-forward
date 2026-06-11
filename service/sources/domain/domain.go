package domain

type ForwardProtocol string

const (
	ProtocolTCP ForwardProtocol = "tcp"
	ProtocolUDP ForwardProtocol = "udp"
)

type UdpMode string

const (
	UdpModeOneWay             UdpMode = "one-way"
	UdpModeBidirectionalLast  UdpMode = "bidirectional-last-client"
	UdpModeBidirectionalMulti UdpMode = "bidirectional-multi-client"
)

type ForwardRule struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Protocol   ForwardProtocol `json:"protocol"`
	ListenHost string          `json:"listenHost"`
	ListenPort int             `json:"listenPort"`
	TargetHost string          `json:"targetHost"`
	TargetPort int             `json:"targetPort"`
	Enabled    bool            `json:"enabled"`
	UdpMode    *UdpMode        `json:"udpMode,omitempty"`
	// Group is an optional, behavior-neutral grouping label (v1.8 Operator
	// Power Tools). It is operator-facing metadata only — it does NOT affect
	// forwarding, duplicate-binding, lifecycle, or status behavior. nil when
	// the rule has no group. Normalized/validated by the validation package.
	Group *string `json:"group,omitempty"`
}

// GroupMaxLength is the maximum length (runes) of a normalized rule group label.
const GroupMaxLength = 64

type PortAdvisory struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type ForwardRuleResponse struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Protocol   ForwardProtocol `json:"protocol"`
	ListenHost string          `json:"listenHost"`
	ListenPort int             `json:"listenPort"`
	TargetHost string          `json:"targetHost"`
	TargetPort int             `json:"targetPort"`
	Enabled    bool            `json:"enabled"`
	UdpMode    *UdpMode        `json:"udpMode,omitempty"`
	Group      *string         `json:"group,omitempty"`
	Advisories []PortAdvisory  `json:"advisories"`
}

type ForwardStatus struct {
	RuleID            string `json:"ruleId"`
	Running           bool   `json:"running"`
	ActiveConnections *int   `json:"activeConnections,omitempty"`
	BytesIn           int64  `json:"bytesIn"`
	BytesOut          int64  `json:"bytesOut"`
	PacketsIn         *int64 `json:"packetsIn,omitempty"`
	PacketsOut        *int64 `json:"packetsOut,omitempty"`
	ActiveUdpSessions *int   `json:"activeUdpSessions,omitempty"`
	LastError         string `json:"lastError,omitempty"`
	StartedAt         string `json:"startedAt,omitempty"`
}

type ExportedConfig struct {
	Version    string        `json:"version"`
	ExportedAt string        `json:"exportedAt"`
	Rules      []ForwardRule `json:"rules"`
}

type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors"`
}

type DiagnosticStatus string

const (
	DiagnosticPass DiagnosticStatus = "pass"
	DiagnosticWarn DiagnosticStatus = "warn"
	DiagnosticFail DiagnosticStatus = "fail"
	DiagnosticSkip DiagnosticStatus = "skip"
)

type DiagnosticCheck struct {
	ID      string           `json:"id"`
	Label   string           `json:"label"`
	Status  DiagnosticStatus `json:"status"`
	Message string           `json:"message"`
	Details map[string]any   `json:"details,omitempty"`
}

type DiagnosticSummary struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type RuleDiagnosticsResult struct {
	RuleID      string            `json:"ruleId"`
	RuleName    string            `json:"ruleName"`
	Protocol    ForwardProtocol   `json:"protocol"`
	Summary     DiagnosticSummary `json:"summary"`
	Checks      []DiagnosticCheck `json:"checks"`
	DiagnosedAt string            `json:"diagnosedAt"`
}
