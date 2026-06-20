import { useRef, useState, type ReactElement, type ChangeEvent } from "react";
import type { ExportedConfig, ForwardRuleResponse, ImportMode } from "@portier/shared";
import { importConfig } from "../../api/portierApi.js";
import { SettingsSection } from "./SettingsSection.js";
import type { ConfigExport } from "./useConfigExport.js";

interface ParsedImport {
  config: ExportedConfig;
  ruleCount: number;
  tcpCount: number;
  udpCount: number;
  enabledCount: number;
}

// ImportConfigSection renders the "Import Config" panel: merge/replace mode,
// file picker with a parsed preview, and a confirm-replace flow. The replace
// confirmation reuses the shared config-export flow for its "backup" button, so
// the `exporting` flag stays shared with the Export Config panel.
export function ImportConfigSection({
  onRulesUpdated,
  configExport
}: {
  onRulesUpdated: (rules: ForwardRuleResponse[]) => void;
  configExport: ConfigExport;
}): ReactElement {
  const { exporting, handleExport } = configExport;
  const [importMode, setImportMode] = useState<ImportMode>("merge");
  const [parsedImport, setParsedImport] = useState<ParsedImport | null>(null);
  const [parseError, setParseError] = useState<string | null>(null);
  const [importing, setImporting] = useState(false);
  const [importResult, setImportResult] = useState<string | null>(null);
  const [importErrors, setImportErrors] = useState<string[]>([]);
  const [confirmReplace, setConfirmReplace] = useState(false);
  const [selectedFileName, setSelectedFileName] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  function handleFileChange(e: ChangeEvent<HTMLInputElement>): void {
    setParsedImport(null);
    setParseError(null);
    setImportResult(null);
    setImportErrors([]);
    setConfirmReplace(false);

    const file = e.target.files?.[0];
    setSelectedFileName(file?.name ?? null);
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
        const rules = parsed.rules;
        setParsedImport({
          config: parsed,
          ruleCount: rules.length,
          tcpCount: rules.filter((r) => (r as { protocol: string }).protocol === "tcp").length,
          udpCount: rules.filter((r) => (r as { protocol: string }).protocol === "udp").length,
          enabledCount: rules.filter((r) => (r as { enabled: boolean }).enabled).length
        });
      } catch (error) {
        /* v8 ignore next -- JSON.parse only throws SyntaxError (an Error); the non-Error fallback is defensive */
        setParseError(error instanceof Error ? error.message : "Parse error.");
      }
    };
    reader.readAsText(file);
  }

  async function handleImport(): Promise<void> {
    /* v8 ignore next -- the Import button only renders once parsedImport is set; this guard is unreachable from the UI */
    if (!parsedImport) return;
    setImporting(true);
    setImportResult(null);
    setImportErrors([]);
    try {
      const response = await importConfig(parsedImport.config, importMode);
      setImportResult(
        `Import complete: ${response.result.imported} rule(s) added, ${response.result.skipped} skipped.`
      );
      onRulesUpdated(response.rules);
      setParsedImport(null);
      setConfirmReplace(false);
      setSelectedFileName(null);
      if (fileInputRef.current) fileInputRef.current.value = "";
    } catch (error) {
      setImportErrors([error instanceof Error ? error.message : "Import failed."]);
    } finally {
      setImporting(false);
    }
  }

  function handleImportClick(): void {
    if (importMode === "replace") {
      setConfirmReplace(true);
    } else {
      void handleImport();
    }
  }

  return (
    <SettingsSection title="Import Config">
      <p className="settings-desc">
        Import rules from a previously exported Portier config file.
        All rules are validated before applying — the import is atomic (all-or-nothing).
      </p>

      {/* Mode selection — shown above file picker */}
      <div className="settings-import-mode">
        <label className="settings-mode-option">
          <input
            type="radio"
            name="import-mode"
            value="merge"
            checked={importMode === "merge"}
            onChange={() => { setImportMode("merge"); setConfirmReplace(false); }}
          />
          <strong>Merge</strong> — adds rules from the file; skips rules with conflicting listen port bindings.
        </label>
        <label className="settings-mode-option">
          <input
            type="radio"
            name="import-mode"
            value="replace"
            checked={importMode === "replace"}
            onChange={() => { setImportMode("replace"); setConfirmReplace(false); }}
          />
          <strong>Replace</strong> — stops and deletes all current rules, then applies imported rules.
          Export a backup first if you want to keep your current rules.
        </label>
      </div>

      <div className="settings-import-controls">
        <div className="settings-file-picker">
          <input
            ref={fileInputRef}
            id="import-file-input"
            type="file"
            accept=".json,application/json"
            onChange={handleFileChange}
            aria-label="Select config file"
            className="settings-file-input-hidden"
          />
          <label htmlFor="import-file-input" className="settings-file-btn">
            Choose file
          </label>
          <span className="settings-file-name">
            {selectedFileName ?? "No file chosen"}
          </span>
        </div>

        {parseError && (
          <div className="settings-error" role="alert">{parseError}</div>
        )}

        {parsedImport && (
          <div className="settings-import-preview">
            <div className="settings-preview-title">File preview</div>
            <div className="settings-row">
              <span className="settings-label">Rules</span>
              <span>{parsedImport.ruleCount} ({parsedImport.tcpCount} TCP, {parsedImport.udpCount} UDP)</span>
            </div>
            <div className="settings-row">
              <span className="settings-label">Autostart</span>
              <span>{parsedImport.enabledCount} enabled</span>
            </div>

            {importErrors.length > 0 && (
              <div className="settings-error" role="alert">
                {importErrors.map((e) => <p key={e}>{e}</p>)}
              </div>
            )}

            {!confirmReplace ? (
              <button
                type="button"
                className={importMode === "replace" ? "danger" : "primary"}
                disabled={importing}
                onClick={handleImportClick}
              >
                {importing ? "Importing…" : importMode === "replace" ? "Replace All Rules" : "Import Rules"}
              </button>
            ) : (
              <div className="settings-confirm-replace">
                <p className="settings-confirm-text">
                  This will delete all existing rules and replace them with the imported ones.
                  Running rules will be stopped. Are you sure?
                </p>
                <button
                  type="button"
                  onClick={handleExport}
                  disabled={importing || exporting}
                >
                  {exporting ? "Exporting…" : "Export current config as backup"}
                </button>
                <button
                  type="button"
                  className="danger"
                  disabled={importing}
                  onClick={() => void handleImport()}
                >
                  {importing ? "Replacing…" : "Confirm Replace"}
                </button>
                <button
                  type="button"
                  onClick={() => setConfirmReplace(false)}
                  disabled={importing}
                >
                  Cancel
                </button>
              </div>
            )}
          </div>
        )}

        {importResult && (
          <div className="settings-success" role="status">{importResult}</div>
        )}
      </div>
    </SettingsSection>
  );
}
