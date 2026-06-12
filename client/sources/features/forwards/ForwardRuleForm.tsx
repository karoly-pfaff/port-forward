import { useMemo, useState, type ReactElement } from "react";
import { Globe, Lock } from "lucide-react";
import type { ForwardProtocol, ForwardRule, ForwardRuleInput, RuntimeInfo, UdpMode } from "@portier/shared";
import {
  PORTIER_GROUP_MAX_LENGTH,
  PORTIER_RECOMMENDED_FORWARD_PORT_MAX,
  PORTIER_RECOMMENDED_FORWARD_PORT_MIN,
  getPortAdvisories,
  validateForwardRule,
} from "@portier/shared";
import { emptyForm, formToPayload, ruleToDuplicateForm, ruleToForm, type RuleFormState } from "./RuleForm.js";

const ADVISORY_TITLES: Record<string, string> = {
  COMMON_PORT: "Common Port",
  PRIVILEGED_PORT: "Privileged Port",
  OUTSIDE_RECOMMENDED_RANGE: "Outside Recommended Range",
  MANAGEMENT_LAN_EXPOSURE: "Management LAN Exposure",
};

function friendlyErrorMessage(raw: string): string {
  const lower = raw.toLowerCase();
  if (
    lower.includes("already listening") ||
    lower.includes("duplicate") ||
    lower.includes("already in use") ||
    (lower.includes("conflict") && (lower.includes("listen") || lower.includes("binding")))
  ) {
    return "Another rule is already using this protocol, listen host, and listen port. Choose a different listen port, or stop/remove the conflicting rule.";
  }
  return raw;
}

// IPv4 | hostname (including dotted labels) | simplified IPv6
const HOST_PATTERN =
  /^(?:(?:\d{1,3}\.){3}\d{1,3}|(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?\.)*[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?|(?:[0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4})$/;

interface ForwardRuleFormProps {
  editingRule: ForwardRule | undefined;
  // When set (and not editing), the form opens in *create* mode pre-filled
  // from this rule — the duplicate-rule flow (v1.8 Slice 8). The source rule
  // is never mutated; save goes through the create path.
  duplicateSource?: ForwardRule;
  rules?: ForwardRule[];
  onSave: (id: string | undefined, payload: ForwardRuleInput) => Promise<void>;
  onCancel: () => void;
  onDelete?: (rule: ForwardRule) => void;
  saving: boolean;
  runtimePlatform?: RuntimeInfo["platform"];
}

export function ForwardRuleForm({
  editingRule,
  duplicateSource,
  rules = [],
  onSave,
  onCancel,
  onDelete,
  saving,
  runtimePlatform,
}: ForwardRuleFormProps): ReactElement {
  const isDuplicating = !editingRule && !!duplicateSource;
  const [form, setForm] = useState<RuleFormState>(() =>
    editingRule
      ? ruleToForm(editingRule)
      : duplicateSource
        ? ruleToDuplicateForm(duplicateSource)
        : emptyForm
  );
  const [formErrors, setFormErrors] = useState<string[]>([]);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [nameTouched, setNameTouched] = useState(false);

  const nameTrimmed = form.name.trim();
  const nameEmpty = nameTrimmed.length === 0;
  const nameDuplicate =
    !nameEmpty &&
    rules.some(
      (r) => r.name.trim().toLowerCase() === nameTrimmed.toLowerCase() && r.id !== form.id
    );
  const showNameEmptyError = nameTouched && nameEmpty;
  const showNameDuplicateError = nameDuplicate;

  const listenPortNum = Number(form.listenPort);
  const listenPortValid =
    Number.isInteger(listenPortNum) && listenPortNum >= 1 && listenPortNum <= 65535;
  const targetPortNum = Number(form.targetPort);
  const targetPortValid =
    Number.isInteger(targetPortNum) && targetPortNum >= 1 && targetPortNum <= 65535;

  const showListenPortError = form.listenPort.trim().length > 0 && !listenPortValid;
  const showTargetPortError = form.targetPort.trim().length > 0 && !targetPortValid;

  const listenHostTrimmed = form.listenHost.trim();
  const targetHostTrimmed = form.targetHost.trim();
  const listenHostValid = listenHostTrimmed.length === 0 || HOST_PATTERN.test(listenHostTrimmed);
  const targetHostValid = targetHostTrimmed.length === 0 || HOST_PATTERN.test(targetHostTrimmed);
  const showListenHostError = listenHostTrimmed.length > 0 && !listenHostValid;
  const showTargetHostError = targetHostTrimmed.length > 0 && !targetHostValid;

  const formAdvisories = useMemo(
    () =>
      listenPortValid
        ? getPortAdvisories({
            port: listenPortNum,
            listenHost: form.listenHost,
            purpose: "forward",
          })
        : [],
    [form.listenHost, listenPortValid, listenPortNum]
  );

  const canSubmit =
    !saving &&
    !nameDuplicate &&
    !showListenPortError &&
    !showTargetPortError &&
    !showListenHostError &&
    !showTargetHostError;

  const isEditing = !!editingRule;

  async function handleSubmit(event: React.FormEvent): Promise<void> {
    event.preventDefault();
    setNameTouched(true);
    if (nameEmpty) return;
    const payload = formToPayload(form);
    const validation = validateForwardRule(payload);
    if (!validation.valid) {
      setFormErrors(validation.errors);
      return;
    }
    setFormErrors([]);
    try {
      await onSave(form.id, payload);
    } catch (error) {
      const raw = error instanceof Error ? error.message : "Save failed.";
      setFormErrors([friendlyErrorMessage(raw)]);
    }
  }

  function setField<K extends keyof RuleFormState>(key: K, value: RuleFormState[K]): void {
    setForm((cur) => ({ ...cur, [key]: value }));
  }

  return (
    <>
      {/* Drawer header */}
      <div className="drawer-header">
        <div className="drawer-header-content">
          <h2 className="drawer-title">
            {isEditing ? "Edit Rule" : isDuplicating ? "Duplicate Rule" : "Add Rule"}
          </h2>
          {isEditing && editingRule && (
            <div className="drawer-subtitle">
              <span className={`protocol-badge protocol-badge--${editingRule.protocol}`}>
                {editingRule.protocol.toUpperCase()}
              </span>
              <span className="drawer-subtitle-name">{editingRule.name}</span>
            </div>
          )}
          {isDuplicating && duplicateSource && (
            <div className="drawer-subtitle">
              <span className="drawer-subtitle-name">New rule copied from &ldquo;{duplicateSource.name}&rdquo;</span>
            </div>
          )}
        </div>
        <button
          type="button"
          className="drawer-close"
          onClick={onCancel}
          aria-label="Close"
        >
          ✕
        </button>
      </div>

      {/* Drawer body */}
      <div className="drawer-body">
        {formErrors.length > 0 && (
          <div className="errors" role="alert" aria-live="polite">
            <span className="advisory-card-title">Error</span>
            {formErrors.map((e) => (
              <p key={e}>{e}</p>
            ))}
          </div>
        )}

        <form id="rule-form" onSubmit={handleSubmit} noValidate>
          <label htmlFor="rule-name">
            Name
            <input
              id="rule-name"
              value={form.name}
              onChange={(e) => setField("name", e.target.value)}
              onBlur={() => setNameTouched(true)}
              autoComplete="off"
              aria-describedby={
                showNameEmptyError
                  ? "name-empty-error"
                  : showNameDuplicateError
                    ? "name-duplicate-error"
                    : undefined
              }
              aria-invalid={showNameEmptyError || showNameDuplicateError ? "true" : undefined}
            />
            {showNameEmptyError && (
              <span id="name-empty-error" className="field-error" role="alert">
                Name is required
              </span>
            )}
            {showNameDuplicateError && (
              <span id="name-duplicate-error" className="field-error" role="alert">
                Name already exists
              </span>
            )}
          </label>

          <label htmlFor="rule-group">
            Group
            <input
              id="rule-group"
              aria-label="Group"
              value={form.group}
              onChange={(e) => setField("group", e.target.value)}
              maxLength={PORTIER_GROUP_MAX_LENGTH}
              autoComplete="off"
              aria-describedby="rule-group-hint"
            />
            <span id="rule-group-hint" className="label-hint">
              Optional label to organize rules (up to {PORTIER_GROUP_MAX_LENGTH} characters). Leave blank for no group.
            </span>
          </label>

          <div className="form-row">
            <label htmlFor="rule-protocol">
              Protocol
              <select
                id="rule-protocol"
                value={form.protocol}
                onChange={(e) => setField("protocol", e.target.value as ForwardProtocol)}
              >
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
              </select>
            </label>

            <label htmlFor="rule-udp-mode">
              UDP Mode
              <select
                id="rule-udp-mode"
                value={form.protocol !== "udp" ? "" : form.udpMode}
                disabled={form.protocol !== "udp"}
                onChange={(e) => setField("udpMode", e.target.value as UdpMode)}
              >
                {form.protocol !== "udp" && <option value="" disabled>Not available</option>}
                <option value="one-way">One-way</option>
                <option value="bidirectional-last-client">
                  Bidirectional – last client
                </option>
                <option value="bidirectional-multi-client">
                  Bidirectional – multi-client
                </option>
              </select>
            </label>
          </div>

          <div className="form-row">
            <label htmlFor="rule-listen-host">
              Listen Host
              <input
                id="rule-listen-host"
                aria-label="Listen Host"
                value={form.listenHost}
                onChange={(e) => setField("listenHost", e.target.value)}
                autoComplete="off"
                aria-describedby={showListenHostError ? "listen-host-error" : undefined}
                aria-invalid={showListenHostError ? "true" : undefined}
              />
              {showListenHostError && (
                <span id="listen-host-error" className="field-error" role="alert">
                  Must be a valid IP or hostname
                </span>
              )}
            </label>

            <label htmlFor="rule-listen-port">
              Listen Port
              <input
                id="rule-listen-port"
                type="number"
                min="1"
                max="65535"
                value={form.listenPort}
                onChange={(e) => setField("listenPort", e.target.value)}
                aria-describedby={showListenPortError ? "listen-port-error" : undefined}
                aria-invalid={showListenPortError ? "true" : undefined}
              />
              {showListenPortError && (
                <span id="listen-port-error" className="field-error" role="alert">
                  Must be 1–65535
                </span>
              )}
            </label>
          </div>

          <div className="listen-host-field">
            <div className="listen-host-presets" role="group" aria-label="Listen host presets">
              <button
                type="button"
                className={`preset-btn preset-btn--local${form.listenHost === "127.0.0.1" ? " preset-btn--active" : ""}`}
                onClick={() => setField("listenHost", "127.0.0.1")}
                aria-pressed={form.listenHost === "127.0.0.1"}
              >
                <Lock size={13} aria-hidden="true" />
                Local only
              </button>
              <button
                type="button"
                className={`preset-btn preset-btn--lan${form.listenHost === "0.0.0.0" ? " preset-btn--active" : ""}`}
                onClick={() => setField("listenHost", "0.0.0.0")}
                aria-pressed={form.listenHost === "0.0.0.0"}
              >
                <Globe size={13} aria-hidden="true" />
                LAN exposed
              </button>
            </div>
            {!showListenHostError && form.listenHost === "127.0.0.1" && (
              <span className="label-hint">Only this computer can connect to this forwarded port.</span>
            )}
            {!showListenHostError && form.listenHost === "0.0.0.0" && (
              <span className="label-hint">Other devices on your network may be able to connect if firewall rules allow it.</span>
            )}
          </div>

          {form.listenHost === "0.0.0.0" && (
            <div className="advisory-card advisory-card--warning" role="alert">
              <div>
                <span className="advisory-card-title">LAN Exposure</span>
                This rule listens on all network interfaces. Devices on your LAN may reach it
                if firewall settings allow the connection. Portier does not create firewall rules
                automatically. Use 127.0.0.1 for local-only access.
              </div>
            </div>
          )}

          <div className="form-row">
            <label htmlFor="rule-target-host">
              Target Host
              <input
                id="rule-target-host"
                value={form.targetHost}
                onChange={(e) => setField("targetHost", e.target.value)}
                autoComplete="off"
                aria-describedby={showTargetHostError ? "target-host-error" : undefined}
                aria-invalid={showTargetHostError ? "true" : undefined}
              />
              {showTargetHostError && (
                <span id="target-host-error" className="field-error" role="alert">
                  Must be a valid IP or hostname
                </span>
              )}
            </label>

            <label htmlFor="rule-target-port">
              Target Port
              <input
                id="rule-target-port"
                type="number"
                min="1"
                max="65535"
                value={form.targetPort}
                onChange={(e) => setField("targetPort", e.target.value)}
                aria-describedby={showTargetPortError ? "target-port-error" : undefined}
                aria-invalid={showTargetPortError ? "true" : undefined}
              />
              {showTargetPortError && (
                <span id="target-port-error" className="field-error" role="alert">
                  Must be 1–65535
                </span>
              )}
            </label>
          </div>

          <div className="field-block">
            <span>Autostart</span>
            <label className="check" htmlFor="rule-autostart">
              <input
                id="rule-autostart"
                type="checkbox"
                checked={form.enabled}
                onChange={(e) => setField("enabled", e.target.checked)}
              />
              <span className="label-hint">Start this rule when Portier starts</span>
            </label>
          </div>
        </form>

        {/* Advisory cards */}
        <div className="advisory-cards" aria-live="polite">
          <div className="advisory-card advisory-card--info">
            <div>
              <span className="advisory-card-title">Recommended forward range</span>
              For best results, use ports in {PORTIER_RECOMMENDED_FORWARD_PORT_MIN}–{PORTIER_RECOMMENDED_FORWARD_PORT_MAX}.
            </div>
          </div>
          {form.listenHost === "0.0.0.0" && (
            <div className="advisory-card advisory-card--info">
              <div>
                <span className="advisory-card-title">Firewall note</span>
                {runtimePlatform === "windows"
                  ? "Windows may ask for firewall permission the first time Portier opens a listening port. Portier does not create firewall rules automatically."
                  : "Your operating system firewall may still block LAN connections. Portier does not create firewall rules automatically."}
              </div>
            </div>
          )}
          {formAdvisories
            .filter((a) => a.code !== "LAN_EXPOSURE")
            .map((advisory) => (
              <div
                key={`${advisory.code}-${advisory.message}`}
                className={`advisory-card advisory-card--${advisory.severity}`}
              >
                <div>
                  {ADVISORY_TITLES[advisory.code] && (
                    <span className="advisory-card-title">{ADVISORY_TITLES[advisory.code]}</span>
                  )}
                  {advisory.message}
                </div>
              </div>
            ))}
        </div>
      </div>

      {/* Drawer footer */}
      <div className="drawer-footer">
        {isEditing && (
          <button type="button" onClick={onCancel} disabled={saving}>
            Cancel
          </button>
        )}
        {isEditing && onDelete && editingRule && (
          confirmingDelete ? (
            <>
              <button
                type="button"
                className="danger"
                onClick={() => onDelete(editingRule)}
                disabled={saving}
              >
                Confirm Delete
              </button>
              <button
                type="button"
                onClick={() => setConfirmingDelete(false)}
                disabled={saving}
              >
                Cancel Delete
              </button>
            </>
          ) : (
            <button
              type="button"
              className="danger"
              onClick={() => setConfirmingDelete(true)}
              disabled={saving}
            >
              Delete Rule
            </button>
          )
        )}
        <div className="drawer-footer-gap" />
        <button
          type="submit"
          form="rule-form"
          className="primary"
          disabled={!canSubmit}
        >
          {isEditing ? "Save Changes" : "Add Rule"}
        </button>
      </div>
    </>
  );
}
