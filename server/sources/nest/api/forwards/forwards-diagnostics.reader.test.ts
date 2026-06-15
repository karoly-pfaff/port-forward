import type { ForwardRule } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { createDefaultDiagnosticReader } from "./forwards-diagnostics.reader.js";

const RULE: ForwardRule = {
  id: "d1",
  name: "Diag",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 48010,
  targetHost: "127.0.0.1",
  targetPort: 8080,
  enabled: false,
};

describe("createDefaultDiagnosticReader", () => {
  it("returns undefined from getRule for an unknown id (the route maps that to 404)", () => {
    const reader = createDefaultDiagnosticReader();
    expect(reader.getRule("missing")).toBeUndefined();
  });

  it("returns a stopped status from getStatus for an unknown id (never throws)", () => {
    const reader = createDefaultDiagnosticReader();
    expect(reader.getStatus("missing").running).toBe(false);
  });

  it("reads back a seeded rule and its status", async () => {
    // The default reader is a ForwardManager, which can also create rules.
    const reader = createDefaultDiagnosticReader() as unknown as {
      addRule(input: unknown): Promise<ForwardRule>;
      getRule(id: string): ForwardRule | undefined;
      getStatus(id: string): { running: boolean };
    };
    await reader.addRule(RULE);
    expect(reader.getRule("d1")?.id).toBe("d1");
    expect(reader.getStatus("d1").running).toBe(false);
  });
});
