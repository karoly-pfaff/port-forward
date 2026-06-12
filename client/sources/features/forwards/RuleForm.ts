import type { ForwardProtocol, ForwardRule, ForwardRuleInput, UdpMode } from "@portier/shared";
import { PORTIER_RECOMMENDED_FORWARD_PORT_MIN } from "@portier/shared";

export type RuleFormState = {
  id?: string;
  name: string;
  protocol: ForwardProtocol;
  listenHost: string;
  listenPort: string;
  targetHost: string;
  targetPort: string;
  enabled: boolean;
  udpMode: UdpMode;
  group: string;
};

export const emptyForm: RuleFormState = {
  name: "",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: String(PORTIER_RECOMMENDED_FORWARD_PORT_MIN),
  targetHost: "127.0.0.1",
  targetPort: "80",
  enabled: false,
  udpMode: "one-way",
  group: ""
};

export function ruleToForm(rule: ForwardRule): RuleFormState {
  return {
    id: rule.id,
    name: rule.name,
    protocol: rule.protocol,
    listenHost: rule.listenHost,
    listenPort: String(rule.listenPort),
    targetHost: rule.targetHost,
    targetPort: String(rule.targetPort),
    enabled: rule.enabled,
    udpMode: rule.udpMode ?? "one-way",
    group: rule.group ?? ""
  };
}

// Build the initial form state for a *duplicate* (v1.8 Slice 8): a brand-new
// rule pre-filled from an existing one. The source rule is never touched. We
// keep only the editable definition fields (via `ruleToForm`) and then:
//   - drop the id so the form saves through the create path (POST), not PATCH
//   - adjust the name so it is clearly a copy
//   - force `enabled: false` so the duplicate cannot auto-start on save
//     (a freshly created enabled rule starts immediately — see addRule)
// Runtime-only fields (status/lastError/health/active connections/sessions)
// are never part of the form, so they are inherently excluded.
export function ruleToDuplicateForm(rule: ForwardRule): RuleFormState {
  return {
    ...ruleToForm(rule),
    id: undefined,
    name: duplicateName(rule.name),
    enabled: false
  };
}

export function duplicateName(name: string): string {
  const trimmed = name.trim();
  return trimmed.length > 0 ? `${trimmed} copy` : "copy";
}

export function formToPayload(form: RuleFormState): ForwardRuleInput {
  return {
    name: form.name,
    protocol: form.protocol,
    listenHost: form.listenHost,
    listenPort: Number(form.listenPort),
    targetHost: form.targetHost,
    targetPort: Number(form.targetPort),
    enabled: form.enabled,
    udpMode: form.protocol === "udp" ? form.udpMode : undefined,
    // Always send `group` so the API contract handles all three cases: an
    // empty string clears the group on PATCH (and normalizes to absent on
    // create), a non-empty value sets it. The server trims/normalizes.
    group: form.group
  };
}
