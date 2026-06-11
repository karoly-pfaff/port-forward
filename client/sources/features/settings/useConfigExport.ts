import { useState } from "react";
import { exportConfig } from "../../api/portierApi.js";

function makeExportFilename(): string {
  const d = new Date();
  const pad = (n: number) => String(n).padStart(2, "0");
  const date = `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}`;
  const time = `${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`;
  return `portier-config-${date}-${time}.json`;
}

export interface ConfigExport {
  exporting: boolean;
  exportSuccess: boolean;
  exportError: string | null;
  handleExport: () => Promise<void>;
}

// useConfigExport owns the config-download flow. It is created once in
// SettingsView and shared by the Export Config panel and the Import panel's
// "Export current config as backup" button, so the in-progress `exporting` flag
// stays shared across both (preserving the previous single-state behaviour).
export function useConfigExport(): ConfigExport {
  const [exporting, setExporting] = useState(false);
  const [exportSuccess, setExportSuccess] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

  async function handleExport(): Promise<void> {
    setExporting(true);
    setExportSuccess(false);
    setExportError(null);
    try {
      const config = await exportConfig();
      const json = JSON.stringify(config, null, 2);
      const blob = new Blob([json], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = makeExportFilename();
      a.click();
      URL.revokeObjectURL(url);
      setExportSuccess(true);
      setTimeout(() => setExportSuccess(false), 3000);
    } catch (error) {
      setExportError(error instanceof Error ? error.message : "Export failed.");
    } finally {
      setExporting(false);
    }
  }

  return { exporting, exportSuccess, exportError, handleExport };
}
