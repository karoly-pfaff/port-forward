import { type ReactElement } from "react";
import type { ActivityEvent, ForwardRuleResponse, ForwardStatus } from "@portier/shared";
import { RuleSummaryCards } from "../../components/RuleSummaryCards.js";
import { formatBytes, formatTimestamp } from "../../utils/format.js";

interface DashboardViewProps {
  rules: ForwardRuleResponse[];
  statusMap: Map<string, ForwardStatus>;
  recentActivity: ActivityEvent[];
  onGoToActivity: () => void;
  onGoToRules: () => void;
}

const SEVERITY_SYMBOL: Record<string, string> = {
  info: "·",
  success: "✓",
  warning: "⚠",
  error: "✕"
};

export function DashboardView({
  rules,
  statusMap,
  recentActivity,
  onGoToActivity,
  onGoToRules,
}: DashboardViewProps): ReactElement {
  const topActive = [...rules]
    .map((r) => {
      const s = statusMap.get(r.id);
      return { rule: r, traffic: (s?.bytesIn ?? 0) + (s?.bytesOut ?? 0) };
    })
    .filter((x) => x.traffic > 0)
    .sort((a, b) => b.traffic - a.traffic)
    .slice(0, 5);

  return (
    <div className="dashboard-view">
      <RuleSummaryCards rules={rules} statusMap={statusMap} />

      <div className="dashboard-columns">
        {/* Top active rules */}
        <div className="dashboard-panel">
          <div className="dashboard-panel-header">
            <span className="dashboard-panel-title">Top Rules by Traffic</span>
            <button type="button" onClick={onGoToRules}>View All</button>
          </div>
          {topActive.length === 0 ? (
            <p className="dashboard-empty">No traffic recorded yet. Start a rule to see stats here.</p>
          ) : (
            <ol className="dashboard-top-rules">
              {topActive.map(({ rule, traffic }) => (
                <li key={rule.id} className="dashboard-top-rule-row">
                  <span className={`protocol-badge protocol-badge--${rule.protocol}`}>
                    {rule.protocol.toUpperCase()}
                  </span>
                  <span className="dashboard-rule-name">{rule.name}</span>
                  <span className="dashboard-rule-traffic">{formatBytes(traffic)}</span>
                </li>
              ))}
            </ol>
          )}
        </div>

        {/* Recent activity */}
        <div className="dashboard-panel">
          <div className="dashboard-panel-header">
            <span className="dashboard-panel-title">Recent Activity</span>
            <button type="button" onClick={onGoToActivity}>View All</button>
          </div>
          {recentActivity.length === 0 ? (
            <p className="dashboard-empty">No activity yet.</p>
          ) : (
            <ol className="dashboard-activity-list">
              {recentActivity.map((ev) => (
                <li key={ev.id} className={`dashboard-activity-row dashboard-activity-row--${ev.severity}`}>
                  <span className="dashboard-activity-sym" aria-hidden="true">
                    {/* v8 ignore next -- ev.severity is a strict ActivitySeverity union fully mapped by SEVERITY_SYMBOL; the ?? fallback is unreachable */}
                    {SEVERITY_SYMBOL[ev.severity] ?? "·"}
                  </span>
                  <span className="dashboard-activity-time">{formatTimestamp(ev.timestamp)}</span>
                  <span className="dashboard-activity-msg">{ev.message}</span>
                </li>
              ))}
            </ol>
          )}
        </div>
      </div>
    </div>
  );
}
