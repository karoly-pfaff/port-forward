import type { ReactElement } from "react";
import type { DiagnosticStatus, RuleDiagnosticsResult } from "@portier/shared";

interface RuleDiagnosticsPanelProps {
  loading: boolean;
  result?: RuleDiagnosticsResult;
  error?: string;
  onClear: () => void;
}

const CHECK_ICON: Record<DiagnosticStatus, string> = {
  pass: "✓",
  warn: "⚠",
  fail: "✕",
  skip: "—",
};

const SUMMARY_ICON: Record<"pass" | "warn" | "fail", string> = {
  pass: "✓",
  warn: "⚠",
  fail: "✕",
};

export function RuleDiagnosticsPanel({
  loading,
  result,
  error,
  onClear,
}: RuleDiagnosticsPanelProps): ReactElement {
  return (
    <div className="diag-panel">
      <div className="diag-panel-header">
        <span className="diag-panel-title">Diagnostics</span>
        <button
          type="button"
          className="btn-icon diag-panel-close"
          onClick={onClear}
          aria-label="Close diagnostics"
          title="Close"
        >
          ✕
        </button>
      </div>

      {loading && (
        <div className="diag-panel-loading" aria-live="polite">
          Running diagnostics…
        </div>
      )}

      {!loading && error && (
        <div className="diag-panel-error" role="alert">
          {error}
        </div>
      )}

      {!loading && result && (
        <div className="diag-panel-body">
          <div className={`diag-summary diag-summary--${result.summary.status}`}>
            <span className="diag-summary-icon" aria-hidden="true">
              {SUMMARY_ICON[result.summary.status]}
            </span>
            <span className="diag-summary-message">{result.summary.message}</span>
            <span className="diag-summary-time" title={result.diagnosedAt}>
              {new Date(result.diagnosedAt).toLocaleTimeString()}
            </span>
          </div>
          <div className="diag-checks">
            {result.checks.map((check) => (
              <div key={check.id} className={`diag-check diag-check--${check.status}`}>
                <span className="diag-check-icon" aria-hidden="true">
                  {CHECK_ICON[check.status]}
                </span>
                <span className="diag-check-label">{check.label}</span>
                <span className="diag-check-message">{check.message}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
