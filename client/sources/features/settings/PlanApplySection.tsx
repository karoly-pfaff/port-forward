import { useRef, useState, type ReactElement, type ChangeEvent } from "react";
import type {
  ConfigPlanResponse,
  ConfigPlanRuleSnapshot,
  ExportedConfig,
  ForwardRuleResponse
} from "@portier/shared";
import { applyConfig, fetchForwardRules, planConfig } from "../../api/portierApi.js";
import {
  changeImpact,
  describeOperationImpact,
  formatChangeValue,
  formatFieldLabel,
  formatOperationType,
  isMetadataOnlyUpdate
} from "./planHelpers.js";
import { SettingsSection } from "./SettingsSection.js";

// PlanApplySection renders the "Plan & Apply Config" panel: pick a config file,
// preview the plan against the running config, then apply with explicit
// destructive confirmation. It owns all plan/apply state; on a successful apply
// it refetches rules and notifies the parent via onRulesUpdated.
export function PlanApplySection({
  onRulesUpdated
}: {
  onRulesUpdated: (rules: ForwardRuleResponse[]) => void;
}): ReactElement {
  const planFileInputRef = useRef<HTMLInputElement>(null);
  const [planSelectedFileName, setPlanSelectedFileName] = useState<string | null>(null);
  const [planParsedConfig, setPlanParsedConfig] = useState<ExportedConfig | null>(null);
  const [planParseError, setPlanParseError] = useState<string | null>(null);
  const [planPreviewing, setPlanPreviewing] = useState(false);
  const [planResult, setPlanResult] = useState<ConfigPlanResponse | null>(null);
  const [planPreviewError, setPlanPreviewError] = useState<string | null>(null);
  const [destructiveConfirmed, setDestructiveConfirmed] = useState(false);
  const [planApplying, setPlanApplying] = useState(false);
  const [planApplyResult, setPlanApplyResult] = useState<string | null>(null);
  const [planApplyError, setPlanApplyError] = useState<string | null>(null);

  function handlePlanFileChange(e: ChangeEvent<HTMLInputElement>): void {
    setPlanParsedConfig(null);
    setPlanParseError(null);
    setPlanResult(null);
    setPlanPreviewError(null);
    setPlanApplyResult(null);
    setPlanApplyError(null);
    setDestructiveConfirmed(false);

    const file = e.target.files?.[0];
    setPlanSelectedFileName(file?.name ?? null);
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (ev) => {
      try {
        const text = ev.target?.result;
        if (typeof text !== "string") throw new Error("Could not read file.");
        const parsed = JSON.parse(text) as ExportedConfig;
        if (parsed.version !== "1" || !Array.isArray(parsed.rules)) {
          throw new Error("Not a valid Portier config file (expected version 1 with a rules array).");
        }
        setPlanParsedConfig(parsed);
      } catch (error) {
        /* v8 ignore next -- JSON.parse only throws SyntaxError (an Error); the non-Error fallback is defensive */
        setPlanParseError(error instanceof Error ? error.message : "Parse error.");
      }
    };
    reader.readAsText(file);
  }

  async function handlePlanPreview(): Promise<void> {
    /* v8 ignore next -- the Preview button only renders once planParsedConfig is set; this guard is unreachable from the UI */
    if (!planParsedConfig) return;
    setPlanPreviewing(true);
    setPlanResult(null);
    setPlanPreviewError(null);
    setPlanApplyResult(null);
    setPlanApplyError(null);
    setDestructiveConfirmed(false);
    try {
      const desired = { rules: planParsedConfig.rules as unknown as ConfigPlanRuleSnapshot[] };
      const result = await planConfig(desired);
      setPlanResult(result);
    } catch (error) {
      setPlanPreviewError(error instanceof Error ? error.message : "Preview failed.");
    } finally {
      setPlanPreviewing(false);
    }
  }

  async function handlePlanApply(): Promise<void> {
    /* v8 ignore next -- the Apply button only renders once a plan has been previewed (planParsedConfig + planResult set); this guard is unreachable from the UI */
    if (!planParsedConfig || !planResult) return;
    setPlanApplying(true);
    setPlanApplyResult(null);
    setPlanApplyError(null);
    try {
      const desired = { rules: planParsedConfig.rules as unknown as ConfigPlanRuleSnapshot[] };
      const response = await applyConfig({ desired, yes: true });
      if (!response.ok) {
        setPlanApplyError("Apply failed: plan has errors. Re-preview the config to see current errors.");
        return;
      }
      const { add, update, remove } = response.applied;
      setPlanApplyResult(`Config applied. ${add} added, ${update} updated, ${remove} removed.`);
      const updatedRules = await fetchForwardRules();
      onRulesUpdated(updatedRules);
      setPlanParsedConfig(null);
      setPlanResult(null);
      setPlanSelectedFileName(null);
      setDestructiveConfirmed(false);
      if (planFileInputRef.current) planFileInputRef.current.value = "";
    } catch (error) {
      setPlanApplyError(error instanceof Error ? error.message : "Apply failed.");
    } finally {
      setPlanApplying(false);
    }
  }

  return (
    <SettingsSection title="Plan & Apply Config">
      <p className="settings-desc">
        Preview changes before applying. Select a config file to compare it with the
        running configuration, then apply safely with explicit confirmation.
      </p>

      <div className="settings-import-controls">
        <div className="settings-file-picker">
          <input
            ref={planFileInputRef}
            id="plan-file-input"
            type="file"
            accept=".json,application/json"
            onChange={handlePlanFileChange}
            aria-label="Select config file for plan"
            className="settings-file-input-hidden"
          />
          <label htmlFor="plan-file-input" className="settings-file-btn">
            Choose file
          </label>
          <span className="settings-file-name">
            {planSelectedFileName ?? "No file chosen"}
          </span>
        </div>

        {planParseError && (
          <div className="settings-error" role="alert">{planParseError}</div>
        )}

        {planParsedConfig && !planResult && (
          <button
            type="button"
            onClick={() => void handlePlanPreview()}
            disabled={planPreviewing}
          >
            {planPreviewing ? "Previewing…" : "Preview changes"}
          </button>
        )}

        {planPreviewError && (
          <div className="settings-error" role="alert">{planPreviewError}</div>
        )}

        {planResult && (
          <div className="settings-plan-preview">
            <div className="settings-preview-title">Plan preview</div>

            <div className="settings-plan-summary">
              <span>Add: {planResult.summary.add}</span>
              {" · "}
              <span>Update: {planResult.summary.update}</span>
              {" · "}
              <span>Remove: {planResult.summary.remove}</span>
              {" · "}
              <span>Unchanged: {planResult.summary.unchanged}</span>
              {planResult.summary.destructive > 0 && (
                <>
                  {" · "}
                  <span className="settings-plan-summary-destructive">
                    Destructive: {planResult.summary.destructive}
                  </span>
                </>
              )}
            </div>

            {!planResult.summary.hasDrift && !planResult.summary.hasErrors && (
              <p className="settings-desc">No drift detected. This config is already in sync.</p>
            )}

            {planResult.errors.length > 0 && (
              <div className="settings-error" role="alert">
                <div>Apply is disabled until plan errors are resolved:</div>
                {planResult.errors.map((e, i) => (
                  <div key={i}>[{e.code}] {e.message}</div>
                ))}
              </div>
            )}

            {planResult.warnings.length > 0 && (
              <div className="settings-warn" role="status">
                {planResult.warnings.map((w, i) => (
                  <div key={i}>[{w.code}] {w.message}</div>
                ))}
              </div>
            )}

            {planResult.operations.length > 0 && (
              <ul className="settings-plan-ops">
                {planResult.operations.map((op, i) => (
                  <li key={i} className={`settings-plan-op settings-plan-op--${op.type}`}>
                    <div className="settings-plan-op-head">
                      <span className={`plan-op-badge plan-op-badge--${op.type}`}>
                        {formatOperationType(op.type)}
                      </span>
                      <span className="settings-plan-op-name">
                        {op.ruleName}{" "}
                        <span className="settings-plan-op-proto">({op.protocol.toUpperCase()})</span>
                      </span>
                      {op.destructive && (
                        <span className="plan-op-tag plan-op-tag--destructive">Destructive</span>
                      )}
                      {isMetadataOnlyUpdate(op) && (
                        <span className="plan-op-tag plan-op-tag--metadata">Metadata only</span>
                      )}
                    </div>
                    <div className="settings-plan-op-impact">{describeOperationImpact(op)}</div>
                    {op.changes && op.changes.length > 0 && (
                      <ul className="settings-plan-changes">
                        {op.changes.map((c, j) => (
                          <li key={j} className="settings-plan-change">
                            <span className="settings-plan-change-field">
                              {formatFieldLabel(c.field)}
                            </span>
                            <span
                              className={`plan-change-impact plan-change-impact--${changeImpact(c.field)}`}
                            >
                              {changeImpact(c.field)}
                            </span>
                            <span className="settings-plan-change-values">
                              {formatChangeValue(c.before)} → {formatChangeValue(c.after)}
                            </span>
                          </li>
                        ))}
                      </ul>
                    )}
                  </li>
                ))}
              </ul>
            )}

            {planResult.summary.destructive > 0 && !planResult.summary.hasErrors && (
              <label className="settings-mode-option">
                <input
                  type="checkbox"
                  checked={destructiveConfirmed}
                  onChange={(e) => setDestructiveConfirmed(e.target.checked)}
                  aria-label="Confirm destructive changes"
                />
                {" "}I understand this will remove or change existing rules.
              </label>
            )}

            {!planResult.summary.hasErrors && (
              <button
                type="button"
                className={planResult.summary.destructive > 0 ? "danger" : "primary"}
                disabled={
                  planApplying ||
                  !planResult.summary.hasDrift ||
                  (planResult.summary.destructive > 0 && !destructiveConfirmed)
                }
                onClick={() => void handlePlanApply()}
              >
                {planApplying ? "Applying…" :
                 !planResult.summary.hasDrift ? "No changes to apply" :
                 "Apply changes"}
              </button>
            )}

            {planApplyError && (
              <div className="settings-error" role="alert">{planApplyError}</div>
            )}
          </div>
        )}

        {planApplyResult && (
          <div className="settings-success" role="status">{planApplyResult}</div>
        )}
      </div>
    </SettingsSection>
  );
}
