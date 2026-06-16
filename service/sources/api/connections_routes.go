package api

import (
	"net/http"
	"time"

	"portier/service/sources/connections"
	"portier/service/sources/domain"
)

// connectionsRoutes registers the live connections endpoint.
func (h *Handler) connectionsRoutes() []modularRoute {
	return []modularRoute{
		{method: http.MethodGet, path: "/api/connections", handler: h.handleConnections},
	}
}

// handleConnections returns the live TCP connections, UDP sessions, and per-rule
// traffic summaries. Read-only with no request input; it only reads the live
// registries + rules via the h.manager bridge (no socket open/close). Behavior —
// the GeneratedAt timestamp formatting, the nil→empty-slice normalization, the
// per-rule summaries, and the 200 LiveConnectionsResponse body via writeJSON — is
// identical to the pre-modularization handler.
func (h *Handler) handleConnections(w http.ResponseWriter, _ *http.Request) {
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

// buildRuleLiveSummary aggregates the live TCP connections and UDP sessions for a
// single rule into its RuleLiveSummary (connections-owned; behavior unchanged).
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

	return connections.RuleLiveSummary{
		RuleID:               rule.ID,
		RuleName:             rule.Name,
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
