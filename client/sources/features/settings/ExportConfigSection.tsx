import type { ReactElement } from "react";
import { SettingsSection } from "./SettingsSection.js";
import type { ConfigExport } from "./useConfigExport.js";

// ExportConfigSection renders the "Export Config" panel. The export flow is
// owned by the shared useConfigExport hook (passed in) so it stays in sync with
// the Import panel's backup button.
export function ExportConfigSection({ configExport }: { configExport: ConfigExport }): ReactElement {
  const { exporting, exportSuccess, exportError, handleExport } = configExport;
  return (
    <SettingsSection title="Export Config">
      <p className="settings-desc">
        Downloads all current rules as a portable JSON file. The Activity Log is not included.
      </p>
      <div>
        <button type="button" onClick={handleExport} disabled={exporting}>
          {exporting ? "Exporting…" : "Download Config"}
        </button>
      </div>
      {exportSuccess && (
        <div className="settings-success" role="status">Config exported successfully.</div>
      )}
      {exportError && (
        <div className="settings-error" role="alert">{exportError}</div>
      )}
    </SettingsSection>
  );
}
