import { useRef, useState, useEffect, type ReactElement, type ChangeEvent } from "react";
import type { ExportedConfig, ForwardRuleResponse, ImportMode, RuntimeInfo } from "@portier/shared";
import {
  PORTIER_DEFAULT_HOST,
  PORTIER_DEFAULT_PORT,
  PORTIER_RECOMMENDED_FORWARD_PORT_MAX,
  PORTIER_RECOMMENDED_FORWARD_PORT_MIN
} from "@portier/shared";
import { exportConfig, importConfig, fetchRuntimeInfo } from "../../api/portierApi.js";

function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  return `${h}h ${m}m`;
}

interface SettingsViewProps {
  onRulesUpdated: (rules: ForwardRuleResponse[]) => void;
}

interface ParsedImport {
  config: ExportedConfig;
  ruleCount: number;
  tcpCount: number;
  udpCount: number;
  enabledCount: number;
}

export function SettingsView({ onRulesUpdated }: SettingsViewProps): ReactElement {
  const [exporting, setExporting] = useState(false);
  const [importMode, setImportMode] = useState<ImportMode>("merge");
  const [parsedImport, setParsedImport] = useState<ParsedImport | null>(null);
  const [parseError, setParseError] = useState<string | null>(null);
  const [importing, setImporting] = useState(false);
  const [importResult, setImportResult] = useState<string | null>(null);
  const [importErrors, setImportErrors] = useState<string[]>([]);
  const [confirmReplace, setConfirmReplace] = useState(false);
  const [selectedFileName, setSelectedFileName] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [runtimeInfo, setRuntimeInfo] = useState<RuntimeInfo | null>(null);
  const [runtimeLoading, setRuntimeLoading] = useState(true);
  const [runtimeUnavailable, setRuntimeUnavailable] = useState(false);

  useEffect(() => {
    fetchRuntimeInfo()
      .then((info) => {
        setRuntimeInfo(info);
        setRuntimeLoading(false);
      })
      .catch(() => {
        setRuntimeUnavailable(true);
        setRuntimeLoading(false);
      });
  }, []);

  async function handleExport(): Promise<void> {
    setExporting(true);
    try {
      const config = await exportConfig();
      const json = JSON.stringify(config, null, 2);
      const date = new Date().toISOString().slice(0, 10);
      const blob = new Blob([json], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `portier-rules-${date}.json`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (error) {
      alert(`Export failed: ${error instanceof Error ? error.message : String(error)}`);
    } finally {
      setExporting(false);
    }
  }

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
        setParseError(error instanceof Error ? error.message : "Parse error.");
      }
    };
    reader.readAsText(file);
  }

  async function handleImport(): Promise<void> {
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
    <div className="settings-view">
      <div className="rule-list-section" style={{ flex: 1, minHeight: 0 }}>
        <div className="rule-list-header">
          <div className="rule-list-title-group">
            <div className="rule-list-title">Settings</div>
            <div className="rule-list-subtitle">Management endpoint, port range, config export/import, and about</div>
          </div>
        </div>
        <div className="rule-list-body">
          <div className="settings-item-list">

            {/* Management endpoint */}
            <div className="settings-section">
              <div className="settings-section-title">Management Endpoint</div>
              <div className="settings-row">
                <span className="settings-label">Default address</span>
                <code className="settings-value">{PORTIER_DEFAULT_HOST}:{PORTIER_DEFAULT_PORT}</code>
              </div>
              <p className="settings-desc">
                The management UI and API only bind to {PORTIER_DEFAULT_HOST} by default.
                Binding to 0.0.0.0 exposes the management interface on the LAN — avoid this unless deliberately needed.
              </p>
            </div>

            {/* Recommended port range */}
            <div className="settings-section">
              <div className="settings-section-title">Recommended Forward Port Range</div>
              <div className="settings-row">
                <span className="settings-label">Range</span>
                <code className="settings-value">{PORTIER_RECOMMENDED_FORWARD_PORT_MIN}–{PORTIER_RECOMMENDED_FORWARD_PORT_MAX}</code>
              </div>
              <p className="settings-desc">
                Ports outside this range are warned about but not blocked.
                Common ports (HTTP, database, etc.) are flagged in the rule form.
              </p>
            </div>

            {/* Config export */}
            <div className="settings-section">
              <div className="settings-section-title">Export Config</div>
              <p className="settings-desc">
                Download all current rules as a portable JSON file.
              </p>
              <div>
                <button type="button" onClick={handleExport} disabled={exporting}>
                  {exporting ? "Exporting…" : "Download rules.json"}
                </button>
              </div>
            </div>

            {/* Config import */}
            <div className="settings-section">
              <div className="settings-section-title">Import Config</div>
              <p className="settings-desc">
                Import rules from a previously exported Portier config file.
                All rules are validated before applying — the import is atomic (all-or-nothing).
              </p>

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

                    <div className="settings-row settings-mode-row">
                      <span className="settings-label">Mode</span>
                      <label className="settings-mode-option">
                        <input
                          type="radio"
                          name="import-mode"
                          value="merge"
                          checked={importMode === "merge"}
                          onChange={() => { setImportMode("merge"); setConfirmReplace(false); }}
                        />
                        Merge (add rules, skip conflicts)
                      </label>
                      <label className="settings-mode-option">
                        <input
                          type="radio"
                          name="import-mode"
                          value="replace"
                          checked={importMode === "replace"}
                          onChange={() => { setImportMode("replace"); setConfirmReplace(false); }}
                        />
                        Replace (delete all existing rules first)
                      </label>
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
            </div>

            {/* Runtime / Environment */}
            <div className="settings-section">
              <div className="settings-section-title">Runtime / Environment</div>
              {runtimeLoading ? (
                <p className="settings-desc">Loading runtime info…</p>
              ) : runtimeUnavailable ? (
                <p className="settings-desc settings-unavailable">Runtime information is unavailable from this backend.</p>
              ) : runtimeInfo ? (
                <>
                  <div className="settings-row">
                    <span className="settings-label">Runtime</span>
                    <code className="settings-value">{runtimeInfo.runtime === "go" ? "Go service" : "Node server"}</code>
                  </div>
                  <div className="settings-row">
                    <span className="settings-label">Version</span>
                    <code className="settings-value">{runtimeInfo.version}</code>
                  </div>
                  <div className="settings-row">
                    <span className="settings-label">Platform</span>
                    <code className="settings-value">{runtimeInfo.platform} / {runtimeInfo.arch}</code>
                  </div>
                  <div className="settings-row">
                    <span className="settings-label">Uptime</span>
                    <span className="settings-value">{formatUptime(runtimeInfo.uptimeSeconds)}</span>
                  </div>
                  <div className="settings-row">
                    <span className="settings-label">Management</span>
                    <code className="settings-value">{runtimeInfo.managementHost}:{runtimeInfo.managementPort}</code>
                  </div>
                  <div className="settings-row">
                    <span className="settings-label">Config path</span>
                    <code className="settings-value settings-value-path">{runtimeInfo.configPath}</code>
                  </div>
                  <div className="settings-row">
                    <span className="settings-label">Static dir</span>
                    <code className="settings-value settings-value-path">{runtimeInfo.staticDir}</code>
                  </div>
                  <div className="settings-row">
                    <span className="settings-label">Service mode</span>
                    <span className="settings-value">{runtimeInfo.serviceMode ? "Yes" : "No"}</span>
                  </div>
                  <div className="settings-row">
                    <span className="settings-label">PID</span>
                    <code className="settings-value">{runtimeInfo.pid}</code>
                  </div>
                </>
              ) : null}
            </div>

            {/* About */}
            <div className="settings-section">
              <div className="settings-section-title">About Portier</div>
              <p className="settings-desc">
                Portier is a local TCP/UDP port forwarding manager. Activity logs are in-memory only and reset on server restart.
                Rules are persisted to an external JSON file.
              </p>
            </div>

          </div>
        </div>
      </div>
    </div>
  );
}
