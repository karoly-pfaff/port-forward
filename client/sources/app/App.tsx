import { useEffect, useRef, useState, type ReactElement } from "react";
import type { ActivityEvent, ForwardRule, ForwardRuleInput, ForwardRuleResponse, ForwardStatus } from "@portier/shared";
import {
  deleteForwardRule,
  diagnoseForwardRule,
  fetchActivity,
  fetchForwardRules,
  fetchForwardStatus,
  reorderForwardRules,
  saveForwardRule,
  setForwardRuleRunning,
  setGroupRunning,
} from "../api/portierApi.js";
import type { DiagnosisEntry } from "../features/forwards/ForwardRuleList.js";
import { ForwardRuleForm } from "../features/forwards/ForwardRuleForm.js";
import { ForwardRuleList } from "../features/forwards/ForwardRuleList.js";
import { ActivityLogView } from "../features/activity/ActivityLogView.js";
import { DashboardView } from "../features/dashboard/DashboardView.js";
import { SettingsView } from "../features/settings/SettingsView.js";
import { ApiDocsView } from "../features/apidocs/ApiDocsView.js";
import { LiveConnectionsView } from "../features/connections/LiveConnectionsView.js";
import { RuleSummaryCards } from "../components/RuleSummaryCards.js";
import { useRuntimeInfo } from "../features/settings/useRuntimeInfo.js";
import { Sidebar } from "./Sidebar.js";
import { TopHeader } from "./TopHeader.js";
import { RecoveryBanner } from "./RecoveryBanner.js";
import { type AppView } from "./NavItem.js";

export { type AppView };

export function App(): ReactElement {
  const [view, setView] = useState<AppView>("rules");
  const [rules, setRules] = useState<ForwardRuleResponse[]>([]);
  const [statuses, setStatuses] = useState<ForwardStatus[]>([]);
  const [recentActivity, setRecentActivity] = useState<ActivityEvent[]>([]);
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null);
  const [duplicateSourceId, setDuplicateSourceId] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [loadingRules, setLoadingRules] = useState(true);
  const [busyRuleIds, setBusyRuleIds] = useState<Set<string>>(new Set());
  const [savingForm, setSavingForm] = useState(false);
  const [errors, setErrors] = useState<string[]>([]);
  const [serverUnavailable, setServerUnavailable] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [autoRefreshInterval, setAutoRefreshInterval] = useState(5);
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);
  const [activityRuleFilter, setActivityRuleFilter] = useState<string | null>(null);
  const [diagnosisMap, setDiagnosisMap] = useState<Map<string, DiagnosisEntry>>(new Map());

  // Runtime info is fetched once for the config-recovery banner (Slice 5). The
  // recovery state is fixed for the process lifetime, so a single fetch is enough.
  const { runtimeInfo } = useRuntimeInfo();

  const refreshInFlightRef = useRef(false);
  const handleCancelRef = useRef<() => void>(() => {});

  const statusMap = new Map(statuses.map((s) => [s.ruleId, s]));

  const editingRule = editingRuleId
    ? rules.find((r) => r.id === editingRuleId)
    : undefined;

  // The rule a duplicate is being created from (create-mode prefill, v1.8
  // Slice 8). Only meaningful while not editing an existing rule.
  const duplicateSource = duplicateSourceId
    ? rules.find((r) => r.id === duplicateSourceId)
    : undefined;

  useEffect(() => {
    void loadInitial();
  }, []);

  useEffect(() => {
    if (!autoRefresh) return;
    const id = setInterval(() => {
      if (refreshInFlightRef.current) return;
      refreshInFlightRef.current = true;
      void Promise.all([fetchForwardRules(), fetchForwardStatus(), fetchActivity({ limit: 5 })])
        .then(([nextRules, nextStatuses, activity]) => {
          setRules(nextRules);
          setStatuses(nextStatuses);
          setRecentActivity(activity);
        })
        .catch(() => {})
        .finally(() => {
          refreshInFlightRef.current = false;
        });
    }, autoRefreshInterval * 1000);
    return () => clearInterval(id);
  }, [autoRefresh, autoRefreshInterval]);

  useEffect(() => {
    if (!showForm) return;
    function onKeyDown(e: KeyboardEvent): void {
      if (e.key === "Escape") handleCancelRef.current();
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [showForm]);

  useEffect(() => {
    if (!mobileSidebarOpen) return;
    function onKeyDown(e: KeyboardEvent): void {
      if (e.key === "Escape") setMobileSidebarOpen(false);
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [mobileSidebarOpen]);

  async function loadInitial(): Promise<void> {
    setLoadingRules(true);
    setErrors([]);
    setServerUnavailable(false);
    try {
      const [nextRules, nextStatuses, activity] = await Promise.all([
        fetchForwardRules(),
        fetchForwardStatus(),
        fetchActivity({ limit: 5 })
      ]);
      setRules(nextRules);
      setStatuses(nextStatuses);
      setRecentActivity(activity);
    } catch (error) {
      if (isNetworkError(error)) {
        setServerUnavailable(true);
      } else {
        setErrors([errorMessage(error)]);
      }
    } finally {
      setLoadingRules(false);
    }
  }

  async function refreshAll(): Promise<void> {
    setErrors([]);
    try {
      const [nextRules, nextStatuses, activity] = await Promise.all([
        fetchForwardRules(),
        fetchForwardStatus(),
        fetchActivity({ limit: 5 }),
      ]);
      setRules(nextRules);
      setStatuses(nextStatuses);
      setRecentActivity(activity);
    } catch (error) {
      setErrors([errorMessage(error)]);
    }
  }

  function addBusyRule(id: string): void {
    setBusyRuleIds((s) => new Set([...s, id]));
  }

  function removeBusyRule(id: string): void {
    setBusyRuleIds((s) => {
      const next = new Set(s);
      next.delete(id);
      return next;
    });
  }

  async function handleStart(rule: ForwardRule): Promise<void> {
    addBusyRule(rule.id);
    setErrors([]);
    try {
      await setForwardRuleRunning(rule, true);
      await refreshAll();
    } catch (error) {
      setErrors([errorMessage(error)]);
    } finally {
      removeBusyRule(rule.id);
    }
  }

  async function handleStop(rule: ForwardRule): Promise<void> {
    addBusyRule(rule.id);
    setErrors([]);
    try {
      await setForwardRuleRunning(rule, false);
      await refreshAll();
    } catch (error) {
      setErrors([errorMessage(error)]);
    } finally {
      removeBusyRule(rule.id);
    }
  }

  // Start/stop every rule in a group (v1.8 Slice 6). Returns the summary so the
  // rule list can show an inline result; refreshes rule/status data on success.
  // Errors propagate to the caller (the list shows them inline near the buttons).
  async function handleGroupAction(group: string, action: "start" | "stop") {
    const result = await setGroupRunning(group, action === "start");
    await refreshAll();
    return result;
  }

  async function handleDelete(rule: ForwardRule): Promise<void> {
    addBusyRule(rule.id);
    setErrors([]);
    try {
      await deleteForwardRule(rule);
      handleClearDiagnosis(rule.id);
      if (editingRuleId === rule.id) {
        setEditingRuleId(null);
        setShowForm(false);
      }
      await refreshAll();
    } catch (error) {
      setErrors([errorMessage(error)]);
    } finally {
      removeBusyRule(rule.id);
    }
  }

  async function handleSaveRule(
    id: string | undefined,
    payload: ForwardRuleInput
  ): Promise<void> {
    setSavingForm(true);
    try {
      await saveForwardRule(id, payload);
    } catch (error) {
      setSavingForm(false);
      throw error;
    }
    setSavingForm(false);
    setEditingRuleId(null);
    setDuplicateSourceId(null);
    setShowForm(false);
    void refreshAll();
  }

  async function handleReorder(ids: string[]): Promise<void> {
    try {
      const updated = await reorderForwardRules(ids);
      setRules(updated);
    } catch (error) {
      setErrors([errorMessage(error)]);
    }
  }

  async function handleDiagnose(ruleId: string): Promise<void> {
    setDiagnosisMap((prev) => new Map([...prev, [ruleId, { state: "pending" }]]));
    try {
      const result = await diagnoseForwardRule(ruleId);
      setDiagnosisMap((prev) => new Map([...prev, [ruleId, { state: "done", result }]]));
    } catch (error) {
      setDiagnosisMap((prev) =>
        new Map([...prev, [ruleId, { state: "error", message: errorMessage(error) }]])
      );
    }
  }

  function handleClearDiagnosis(ruleId: string): void {
    setDiagnosisMap((prev) => {
      const next = new Map(prev);
      next.delete(ruleId);
      return next;
    });
  }

  function handleEditRule(rule: ForwardRule): void {
    setEditingRuleId(rule.id);
    setDuplicateSourceId(null);
    setShowForm(true);
  }

  // Open the create form pre-filled from an existing rule (v1.8 Slice 8). The
  // source rule is not edited; saving creates a new rule.
  function handleDuplicateRule(rule: ForwardRule): void {
    setEditingRuleId(null);
    setDuplicateSourceId(rule.id);
    setShowForm(true);
    setView("rules");
  }

  function handleAddRule(): void {
    setEditingRuleId(null);
    setDuplicateSourceId(null);
    setShowForm(true);
    setView("rules");
  }

  function handleCancel(): void {
    setEditingRuleId(null);
    setDuplicateSourceId(null);
    setShowForm(false);
  }
  handleCancelRef.current = handleCancel;

  function handleToggleAutoRefresh(): void {
    setAutoRefresh((v) => !v);
  }

  function handleChangeAutoRefreshInterval(seconds: number): void {
    setAutoRefreshInterval(seconds);
  }

  function handleNavClick(next: AppView): void {
    setView(next);
    setMobileSidebarOpen(false);
    if (next === "activity") setActivityRuleFilter(null);
    if (next !== "rules") {
      setEditingRuleId(null);
      setDuplicateSourceId(null);
      setShowForm(false);
    }
  }

  function handleGoToActivity(ruleId: string): void {
    setActivityRuleFilter(ruleId);
    setView("activity");
    setMobileSidebarOpen(false);
  }

  function handleRulesUpdatedFromSettings(updatedRules: ForwardRuleResponse[]): void {
    setRules(updatedRules);
    void fetchForwardStatus().then(setStatuses).catch(() => {});
  }

  return (
    <div className="app-shell">
      {mobileSidebarOpen && (
        <div
          className="sidebar-backdrop"
          aria-hidden="true"
          onClick={() => setMobileSidebarOpen(false)}
        />
      )}

      <TopHeader
        onMenuOpen={() => setMobileSidebarOpen(true)}
        onNavClick={handleNavClick}
      />

      <div className="app-body">
        <Sidebar
          open={mobileSidebarOpen}
          currentView={view}
          onNavClick={handleNavClick}
        />

        <main className="main-content">
          <RecoveryBanner recovery={runtimeInfo?.recovery} />

          {serverUnavailable && (
            <section className="server-unavailable" role="alert">
              <p>
                <strong>Server unavailable.</strong> Make sure the Portier service
                is running on 127.0.0.1:47831.
              </p>
            </section>
          )}

          {errors.length > 0 && (
            <section className="errors" role="alert" aria-live="polite">
              {errors.map((error) => (
                <p key={error}>{error}</p>
              ))}
            </section>
          )}

          {view === "dashboard" && (
            <DashboardView
              rules={rules}
              statusMap={statusMap}
              recentActivity={recentActivity}
              onGoToActivity={() => handleNavClick("activity")}
              onGoToRules={() => handleNavClick("rules")}
            />
          )}

          {view === "rules" && (
            <>
              <RuleSummaryCards
                rules={rules}
                statusMap={statusMap}
                totalDesc="All configured rules"
              />
              <ForwardRuleList
                rules={rules}
                statusMap={statusMap}
                busyRuleIds={busyRuleIds}
                loading={loadingRules}
                editingRuleId={editingRuleId}
                diagnosisMap={diagnosisMap}
                onEdit={handleEditRule}
                onDuplicate={handleDuplicateRule}
                onStart={handleStart}
                onStop={handleStop}
                onDelete={handleDelete}
                onDiagnose={handleDiagnose}
                onClearDiagnosis={handleClearDiagnosis}
                onReorder={handleReorder}
                onGoToActivity={handleGoToActivity}
                onGroupAction={handleGroupAction}
                onAddRule={handleAddRule}
                onRefresh={refreshAll}
                autoRefresh={autoRefresh}
                autoRefreshInterval={autoRefreshInterval}
                onToggleAutoRefresh={handleToggleAutoRefresh}
                onChangeAutoRefreshInterval={handleChangeAutoRefreshInterval}
              />
            </>
          )}

          {view === "activity" && (
            <>
              <RuleSummaryCards rules={rules} statusMap={statusMap} />
              <ActivityLogView
                rules={rules}
                ruleId={activityRuleFilter ?? undefined}
                onClearRuleFilter={() => setActivityRuleFilter(null)}
              />
            </>
          )}

          {view === "settings" && (
            <SettingsView
              onRulesUpdated={handleRulesUpdatedFromSettings}
              diagnosisMap={diagnosisMap}
            />
          )}

          {view === "connections" && <LiveConnectionsView />}

          {view === "api-docs" && <ApiDocsView />}
        </main>

        {showForm && (
          <aside
            className="drawer"
            aria-label={
              editingRule
                ? "Edit Forward Rule"
                : duplicateSource
                  ? "Duplicate Forward Rule"
                  : "Add Forward Rule"
            }
          >
            <ForwardRuleForm
              key={editingRuleId ?? (duplicateSourceId ? `dup-${duplicateSourceId}` : "new")}
              editingRule={editingRule}
              duplicateSource={duplicateSource}
              rules={rules}
              onSave={handleSaveRule}
              onCancel={handleCancel}
              onDelete={handleDelete}
              saving={savingForm}
            />
          </aside>
        )}
      </div>
    </div>
  );
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Unexpected error.";
}

function isNetworkError(error: unknown): boolean {
  return (
    error instanceof TypeError &&
    (error.message.toLowerCase().includes("fetch") ||
      error.message.toLowerCase().includes("network"))
  );
}
