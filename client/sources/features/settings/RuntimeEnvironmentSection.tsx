import type { ReactElement } from "react";
import { SettingsSection } from "./SettingsSection.js";
import { useRuntimeInfo } from "./useRuntimeInfo.js";
import { useClipboardCopy } from "./useClipboardCopy.js";

function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  return `${h}h ${m}m`;
}

// RuntimeEnvironmentSection renders the "Runtime / Environment" panel: it fetches
// runtime info on mount and exposes copy buttons for the management URL, config
// path, and static dir.
export function RuntimeEnvironmentSection(): ReactElement {
  const { runtimeInfo, runtimeLoading, runtimeUnavailable } = useRuntimeInfo();
  const { copyToClipboard, copyLabel } = useClipboardCopy();

  return (
    <SettingsSection title="Runtime / Environment">
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
          <div className="settings-row settings-row-copyable">
            <span className="settings-label">Management</span>
            <code className="settings-value">{runtimeInfo.managementHost}:{runtimeInfo.managementPort}</code>
            <button
              type="button"
              className="settings-copy-btn"
              aria-label="Copy management URL"
              onClick={() => void copyToClipboard(`${runtimeInfo.managementHost}:${runtimeInfo.managementPort}`, "managementUrl")}
            >
              {copyLabel("managementUrl")}
            </button>
          </div>
          <div className="settings-row settings-row-copyable">
            <span className="settings-label">Config path</span>
            <code className="settings-value settings-value-path">{runtimeInfo.configPath}</code>
            <button
              type="button"
              className="settings-copy-btn"
              aria-label="Copy config path"
              onClick={() => void copyToClipboard(runtimeInfo.configPath, "configPath")}
            >
              {copyLabel("configPath")}
            </button>
          </div>
          <div className="settings-row settings-row-copyable">
            <span className="settings-label">Static dir</span>
            <code className="settings-value settings-value-path">{runtimeInfo.staticDir}</code>
            <button
              type="button"
              className="settings-copy-btn"
              aria-label="Copy static dir"
              onClick={() => void copyToClipboard(runtimeInfo.staticDir, "staticDir")}
            >
              {copyLabel("staticDir")}
            </button>
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
    </SettingsSection>
  );
}
