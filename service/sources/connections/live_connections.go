package connections

// RuleLiveSummary is an aggregated live-traffic snapshot for one forwarding rule.
type RuleLiveSummary struct {
	RuleID               string  `json:"ruleId"`
	RuleName             string  `json:"ruleName"`
	Protocol             string  `json:"protocol"`
	ActiveTCPConnections int     `json:"activeTcpConnections"`
	ActiveUDPSessions    int     `json:"activeUdpSessions"`
	BytesIn              int64   `json:"bytesIn"`
	BytesOut             int64   `json:"bytesOut"`
	PacketsIn            int64   `json:"packetsIn"`
	PacketsOut           int64   `json:"packetsOut"`
	LastTrafficAt        *string `json:"lastTrafficAt"`
}

// LiveConnectionsResponse is the response body for GET /api/connections.
type LiveConnectionsResponse struct {
	GeneratedAt    string              `json:"generatedAt"`
	TCPConnections []TcpConnectionInfo `json:"tcpConnections"`
	UDPSessions    []UdpSessionInfo    `json:"udpSessions"`
	RuleSummaries  []RuleLiveSummary   `json:"ruleSummaries"`
}
