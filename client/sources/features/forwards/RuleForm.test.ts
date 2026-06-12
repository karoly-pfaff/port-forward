import { describe, expect, it } from "vitest";
import type { ForwardRule } from "@portier/shared";
import { duplicateName, ruleToDuplicateForm, ruleToForm } from "./RuleForm.js";

const sourceRule: ForwardRule = {
  id: "r1",
  name: "Web API",
  protocol: "udp",
  udpMode: "bidirectional-multi-client",
  listenHost: "0.0.0.0",
  listenPort: 48010,
  targetHost: "127.0.0.1",
  targetPort: 9000,
  enabled: true,
  group: "backend"
};

describe("duplicateName", () => {
  it("appends ' copy' to a trimmed name", () => {
    expect(duplicateName("Web API")).toBe("Web API copy");
  });

  it("trims surrounding whitespace before appending", () => {
    expect(duplicateName("  Web API  ")).toBe("Web API copy");
  });

  it("falls back to 'copy' for an empty/whitespace name", () => {
    expect(duplicateName("")).toBe("copy");
    expect(duplicateName("   ")).toBe("copy");
  });
});

describe("ruleToDuplicateForm", () => {
  it("copies the editable definition fields from the source rule", () => {
    const form = ruleToDuplicateForm(sourceRule);
    expect(form.protocol).toBe("udp");
    expect(form.udpMode).toBe("bidirectional-multi-client");
    expect(form.listenHost).toBe("0.0.0.0");
    expect(form.listenPort).toBe("48010");
    expect(form.targetHost).toBe("127.0.0.1");
    expect(form.targetPort).toBe("9000");
  });

  it("copies the group metadata", () => {
    expect(ruleToDuplicateForm(sourceRule).group).toBe("backend");
  });

  it("does not carry over the source rule id (saves through create path)", () => {
    expect(ruleToDuplicateForm(sourceRule).id).toBeUndefined();
  });

  it("adjusts the name to indicate a copy", () => {
    expect(ruleToDuplicateForm(sourceRule).name).toBe("Web API copy");
  });

  it("forces autostart off so a duplicate cannot auto-start on save", () => {
    expect(sourceRule.enabled).toBe(true);
    expect(ruleToDuplicateForm(sourceRule).enabled).toBe(false);
  });

  it("does not mutate the source rule", () => {
    const snapshot = JSON.stringify(sourceRule);
    ruleToDuplicateForm(sourceRule);
    expect(JSON.stringify(sourceRule)).toBe(snapshot);
  });

  it("normalizes an absent group to an empty string (same as ruleToForm)", () => {
    const ungrouped: ForwardRule = { ...sourceRule, group: undefined };
    expect(ruleToDuplicateForm(ungrouped).group).toBe("");
    // sanity: matches the create/edit form normalization
    expect(ruleToForm(ungrouped).group).toBe("");
  });
});
