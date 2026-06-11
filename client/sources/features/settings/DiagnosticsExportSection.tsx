import { useState, type ReactElement } from "react";
import type { DiagnosisEntry } from "../forwards/ForwardRuleList.js";
import { buildDiagnosticsBundle, buildDiagnosticsFilename, downloadJson } from "./diagnosticsExport.js";
import { SettingsSection } from "./SettingsSection.js";

// DiagnosticsExportSection renders the "Diagnostics Export" panel and owns its
// own export state. A bundle with a non-empty errors array surfaces a partial
// warning; a full success shows a transient confirmation.
export function DiagnosticsExportSection({
  diagnosisMap
}: {
  diagnosisMap?: ReadonlyMap<string, DiagnosisEntry>;
}): ReactElement {
  const [diagExporting, setDiagExporting] = useState(false);
  const [diagExportSuccess, setDiagExportSuccess] = useState(false);
  const [diagExportPartial, setDiagExportPartial] = useState(false);
  const [diagExportError, setDiagExportError] = useState<string | null>(null);

  async function handleDiagnosticsExport(): Promise<void> {
    setDiagExporting(true);
    setDiagExportSuccess(false);
    setDiagExportPartial(false);
    setDiagExportError(null);
    try {
      const bundle = await buildDiagnosticsBundle(diagnosisMap ?? new Map());
      downloadJson(buildDiagnosticsFilename(), bundle);
      if (bundle.errors && bundle.errors.length > 0) {
        setDiagExportPartial(true);
      } else {
        setDiagExportSuccess(true);
        setTimeout(() => setDiagExportSuccess(false), 3000);
      }
    } catch (error) {
      setDiagExportError(error instanceof Error ? error.message : "Export failed.");
    } finally {
      setDiagExporting(false);
    }
  }

  return (
    <SettingsSection title="Diagnostics Export">
      <p className="settings-desc">
        Downloads a local JSON bundle with runtime info, rules, statuses, recent activity,
        and any diagnostics run in this session. Does not include logs, environment variables,
        or raw local files. Nothing is uploaded.
      </p>
      <div>
        <button type="button" onClick={() => void handleDiagnosticsExport()} disabled={diagExporting}>
          {diagExporting ? "Generating…" : "Download Diagnostics"}
        </button>
      </div>
      {diagExportSuccess && (
        <div className="settings-success" role="status">Diagnostics exported successfully.</div>
      )}
      {diagExportPartial && (
        <div className="settings-warn" role="status">
          Exported with partial data — some sources could not be reached. See the errors field in the downloaded file.
        </div>
      )}
      {diagExportError && (
        <div className="settings-error" role="alert">{diagExportError}</div>
      )}
    </SettingsSection>
  );
}
