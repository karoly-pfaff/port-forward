import { useEffect, useRef, useState, type ReactElement } from "react";
import type { ActivityEvent, ActivitySeverity, ForwardRuleResponse } from "@portier/shared";
import { fetchActivity } from "../../api/portierApi.js";
import { formatTimestamp } from "../../utils/format.js";

const SEVERITY_OPTIONS: { value: "" | ActivitySeverity; label: string }[] = [
  { value: "", label: "All severities" },
  { value: "info", label: "Info" },
  { value: "success", label: "Success" },
  { value: "warning", label: "Warning" },
  { value: "error", label: "Error" }
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

interface ActivityLogViewProps {
  rules?: ForwardRuleResponse[];
  ruleId?: string;
  onClearRuleFilter?: () => void;
}

export function ActivityLogView({ rules, ruleId, onClearRuleFilter }: ActivityLogViewProps): ReactElement {
  const [events, setEvents] = useState<ActivityEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [limit, setLimit] = useState(100);
  const [severity, setSeverity] = useState<"" | ActivitySeverity>("");
  const [ruleIdFilter, setRuleIdFilter] = useState(ruleId ?? "");
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [autoRefreshInterval, setAutoRefreshInterval] = useState(5);
  const fetchInFlightRef = useRef(false);

  useEffect(() => {
    setRuleIdFilter(ruleId ?? "");
  }, [ruleId]);

  async function load(opts: { limit: number; severity: "" | ActivitySeverity; ruleId: string }): Promise<void> {
    if (fetchInFlightRef.current) return;
    fetchInFlightRef.current = true;
    setError(null);
    try {
      const result = await fetchActivity({
        limit: opts.limit,
        severity: opts.severity || undefined,
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
    void load({ limit, severity, ruleId: ruleIdFilter });
  }, [limit, severity, ruleIdFilter]);

  useEffect(() => {
    if (!autoRefresh) return;
    const id = setInterval(() => {
      void load({ limit, severity, ruleId: ruleIdFilter });
    }, autoRefreshInterval * 1000);
    return () => clearInterval(id);
  }, [autoRefresh, autoRefreshInterval, limit, severity, ruleIdFilter]);

  function handleRefresh(): void {
    void load({ limit, severity, ruleId: ruleIdFilter });
  }

  function handleRuleChange(e: React.ChangeEvent<HTMLSelectElement>): void {
    const val = e.target.value;
    setRuleIdFilter(val);
    if (!val) onClearRuleFilter?.();
  }

  return (
    <div className="rule-list-section">
      <div className="rule-list-header">
        <div className="rule-list-title-group">
          <div className="rule-list-title">Activity</div>
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
              <option value="">All rules</option>
              {rules.map((r) => (
                <option key={r.id} value={r.id}>{r.name}</option>
              ))}
            </select>
          )}
          <select
            className="filter-select"
            value={severity}
            onChange={(e) => setSeverity(e.target.value as "" | ActivitySeverity)}
            aria-label="Filter by severity"
          >
            {SEVERITY_OPTIONS.map((opt) => (
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
        </div>
      </div>

      <div className="rule-list-body">
        {error && (
          <div className="activity-error" role="alert">
            <p>{error}</p>
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
      </div>

      <div className="rule-list-footer">
        <span>{loading ? "Loading…" : `${events.length} event${events.length !== 1 ? "s" : ""} shown`}</span>
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
          <button type="button" className="refresh-btn" onClick={handleRefresh} aria-label="Refresh">
            ↻
          </button>
        </div>
      </div>
    </div>
  );
}
