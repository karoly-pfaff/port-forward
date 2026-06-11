import type { ReactElement } from "react";
import type { ForwardRuleResponse } from "@portier/shared";
import {
  PORTIER_DEFAULT_HOST,
  PORTIER_DEFAULT_PORT,
  PORTIER_RECOMMENDED_FORWARD_PORT_MAX,
  PORTIER_RECOMMENDED_FORWARD_PORT_MIN
} from "@portier/shared";
import type { DiagnosisEntry } from "../forwards/ForwardRuleList.js";
import { SettingsSection } from "./SettingsSection.js";
import { ExportConfigSection } from "./ExportConfigSection.js";
import { ImportConfigSection } from "./ImportConfigSection.js";
import { PlanApplySection } from "./PlanApplySection.js";
import { RuntimeEnvironmentSection } from "./RuntimeEnvironmentSection.js";
import { DiagnosticsExportSection } from "./DiagnosticsExportSection.js";
import { useConfigExport } from "./useConfigExport.js";

interface SettingsViewProps {
  onRulesUpdated: (rules: ForwardRuleResponse[]) => void;
  diagnosisMap?: ReadonlyMap<string, DiagnosisEntry>;
}

// SettingsView composes the settings panels. Each stateful panel owns its own
// state; the config-export flow is the one cross-panel concern (shared by the
// Export panel and the Import panel's backup button), so it is created here once
// and passed to both.
export function SettingsView({ onRulesUpdated, diagnosisMap }: SettingsViewProps): ReactElement {
  const configExport = useConfigExport();

  return (
    <div className="settings-view">
      <div className="rule-list-section" style={{ flex: 1, minHeight: 0 }}>
        <div className="rule-list-header">
          <div className="rule-list-title-group">
            <div className="rule-list-title">Settings</div>
            <div className="rule-list-subtitle">Management endpoint, port range, config export/import, and runtime</div>
          </div>
        </div>
        <div className="rule-list-body">
          <div className="settings-item-list">

            {/* Management endpoint */}
            <SettingsSection title="Management Endpoint">
              <div className="settings-row">
                <span className="settings-label">Default address</span>
                <code className="settings-value">{PORTIER_DEFAULT_HOST}:{PORTIER_DEFAULT_PORT}</code>
              </div>
              <p className="settings-desc">
                The management UI and API only bind to {PORTIER_DEFAULT_HOST} by default.
                Binding to 0.0.0.0 exposes the management interface on the LAN — avoid this unless deliberately needed.
              </p>
            </SettingsSection>

            {/* Recommended port range */}
            <SettingsSection title="Recommended Forward Port Range">
              <div className="settings-row">
                <span className="settings-label">Range</span>
                <code className="settings-value">{PORTIER_RECOMMENDED_FORWARD_PORT_MIN}–{PORTIER_RECOMMENDED_FORWARD_PORT_MAX}</code>
              </div>
              <p className="settings-desc">
                Ports outside this range are warned about but not blocked.
                Common ports (HTTP, database, etc.) are flagged in the rule form.
              </p>
            </SettingsSection>

            <ExportConfigSection configExport={configExport} />

            <PlanApplySection onRulesUpdated={onRulesUpdated} />

            <ImportConfigSection onRulesUpdated={onRulesUpdated} configExport={configExport} />

            <RuntimeEnvironmentSection />

            <DiagnosticsExportSection diagnosisMap={diagnosisMap} />

            {/* About */}
            <SettingsSection title="About Portier">
              <p className="settings-desc">
                Portier is a local TCP/UDP port forwarding manager. Activity logs are in-memory only and reset on server restart.
                Rules are persisted to an external JSON file.
              </p>
            </SettingsSection>

          </div>
        </div>
      </div>
    </div>
  );
}
