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
