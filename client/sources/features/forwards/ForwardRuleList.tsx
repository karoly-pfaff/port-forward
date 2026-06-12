import { useMemo, useState, useRef, Fragment, type ReactElement } from "react";
import { Activity, Pencil, Stethoscope } from "lucide-react";
import type { ForwardRule, ForwardRuleResponse, ForwardStatus, GroupActionResponse, RuleDiagnosticsResult } from "@portier/shared";
import { AdvisoryList } from "../../components/AdvisoryList.js";
import { ForwardStatusBadge } from "./ForwardStatusBadge.js";
import { RuleHealthBadge } from "./RuleHealthBadge.js";
import { RuleDiagnosticsPanel } from "./RuleDiagnosticsPanel.js";
import { formatBytes, formatUdpModeLabel } from "../../utils/format.js";

export type DiagnosisEntry =
  | { state: "pending" }
  | { state: "done"; result: RuleDiagnosticsResult }
  | { state: "error"; message: string };

// Sentinel values for the group filter <select>. Real group names cannot be
// empty (trimmed/normalized away) and these wrapped tokens cannot collide with
// a user-entered group, so they are safe non-group selections.
const ALL_GROUPS = "__all__";
const UNGROUPED = "__ungrouped__";

interface ForwardRuleListProps {
  rules: ForwardRuleResponse[];
  statusMap: Map<string, ForwardStatus>;
  busyRuleIds: Set<string>;
  loading: boolean;
  editingRuleId: string | null;
  diagnosisMap: Map<string, DiagnosisEntry>;
  onEdit: (rule: ForwardRule) => void;
  onStart: (rule: ForwardRule) => void;
  onStop: (rule: ForwardRule) => void;
  onDelete: (rule: ForwardRule) => void;
  onDiagnose: (ruleId: string) => void;
  onClearDiagnosis: (ruleId: string) => void;
  onReorder?: (ids: string[]) => void;
  onGoToActivity?: (ruleId: string) => void;
  onGroupAction?: (group: string, action: "start" | "stop") => Promise<GroupActionResponse>;
  onAddRule: () => void;
  onRefresh: () => void;
  autoRefresh: boolean;
  autoRefreshInterval: number;
  onToggleAutoRefresh: () => void;
  onChangeAutoRefreshInterval: (seconds: number) => void;
}

export function ForwardRuleList({
  rules,
  statusMap,
  busyRuleIds,
  loading,
  editingRuleId,
  diagnosisMap,
  onEdit,
  onStart,
  onStop,
  onDelete,
  onDiagnose,
  onClearDiagnosis,
  onReorder,
  onGoToActivity,
  onGroupAction,
  onAddRule,
  onRefresh,
  autoRefresh,
  autoRefreshInterval,
  onToggleAutoRefresh,
  onChangeAutoRefreshInterval,
}: ForwardRuleListProps): ReactElement {
  const [confirmingDeleteId, setConfirmingDeleteId] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "running" | "stopped" | "error">("all");
  const [groupFilter, setGroupFilter] = useState<string>(ALL_GROUPS);
  const [groupActionBusy, setGroupActionBusy] = useState<"" | "start" | "stop">("");
  const [groupActionResult, setGroupActionResult] =
    useState<{ tone: "info" | "warn" | "error"; text: string } | null>(null);
  const [dragOverId, setDragOverId] = useState<string | null>(null);
  const dragIdRef = useRef<string | null>(null);

  // Distinct, normalized group names present in the current rules, sorted
  // locale-insensitive alphabetical. Derived from rules only — never persisted.
  const groupOptions = useMemo(() => {
    const seen = new Set<string>();
    for (const rule of rules) {
      const g = rule.group?.trim();
      if (g) seen.add(g);
    }
    return [...seen].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" }));
  }, [rules]);
  const hasUngrouped = useMemo(() => rules.some((r) => !r.group?.trim()), [rules]);

  // Clamp a stale selection (e.g. the last rule in a group was deleted) back to
  // "all groups" so the control never shows a value with no matching option.
  const activeGroupFilter =
    groupFilter === ALL_GROUPS ||
    groupFilter === UNGROUPED ||
    groupOptions.includes(groupFilter)
      ? groupFilter
      : ALL_GROUPS;

  const groupFilterActive = activeGroupFilter !== ALL_GROUPS;
  const canReorder =
    !!onReorder && !searchQuery.trim() && statusFilter === "all" && !groupFilterActive;

  const runningCount = rules.filter((r) => statusMap.get(r.id)?.running).length;
  const errorCount = rules.filter((r) => !!statusMap.get(r.id)?.lastError).length;

  const filteredRules = rules.filter((rule) => {
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      const matches =
        rule.name.toLowerCase().includes(q) ||
        rule.listenHost.includes(q) ||
        String(rule.listenPort).includes(q) ||
        rule.targetHost.includes(q) ||
        String(rule.targetPort).includes(q) ||
        rule.protocol.includes(q) ||
        (rule.group?.toLowerCase().includes(q) ?? false);
      if (!matches) return false;
    }
    if (statusFilter !== "all") {
      const status = statusMap.get(rule.id);
      if (statusFilter === "running" && !status?.running) return false;
      if (statusFilter === "stopped" && status?.running) return false;
      if (statusFilter === "error" && !status?.lastError) return false;
    }
    if (groupFilterActive) {
      const g = rule.group?.trim() ?? "";
      if (activeGroupFilter === UNGROUPED) {
        if (g !== "") return false;
      } else if (g !== activeGroupFilter) {
        return false;
      }
    }
    return true;
  });

  function handleConfirmDelete(rule: ForwardRule): void {
    setConfirmingDeleteId(null);
    onClearDiagnosis(rule.id);
    onDelete(rule);
  }

  // Group start/stop actions are only meaningful for a single concrete group —
  // never "All Groups" and never "Ungrouped" (no ungrouped bulk action exists).
  const groupActionInFlight = groupActionBusy !== "";
  const showGroupActions =
    !!onGroupAction && groupFilterActive && activeGroupFilter !== UNGROUPED;

  function changeGroupFilter(value: string): void {
    setGroupFilter(value);
    setGroupActionResult(null); // result is scoped to the previously selected group
  }

  async function runGroupAction(action: "start" | "stop"): Promise<void> {
    if (!onGroupAction || groupActionInFlight) return;
    if (activeGroupFilter === ALL_GROUPS || activeGroupFilter === UNGROUPED) return;
    const group = activeGroupFilter;
    setGroupActionBusy(action);
    setGroupActionResult(null);
    try {
      const res = await onGroupAction(group, action);
      const verb = res.action === "start" ? "Started" : "Stopped";
      const parts = [`${res.succeeded} succeeded`, `${res.skipped} skipped`];
      if (res.failed > 0) parts.push(`${res.failed} failed`);
      setGroupActionResult({
        tone: res.failed > 0 ? "warn" : "info",
        text: `${verb} group "${res.group}": ${parts.join(", ")} (${res.total} total)`,
      });
    } catch (error) {
      setGroupActionResult({
        tone: "error",
        text: error instanceof Error ? error.message : "Group action failed.",
      });
    } finally {
      setGroupActionBusy("");
    }
  }

  return (
    <section className="rule-list-section">
      {/* Header */}
      <div className="rule-list-header">
        <div className="rule-list-title-group">
          <div className="rule-list-title">Forward Rules</div>
          <div className="rule-list-subtitle">
            <span>{rules.length} configured</span>
            <span aria-hidden="true"> · </span>
            <span>{runningCount} running</span>
            {errorCount > 0 && (
              <>
                <span aria-hidden="true"> · </span>
                <span>{errorCount} error</span>
              </>
            )}
          </div>
        </div>
        <div className="rule-list-controls">
          <input
            className="search-input"
            type="search"
            placeholder="Search rules…"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            aria-label="Search rules"
          />
          <select
            className="filter-select"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as typeof statusFilter)}
            aria-label="Filter by status"
          >
            <option value="all">All Statuses</option>
            <option value="running">Running</option>
            <option value="stopped">Stopped</option>
            <option value="error">Error</option>
          </select>
          {groupOptions.length > 0 && (
            <select
              className="filter-select"
              value={activeGroupFilter}
              onChange={(e) => changeGroupFilter(e.target.value)}
              aria-label="Filter by group"
            >
              <option value={ALL_GROUPS}>All Groups</option>
              {groupOptions.map((group) => (
                <option key={group} value={group}>
                  {group}
                </option>
              ))}
              {hasUngrouped && <option value={UNGROUPED}>Ungrouped</option>}
            </select>
          )}
          <button type="button" className="primary rule-list-add-btn" onClick={onAddRule}>
            + Add Rule
          </button>
        </div>
      </div>

      {/* Group action toolbar — only when filtered to one concrete group */}
      {showGroupActions && (
        <div
          className="group-action-bar"
          role="group"
          aria-label={`Group actions for ${activeGroupFilter}`}
        >
          <button
            type="button"
            className="group-action-btn"
            onClick={() => void runGroupAction("start")}
            disabled={groupActionInFlight}
          >
            {groupActionBusy === "start" ? "Starting…" : `Start group "${activeGroupFilter}"`}
          </button>
          <button
            type="button"
            className="group-action-btn"
            onClick={() => void runGroupAction("stop")}
            disabled={groupActionInFlight}
          >
            {groupActionBusy === "stop" ? "Stopping…" : `Stop group "${activeGroupFilter}"`}
          </button>
          {groupActionResult && (
            <span
              className={`group-action-status group-action-status--${groupActionResult.tone}`}
              role={groupActionResult.tone === "info" ? "status" : "alert"}
            >
              {groupActionResult.text}
            </span>
          )}
        </div>
      )}

      {/* Table */}
      <div className="rule-list-body">
        {loading && rules.length === 0 ? (
          <p className="table-loading">Loading rules…</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th scope="col" className="col-drag-handle" />
                <th scope="col">Name</th>
                <th scope="col">Protocol</th>
                <th scope="col">Listen Endpoint</th>
                <th scope="col">Target Endpoint</th>
                <th scope="col">Autostart</th>
                <th scope="col">Status</th>
                <th scope="col">Traffic</th>
                <th scope="col">Actions</th>
                <th scope="col" className="delete-cell" />
              </tr>
            </thead>
            <tbody>
              {filteredRules.map((rule) => {
                const status = statusMap.get(rule.id);
                const isBusy = busyRuleIds.has(rule.id);
                const isConfirming = confirmingDeleteId === rule.id;
                const isSelected = editingRuleId === rule.id;
                const diagEntry = diagnosisMap.get(rule.id);
                const isDiagPending = diagEntry?.state === "pending";

                return (
                  <Fragment key={rule.id}>
                    <tr
                      draggable={canReorder}
                      onDragStart={() => { dragIdRef.current = rule.id; }}
                      onDragOver={(e) => { e.preventDefault(); setDragOverId(rule.id); }}
                      onDrop={() => {
                        const fromId = dragIdRef.current;
                        if (!fromId || fromId === rule.id) return;
                        const fromIdx = filteredRules.findIndex((r) => r.id === fromId);
                        const toIdx = filteredRules.findIndex((r) => r.id === rule.id);
                        const reordered = [...filteredRules];
                        const [moved] = reordered.splice(fromIdx, 1);
                        reordered.splice(toIdx, 0, moved);
                        onReorder?.(reordered.map((r) => r.id));
                        dragIdRef.current = null;
                        setDragOverId(null);
                      }}
                      onDragEnd={() => { dragIdRef.current = null; setDragOverId(null); }}
                      className={
                        dragIdRef.current === rule.id
                          ? "row-dragging"
                          : dragOverId === rule.id
                          ? "row-dragover"
                          : isSelected
                          ? "row-selected"
                          : status?.lastError
                          ? "row-error"
                          : undefined
                      }
                    >
                      <td className="col-drag-handle">
                        {canReorder && <span className="drag-handle" aria-hidden="true">⠿</span>}
                      </td>
                      <td>
                        {rule.name}
                        {rule.group && (
                          <span className="rule-group-label" title={`Group: ${rule.group}`}>
                            {rule.group}
                          </span>
                        )}
                        <AdvisoryList advisories={rule.advisories.filter((a) => a.code !== "LAN_EXPOSURE")} compact />
                      </td>
                      <td>
                        <span className={`protocol-badge protocol-badge--${rule.protocol}`}>
                          {rule.protocol.toUpperCase()}
                        </span>
                        {rule.protocol === "udp" && (
                          <span className="udp-mode-label">{formatUdpModeLabel(rule.udpMode)}</span>
                        )}
                      </td>
                      <td className="endpoint-cell">
                        <span className="endpoint-host">{rule.listenHost}</span>
                        <span className="endpoint-port">:{rule.listenPort}</span>
                      </td>
                      <td className="endpoint-cell">
                        <span className="endpoint-host">{rule.targetHost}</span>
                        <span className="endpoint-port">:{rule.targetPort}</span>
                      </td>
                      <td>
                        <span className={`autostart-cell autostart-cell--${rule.enabled ? "yes" : "no"}`}>
                          {rule.enabled ? "Yes" : "No"}
                        </span>
                      </td>
                      <td>
                        <span className="status-cell">
                          <ForwardStatusBadge
                            status={status}
                            onErrorClick={onGoToActivity ? () => onGoToActivity(rule.id) : undefined}
                          />
                          {status && <RuleHealthBadge health={status.health} />}
                        </span>
                      </td>
                      <td className="traffic-cell">
                        {rule.protocol === "tcp" && (
                          <span className="traffic-active">
                            {status?.activeConnections ?? 0} active
                          </span>
                        )}
                        {rule.protocol === "udp" && rule.udpMode === "bidirectional-multi-client" && (
                          <span className="traffic-active">
                            {status?.activeUdpSessions ?? 0} sessions
                          </span>
                        )}
                        <span className="traffic-bytes">
                          {formatBytes(status?.bytesIn ?? 0)} / {formatBytes(status?.bytesOut ?? 0)}
                        </span>
                      </td>
                      <td>
                        {isConfirming ? (
                          <div className="confirm-delete">
                            <span className="confirm-label">
                              Delete &ldquo;{rule.name}&rdquo;?
                            </span>
                            <button
                              type="button"
                              className="danger"
                              onClick={() => handleConfirmDelete(rule)}
                              disabled={isBusy}
                            >
                              Confirm
                            </button>
                            <button
                              type="button"
                              onClick={() => setConfirmingDeleteId(null)}
                            >
                              Cancel
                            </button>
                          </div>
                        ) : (
                          <div className="actions">
                            <button
                              type="button"
                              className={`btn-icon${!isBusy && !status?.running ? " btn-icon--start" : ""}${!isBusy && status?.running ? " btn-icon--stop" : ""}`}
                              aria-label={isBusy ? undefined : status?.running ? "Stop" : "Start"}
                              title={isBusy ? undefined : status?.running ? "Stop" : "Start"}
                              onClick={() =>
                                status?.running ? onStop(rule) : onStart(rule)
                              }
                              disabled={isBusy}
                            >
                              {isBusy ? "…" : status?.running ? "⏹" : "▶"}
                            </button>
                            <button
                              type="button"
                              className="btn-icon"
                              aria-label="Edit"
                              title="Edit"
                              onClick={() => onEdit(rule)}
                              disabled={isBusy}
                            >
                              <Pencil size={14} aria-hidden="true" />
                            </button>
                            <button
                              type="button"
                              className="btn-icon"
                              aria-label="Diagnose"
                              title="Diagnose"
                              onClick={() => onDiagnose(rule.id)}
                              disabled={isBusy || isDiagPending}
                            >
                              {isDiagPending ? "…" : <Stethoscope size={14} aria-hidden="true" />}
                            </button>
                            {onGoToActivity && (
                              <button
                                type="button"
                                className="btn-icon"
                                aria-label="View activity"
                                title="View activity"
                                onClick={() => onGoToActivity(rule.id)}
                              >
                                <Activity size={14} aria-hidden="true" />
                              </button>
                            )}
                          </div>
                        )}
                      </td>
                      <td className="delete-cell">
                        <button
                          type="button"
                          className="delete-cell-btn"
                          aria-label="Delete"
                          title="Delete"
                          onClick={() => setConfirmingDeleteId(rule.id)}
                          disabled={isBusy || isConfirming}
                        >
                          🗑
                        </button>
                      </td>
                    </tr>
                    {diagEntry && (
                      <tr className="diag-row">
                        <td colSpan={10} className="diag-row-cell">
                          <RuleDiagnosticsPanel
                            loading={diagEntry.state === "pending"}
                            result={diagEntry.state === "done" ? diagEntry.result : undefined}
                            error={diagEntry.state === "error" ? diagEntry.message : undefined}
                            onClear={() => onClearDiagnosis(rule.id)}
                          />
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
              {filteredRules.length === 0 && !loading && (
                <tr>
                  <td colSpan={10} className="empty">
                    {rules.length === 0
                      ? "No forwarding rules yet. Click + Add Rule to create one."
                      : "No rules match the current filter."}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        )}
      </div>

      {/* Footer */}
      <div className="rule-list-footer">
        <span>Showing {filteredRules.length} of {rules.length} rule{rules.length !== 1 ? "s" : ""}</span>
        <div className="rule-list-footer-right">
          <label className="auto-refresh-toggle" title="Auto-refresh">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={onToggleAutoRefresh}
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
            onChange={(e) => onChangeAutoRefreshInterval(Number(e.target.value))}
            disabled={!autoRefresh}
            aria-label="Auto-refresh interval"
          >
            <option value="" disabled>Off</option>
            <option value={2}>2s</option>
            <option value={5}>5s</option>
            <option value={10}>10s</option>
            <option value={30}>30s</option>
          </select>
          <button type="button" className="refresh-btn" onClick={onRefresh} aria-label="Refresh">
            ↻
          </button>
        </div>
      </div>
    </section>
  );
}
