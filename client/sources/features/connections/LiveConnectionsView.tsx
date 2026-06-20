import { useEffect, useRef, useState, type ReactElement } from "react";
import type { LiveConnectionsResponse, RuleLiveSummary, TcpConnectionInfo, UdpSessionInfo } from "@portier/shared";
import { fetchLiveConnections } from "../../api/portierApi.js";
import { formatBytes, formatDurationMs, formatEndpoint, formatTimestamp, formatTimestampOrNever, formatUdpModeLabel } from "../../utils/format.js";

type TabId = "tcp" | "udp" | "summary";
type ProtocolFilter = "" | "tcp" | "udp";
type StatusFilter = "" | "active" | "idle";

const AUTO_REFRESH_INTERVALS = [2, 5, 10, 30];

function totalBytes(summaries: RuleLiveSummary[]): number {
  return summaries.reduce((n, r) => n + r.bytesIn + r.bytesOut, 0);
}

function activeRuleCount(summaries: RuleLiveSummary[]): number {
  return summaries.filter((r) => r.activeTcpConnections > 0 || r.activeUdpSessions > 0).length;
}

function filterTcp(
  conns: TcpConnectionInfo[],
  protocol: ProtocolFilter,
  status: StatusFilter,
  ruleId: string
): TcpConnectionInfo[] {
  return conns.filter((c) => {
    if (protocol === "udp") return false;
    if (status === "idle") return false;
    if (ruleId && c.ruleId !== ruleId) return false;
    return true;
  });
}

function filterUdp(
  sessions: UdpSessionInfo[],
  protocol: ProtocolFilter,
  status: StatusFilter,
  ruleId: string
): UdpSessionInfo[] {
  return sessions.filter((s) => {
    if (protocol === "tcp") return false;
    if (status && s.status !== status) return false;
    if (ruleId && s.ruleId !== ruleId) return false;
    return true;
  });
}

function filterAndSortSummaries(
  summaries: RuleLiveSummary[],
  protocol: ProtocolFilter,
  status: StatusFilter,
  ruleId: string
): RuleLiveSummary[] {
  const filtered = summaries.filter((r) => {
    if (protocol && r.protocol !== protocol) return false;
    if (status === "active" && r.activeTcpConnections === 0 && r.activeUdpSessions === 0) return false;
    if (status === "idle" && (r.activeTcpConnections > 0 || r.activeUdpSessions > 0)) return false;
    if (ruleId && r.ruleId !== ruleId) return false;
    return true;
  });
  return [...filtered].sort((a, b) => {
    const aActive = a.activeTcpConnections + a.activeUdpSessions;
    const bActive = b.activeTcpConnections + b.activeUdpSessions;
    if (bActive !== aActive) return bActive - aActive;
    return a.ruleName.localeCompare(b.ruleName);
  });
}

export function LiveConnectionsView(): ReactElement {
  const [data, setData] = useState<LiveConnectionsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<TabId>("tcp");
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [autoRefreshInterval, setAutoRefreshInterval] = useState(2);
  const [protocolFilter, setProtocolFilter] = useState<ProtocolFilter>("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("");
  const [ruleFilter, setRuleFilter] = useState("");
  const fetchInFlightRef = useRef(false);

  async function load(): Promise<void> {
    if (fetchInFlightRef.current) return;
    fetchInFlightRef.current = true;
    setError(null);
    try {
      const result = await fetchLiveConnections();
      setData(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load connections.");
    } finally {
      fetchInFlightRef.current = false;
      setLoading(false);
    }
  }

  useEffect(() => {
    setLoading(true);
    void load();
  }, []); // load is defined in component scope; re-running on every render is intentional

  useEffect(() => {
    if (!autoRefresh) return;
    const id = setInterval(() => { void load(); }, autoRefreshInterval * 1000);
    return () => clearInterval(id);
  }, [autoRefresh, autoRefreshInterval]); // load does not depend on filter state

  const tcpConnections = data?.tcpConnections ?? [];
  const udpSessions = data?.udpSessions ?? [];
  const ruleSummaries = data?.ruleSummaries ?? [];

  const filteredTcp = filterTcp(tcpConnections, protocolFilter, statusFilter, ruleFilter);
  const filteredUdp = filterUdp(udpSessions, protocolFilter, statusFilter, ruleFilter);
  const filteredSummaries = filterAndSortSummaries(ruleSummaries, protocolFilter, statusFilter, ruleFilter);

  const hasFilters = !!(protocolFilter || statusFilter || ruleFilter);

  const subtitle = !loading && data
    ? `Updated at ${formatTimestamp(data.generatedAt)}`
    : "Active TCP connections, UDP sessions, and per-rule summaries";

  function handleClearFilters(): void {
    setProtocolFilter("");
    setStatusFilter("");
    setRuleFilter("");
  }

  return (
    <div className="rule-list-section">
      <div className="rule-list-header">
        <div className="rule-list-title-group">
          <div className="rule-list-title">Live Connections</div>
          <div className="rule-list-subtitle">{subtitle}</div>
        </div>
        <div className="rule-list-controls">
          <select
            className="filter-select"
            value={protocolFilter}
            onChange={(e) => setProtocolFilter(e.target.value as ProtocolFilter)}
            aria-label="Filter by protocol"
          >
            <option value="">All Protocols</option>
            <option value="tcp">TCP</option>
            <option value="udp">UDP</option>
          </select>
          <select
            className="filter-select"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as StatusFilter)}
            aria-label="Filter by status"
          >
            <option value="">All Statuses</option>
            <option value="active">Active</option>
            <option value="idle">Idle</option>
          </select>
          {ruleSummaries.length > 0 && (
            <select
              className="filter-select"
              value={ruleFilter}
              onChange={(e) => setRuleFilter(e.target.value)}
              aria-label="Filter by rule"
            >
              <option value="">All Rules</option>
              {ruleSummaries.map((r) => (
                <option key={r.ruleId} value={r.ruleId}>{r.ruleName}</option>
              ))}
            </select>
          )}
          {hasFilters && (
            <button
              type="button"
              className="btn-text"
              onClick={handleClearFilters}
              aria-label="Clear all filters"
            >
              Clear Filters
            </button>
          )}
        </div>
      </div>

      <div className="connections-summary" aria-label="Live connections summary">
        <div className="connections-summary-item">
          <span className="connections-summary-value">{tcpConnections.length}</span>
          <span className="connections-summary-label">TCP Connections</span>
        </div>
        <div className="connections-summary-item">
          <span className="connections-summary-value">{udpSessions.length}</span>
          <span className="connections-summary-label">UDP Sessions</span>
        </div>
        <div className="connections-summary-item">
          <span className="connections-summary-value">{activeRuleCount(ruleSummaries)}</span>
          <span className="connections-summary-label">Active Rules</span>
        </div>
        <div className="connections-summary-item">
          <span className="connections-summary-value">{formatBytes(totalBytes(ruleSummaries))}</span>
          <span className="connections-summary-label">Total Traffic</span>
        </div>
      </div>

      <div className="connections-tabs" role="tablist" aria-label="Connection views">
        <button
          type="button"
          role="tab"
          aria-selected={activeTab === "tcp"}
          className={`connections-tab${activeTab === "tcp" ? " connections-tab--active" : ""}`}
          onClick={() => setActiveTab("tcp")}
        >
          TCP ({tcpConnections.length})
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={activeTab === "udp"}
          className={`connections-tab${activeTab === "udp" ? " connections-tab--active" : ""}`}
          onClick={() => setActiveTab("udp")}
        >
          UDP ({udpSessions.length})
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={activeTab === "summary"}
          className={`connections-tab${activeTab === "summary" ? " connections-tab--active" : ""}`}
          onClick={() => setActiveTab("summary")}
        >
          Rule Summary ({ruleSummaries.length})
        </button>
      </div>

      <div className="rule-list-body">
        {error && (
          <div className="activity-error" role="alert">
            <p>{error}</p>
          </div>
        )}

        {!error && loading && (
          <p className="table-loading">Loading connections…</p>
        )}

        {!error && !loading && activeTab === "tcp" && (
          filteredTcp.length === 0 ? (
            <p className="empty">No active TCP connections.</p>
          ) : (
            <table aria-label="TCP connections">
              <thead>
                <tr>
                  <th>Rule</th>
                  <th>Client</th>
                  <th>Target</th>
                  <th>Duration</th>
                  <th>Bytes In</th>
                  <th>Bytes Out</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {filteredTcp.map((conn) => (
                  <tr key={conn.id}>
                    <td>{conn.ruleName}</td>
                    <td className="endpoint-cell">{formatEndpoint(conn.clientAddress, conn.clientPort)}</td>
                    <td className="endpoint-cell">{formatEndpoint(conn.targetAddress, conn.targetPort)}</td>
                    <td>{formatDurationMs(conn.durationMs)}</td>
                    <td>{formatBytes(conn.bytesIn)}</td>
                    <td>{formatBytes(conn.bytesOut)}</td>
                    <td>
                      <span className="conn-status-badge conn-status-badge--active">Active</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )
        )}

        {!error && !loading && activeTab === "udp" && (
          filteredUdp.length === 0 ? (
            <p className="empty">No active or recent UDP sessions.</p>
          ) : (
            <table aria-label="UDP sessions">
              <thead>
                <tr>
                  <th>Rule</th>
                  <th>Mode</th>
                  <th>Client</th>
                  <th>Target</th>
                  <th>Idle</th>
                  <th>Packets In / Out</th>
                  <th>Bytes In / Out</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {filteredUdp.map((sess) => (
                  <tr key={sess.id}>
                    <td>{sess.ruleName}</td>
                    <td>
                      <span className="udp-mode-label">{formatUdpModeLabel(sess.mode)}</span>
                    </td>
                    <td className="endpoint-cell">{formatEndpoint(sess.clientAddress, sess.clientPort)}</td>
                    <td className="endpoint-cell">{formatEndpoint(sess.targetAddress, sess.targetPort)}</td>
                    <td>{formatDurationMs(sess.idleMs)}</td>
                    <td>{sess.packetsIn} / {sess.packetsOut}</td>
                    <td>{formatBytes(sess.bytesIn)} / {formatBytes(sess.bytesOut)}</td>
                    <td>
                      <span className={`conn-status-badge conn-status-badge--${sess.status}`}>
                        {sess.status === "active" ? "Active" : "Idle"}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )
        )}

        {!error && !loading && activeTab === "summary" && (
          filteredSummaries.length === 0 ? (
            <p className="empty">No rule summaries available.</p>
          ) : (
            <table aria-label="Rule summaries">
              <thead>
                <tr>
                  <th>Rule</th>
                  <th>Protocol</th>
                  <th>TCP</th>
                  <th>UDP</th>
                  <th>Bytes In</th>
                  <th>Bytes Out</th>
                  <th>Packets In / Out</th>
                  <th>Last Traffic</th>
                </tr>
              </thead>
              <tbody>
                {filteredSummaries.map((r) => (
                  <tr key={r.ruleId}>
                    <td>{r.ruleName}</td>
                    <td>
                      <span className={`protocol-badge protocol-badge--${r.protocol}`}>
                        {r.protocol.toUpperCase()}
                      </span>
                    </td>
                    <td>{r.activeTcpConnections}</td>
                    <td>{r.activeUdpSessions}</td>
                    <td>{formatBytes(r.bytesIn)}</td>
                    <td>{formatBytes(r.bytesOut)}</td>
                    <td>{r.packetsIn} / {r.packetsOut}</td>
                    <td>{formatTimestampOrNever(r.lastTrafficAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )
        )}
      </div>

      <div className="rule-list-footer">
        <span>
          {loading ? "Loading…" : (
            activeTab === "tcp"
              ? `${filteredTcp.length} connection${filteredTcp.length !== 1 ? "s" : ""}`
              : activeTab === "udp"
                ? `${filteredUdp.length} session${filteredUdp.length !== 1 ? "s" : ""}`
                : `${filteredSummaries.length} rule${filteredSummaries.length !== 1 ? "s" : ""}`
          )}
        </span>
        <div className="rule-list-footer-right">
          <label className="auto-refresh-toggle" title="Auto-refresh">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
              aria-label="Auto-refresh"
            />
            <span className="auto-refresh-label">Auto-refresh</span>
            <span className="auto-refresh-track" aria-hidden="true">
              <span className="auto-refresh-thumb" />
            </span>
          </label>
          <select
            className="auto-refresh-interval"
            value={autoRefresh ? autoRefreshInterval : ""}
            onChange={(e) => setAutoRefreshInterval(Number(e.target.value))}
            disabled={!autoRefresh}
            aria-label="Auto-refresh interval"
          >
            <option value="" disabled>Off</option>
            {AUTO_REFRESH_INTERVALS.map((s) => (
              <option key={s} value={s}>{s}s</option>
            ))}
          </select>
          <button
            type="button"
            className="refresh-btn"
            onClick={() => { void load(); }}
            aria-label="Refresh"
          >
            ↻
          </button>
        </div>
      </div>
    </div>
  );
}
