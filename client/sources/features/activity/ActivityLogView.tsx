import { useEffect, useRef, useState, type ReactElement } from "react";
import type { ActivityEvent, ActivityEventType, ActivitySeverity, ForwardRuleResponse } from "@portier/shared";
import { clearActivity, fetchActivity } from "../../api/portierApi.js";
import { formatTimestamp } from "../../utils/format.js";

const SEVERITY_OPTIONS: { value: "" | ActivitySeverity; label: string }[] = [
  { value: "", label: "All Severities" },
  { value: "info", label: "Info" },
  { value: "success", label: "Success" },
  { value: "warning", label: "Warning" },
  { value: "error", label: "Error" }
];

const TYPE_OPTIONS: { value: "" | ActivityEventType; label: string }[] = [
  { value: "", label: "All Types" },
  { value: "rule.created", label: "Rule created" },
  { value: "rule.updated", label: "Rule updated" },
  { value: "rule.deleted", label: "Rule deleted" },
  { value: "rule.started", label: "Rule started" },
  { value: "rule.stopped", label: "Rule stopped" },
  { value: "rule.error", label: "Rule error" },
  { value: "tcp.connection.opened", label: "TCP connection opened" },
  { value: "tcp.connection.closed", label: "TCP connection closed" },
  { value: "tcp.connection.error", label: "TCP connection error" },
  { value: "udp.packet.forwarded", label: "UDP packet forwarded" },
  { value: "udp.packet.returned", label: "UDP packet returned" },
  { value: "udp.packet.error", label: "UDP packet error" },
  { value: "udp.session.opened", label: "UDP session opened" },
  { value: "udp.session.closed", label: "UDP session closed" },
  { value: "config.exported", label: "Config exported" },
  { value: "config.imported", label: "Config imported" },
  { value: "config.import.failed", label: "Config import failed" }
];

const LIMIT_OPTIONS = [25, 50, 100, 200, 500];

const AUTO_REFRESH_INTERVALS = [2, 5, 10, 30];

function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

function severityLabel(severity: ActivitySeverity): string {
  return severity.charAt(0).toUpperCase() + severity.slice(1);
}

function typeLabel(type: string): string {
  return type.replace(/\./g, " · ");
}

function buildExportFilename(): string {
  const now = new Date();
  const pad = (n: number): string => String(n).padStart(2, "0");
  const date = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}`;
  const time = `${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
  return `portier-activity-${date}-${time}.json`;
}

interface ActivityLogViewProps {
  rules?: ForwardRuleResponse[];
  ruleId?: string;
  /** Initial severity filter — set when navigating here from an error affordance
   *  (the Error summary card or a rule's error health badge). */
  severity?: ActivitySeverity;
  onClearRuleFilter?: () => void;
}

export function ActivityLogView({ rules, ruleId, severity, onClearRuleFilter }: ActivityLogViewProps): ReactElement {
  const [events, setEvents] = useState<ActivityEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [clearError, setClearError] = useState<string | null>(null);
  const [limit, setLimit] = useState(100);
  const [severityFilter, setSeverityFilter] = useState<"" | ActivitySeverity>(severity ?? "");
  const [type, setType] = useState<"" | ActivityEventType>("");
  const [ruleIdFilter, setRuleIdFilter] = useState(ruleId ?? "");
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [autoRefreshInterval, setAutoRefreshInterval] = useState(5);
  const fetchInFlightRef = useRef(false);

  useEffect(() => {
    setRuleIdFilter(ruleId ?? "");
  }, [ruleId]);

  useEffect(() => {
    setSeverityFilter(severity ?? "");
  }, [severity]);

  async function load(opts: {
    limit: number;
    severity: "" | ActivitySeverity;
    type: "" | ActivityEventType;
    ruleId: string;
  }): Promise<void> {
    /* v8 ignore next -- concurrency guard: skips an overlapping load while one is in flight; not deterministically triggerable in a unit test */
    if (fetchInFlightRef.current) return;
    fetchInFlightRef.current = true;
    setError(null);
    try {
      const result = await fetchActivity({
        limit: opts.limit,
        severity: opts.severity || undefined,
        type: opts.type || undefined,
        ruleId: opts.ruleId || undefined
      });
      setEvents(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load activity.");
    } finally {
      fetchInFlightRef.current = false;
      setLoading(false);
    }
  }

  useEffect(() => {
    setLoading(true);
    void load({ limit, severity: severityFilter, type, ruleId: ruleIdFilter });
  }, [limit, severityFilter, type, ruleIdFilter]);

  useEffect(() => {
    if (!autoRefresh) return;
    const id = setInterval(() => {
      void load({ limit, severity: severityFilter, type, ruleId: ruleIdFilter });
    }, autoRefreshInterval * 1000);
    return () => clearInterval(id);
  }, [autoRefresh, autoRefreshInterval, limit, severityFilter, type, ruleIdFilter]);

  function handleRefresh(): void {
    void load({ limit, severity: severityFilter, type, ruleId: ruleIdFilter });
  }

  function handleRuleChange(e: React.ChangeEvent<HTMLSelectElement>): void {
    const val = e.target.value;
    setRuleIdFilter(val);
    if (!val) onClearRuleFilter?.();
  }

  async function handleClearActivity(): Promise<void> {
    setClearError(null);
    try {
      await clearActivity();
      setEvents([]);
    } catch (err) {
      setClearError(err instanceof Error ? err.message : "Failed to clear activity.");
    }
  }

  function handleExport(): void {
    const activeFilters: Record<string, string | number> = {};
    if (ruleIdFilter) activeFilters.ruleId = ruleIdFilter;
    if (severityFilter) activeFilters.severity = severityFilter;
    if (type) activeFilters.type = type;
    activeFilters.limit = limit;

    const payload = {
      exportedAt: new Date().toISOString(),
      filters: activeFilters,
      events
    };

    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = buildExportFilename();
    a.click();
    URL.revokeObjectURL(url);
  }

  const activeRuleName = ruleIdFilter
    ? (rules?.find((r) => r.id === ruleIdFilter)?.name ?? ruleIdFilter)
    : null;

  const hasActiveFilters = !!(ruleIdFilter || severityFilter || type);

  return (
    <div className="rule-list-section">
      <div className="rule-list-header">
        <div className="rule-list-title-group">
          <div className="rule-list-title">Activity Log</div>
          <div className="rule-list-subtitle">Recent forwarding and rule events</div>
        </div>
        <div className="rule-list-controls">
          {rules && rules.length > 0 && (
            <select
              className="filter-select"
              value={ruleIdFilter}
              onChange={handleRuleChange}
              aria-label="Filter by rule"
            >
              <option value="">All Rules</option>
              {rules.map((r) => (
                <option key={r.id} value={r.id}>{r.name}</option>
              ))}
            </select>
          )}
          <select
            className="filter-select"
            value={severityFilter}
            onChange={(e) => setSeverityFilter(e.target.value as "" | ActivitySeverity)}
            aria-label="Filter by severity"
          >
            {SEVERITY_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
          <select
            className="filter-select"
            value={type}
            onChange={(e) => setType(e.target.value as "" | ActivityEventType)}
            aria-label="Filter by type"
          >
            {TYPE_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
          <select
            className="filter-select"
            value={limit}
            onChange={(e) => setLimit(Number(e.target.value))}
            aria-label="Event limit"
          >
            {LIMIT_OPTIONS.map((n) => (
              <option key={n} value={n}>Last {n}</option>
            ))}
          </select>
          {hasActiveFilters && (
            <button
              type="button"
              className="btn-text"
              onClick={() => {
                setRuleIdFilter("");
                setSeverityFilter("");
                setType("");
                onClearRuleFilter?.();
              }}
              aria-label="Clear all filters"
              title="Clear all filters"
            >
              Clear Filters
            </button>
          )}
        </div>
      </div>

      {activeRuleName && (
        <div className="activity-filter-banner" role="status">
          <span>Filtered to rule: <strong>{activeRuleName}</strong></span>
          <button
            type="button"
            className="btn-text"
            onClick={() => {
              setRuleIdFilter("");
              onClearRuleFilter?.();
            }}
          >
            Clear
          </button>
        </div>
      )}

      <div className="rule-list-body">
        {error && (
          <div className="activity-error" role="alert">
            <p>{error}</p>
          </div>
        )}

        {clearError && (
          <div className="activity-error" role="alert">
            <p>{clearError}</p>
          </div>
        )}

        {!error && !loading && events.length === 0 && (
          <div className="activity-empty">
            <p>No activity yet.</p>
            <p className="activity-empty-hint">
              Start a rule or send traffic through a forwarded port to see events here.
            </p>
          </div>
        )}

        {!error && events.length > 0 && (
          <ol className="activity-event-list" aria-label="Activity events">
            {events.map((event) => (
              <li key={event.id} className={`activity-event activity-event--${event.severity}`}>
                <span className={`activity-severity-badge activity-severity-badge--${event.severity}`} aria-label={severityLabel(event.severity)}>
                  {severityLabel(event.severity)}
                </span>
                <span className="activity-event-time" title={event.timestamp}>
                  <span className="activity-event-date">{formatDate(event.timestamp)}</span>
                  {" "}
                  {formatTimestamp(event.timestamp)}
                </span>
                <span className="activity-event-type">{typeLabel(event.type)}</span>
                <span className="activity-event-message">{event.message}</span>
                {(event.ruleName ?? event.protocol) && (
                  <span className="activity-event-meta">
                    {event.protocol && (
                      <span className={`activity-proto-badge activity-proto-badge--${event.protocol}`}>
                        {event.protocol.toUpperCase()}
                      </span>
                    )}
                    {event.ruleName && (
                      <span className="activity-rule-name">{event.ruleName}</span>
                    )}
                  </span>
                )}
              </li>
            ))}
          </ol>
        )}

        <p className="activity-throttle-note">
          High-frequency packet events may be summarized or throttled; counters remain exact in rule status.
        </p>
      </div>

      <div className="rule-list-footer">
        <span>{loading ? "Loading…" : `${events.length} event${events.length !== 1 ? "s" : ""} shown`}</span>
        <div className="rule-list-footer-right">
          <button
            type="button"
            className="btn-text"
            onClick={handleExport}
            disabled={events.length === 0}
            title="Export visible events as JSON"
            aria-label="Export activity as JSON"
          >
            Export JSON
          </button>
          <button
            type="button"
            className="btn-text btn-text--danger"
            onClick={() => { void handleClearActivity(); }}
            title="Clear all activity events"
            aria-label="Clear activity log"
          >
            Clear Log
          </button>
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
          <button type="button" className="refresh-btn" onClick={handleRefresh} aria-label="Refresh">
            ↻
          </button>
        </div>
      </div>
    </div>
  );
}
