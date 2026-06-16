import type { ExportedConfig, ForwardRule, ImportMode, ImportResult } from "@portier/shared";
import { describe, expect, it, vi } from "vitest";
import { ApiBadRequestException } from "../../common/api-errors.js";
import type { ClockReader } from "../../common/clock.reader.js";
import { ConfigApplyService } from "./config-apply.service.js";
import type { ConfigApplier } from "./config-apply.writer.js";

const FIXED = new Date("2026-06-15T08:30:00.000Z");
const APPLIED_AT = FIXED.toISOString();

function rule(over: Partial<ForwardRule> = {}): ForwardRule {
  return { id: "a", name: "Web", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48010, targetHost: "127.0.0.1", targetPort: 8080, enabled: false, ...over };
}

const clock: ClockReader = { now: () => FIXED };

function service(applier: ConfigApplier): ConfigApplyService {
  return new ConfigApplyService(applier, clock);
}

function applier(over: Partial<ConfigApplier> = {}): ConfigApplier {
  return {
    listRules: over.listRules ?? (() => []),
    importConfig: over.importConfig ?? (async () => ({ imported: 0, skipped: 0, errors: [] })),
  };
}

describe("ConfigApplyService.apply — request validation", () => {
  it.each([
    ["a missing desired key", {}],
    ["a non-object body", "nope"],
    ["an undefined body", undefined],
    ["a null body", null],
  ])("throws 400 'desired is required.' for %s", async (_label, body) => {
    await expect(service(applier()).apply(body)).rejects.toBeInstanceOf(ApiBadRequestException);
    try {
      await service(applier()).apply(body);
    } catch (error) {
      expect((error as ApiBadRequestException).getResponse()).toEqual({ errors: ["desired is required."] });
    }
  });
});

describe("ConfigApplyService.apply — outcomes", () => {
  it("returns ok:false (200) with zeroed applied counts when the plan has errors — no import", async () => {
    const importConfig = vi.fn();
    const out = await service(applier({ importConfig })).apply({ desired: [rule({ listenPort: 70000 })] });
    expect(out.ok).toBe(false);
    expect(out.dryRun).toBe(false);
    expect(out.appliedAt).toBe(APPLIED_AT);
    expect(out.plan.summary.hasErrors).toBe(true);
    expect(out.applied).toEqual({ add: 0, update: 0, remove: 0, unchanged: 0 });
    expect(importConfig).not.toHaveBeenCalled();
  });

  it("treats desired:null as a plan error (ok:false 200), NOT a 400", async () => {
    const out = await service(applier()).apply({ desired: null });
    expect(out.ok).toBe(false);
    expect(out.plan.summary.hasErrors).toBe(true);
  });

  it("reflects dryRun in the ok:false plan-error response", async () => {
    const out = await service(applier()).apply({ desired: [rule({ listenPort: 70000 })], dryRun: true });
    expect(out.ok).toBe(false);
    expect(out.dryRun).toBe(true);
  });

  it("returns ok:true dryRun:true with applied counts and does NOT import on a dry-run", async () => {
    const importConfig = vi.fn();
    const out = await service(applier({ importConfig })).apply({ desired: [rule()], dryRun: true });
    expect(out.ok).toBe(true);
    expect(out.dryRun).toBe(true);
    expect(out.appliedAt).toBe(APPLIED_AT);
    expect(out.applied).toEqual({ add: 1, update: 0, remove: 0, unchanged: 0 });
    expect(importConfig).not.toHaveBeenCalled();
  });

  it("allows a DESTRUCTIVE dry-run (ok:true) without yes (gate is after the dry-run check)", async () => {
    const importConfig = vi.fn();
    // current has a rule, desired removes it → destructive remove.
    const out = await service(applier({ listRules: () => [rule()], importConfig })).apply({ desired: [], dryRun: true });
    expect(out.ok).toBe(true);
    expect(out.dryRun).toBe(true);
    expect(out.plan.summary.destructive).toBe(1);
    expect(importConfig).not.toHaveBeenCalled();
  });

  it("throws 400 when destructive operations are present and yes is not true (non-dry-run)", async () => {
    const importConfig = vi.fn();
    try {
      await service(applier({ listRules: () => [rule()], importConfig })).apply({ desired: [] });
      throw new Error("expected throw");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiBadRequestException);
      expect((error as ApiBadRequestException).getResponse()).toEqual({
        errors: ["Apply requires yes: true when destructive operations are present."],
      });
    }
    expect(importConfig).not.toHaveBeenCalled();
  });

  it("applies a destructive change with yes:true → replace import → ok:true", async () => {
    let receivedMode: ImportMode | undefined;
    let receivedConfig: ExportedConfig | undefined;
    const importConfig = async (config: ExportedConfig, mode: ImportMode): Promise<ImportResult> => {
      receivedConfig = config;
      receivedMode = mode;
      return { imported: 0, skipped: 0, errors: [] };
    };
    const out = await service(applier({ listRules: () => [rule()], importConfig })).apply({ desired: [], yes: true });
    expect(out.ok).toBe(true);
    expect(out.dryRun).toBe(false);
    expect(receivedMode).toBe("replace");
    expect(receivedConfig).toEqual({ version: "1", exportedAt: APPLIED_AT, rules: [] });
    expect(out.applied).toEqual({ add: 0, update: 0, remove: 1, unchanged: 0 });
  });

  it("applies a non-destructive add (drift) → replace import → ok:true with the new rule list", async () => {
    let received: ExportedConfig | undefined;
    const out = await service(
      applier({ importConfig: async (config) => { received = config; return { imported: 1, skipped: 0, errors: [] }; } })
    ).apply({ desired: [rule()] });
    expect(out.ok).toBe(true);
    expect(out.applied).toEqual({ add: 1, update: 0, remove: 0, unchanged: 0 });
    expect(received?.rules.map((r) => r.id)).toEqual(["a"]);
  });

  it("returns ok:true WITHOUT importing when the plan has no drift (current === desired)", async () => {
    const importConfig = vi.fn();
    const out = await service(applier({ listRules: () => [rule()], importConfig })).apply({ desired: [rule()] });
    expect(out.ok).toBe(true);
    expect(out.plan.summary.hasDrift).toBe(false);
    expect(out.applied).toEqual({ add: 0, update: 0, remove: 0, unchanged: 1 });
    expect(importConfig).not.toHaveBeenCalled();
  });

  it("surfaces import errors as ok:false (200) via plan.errors with zeroed counts (belt-and-suspenders guard)", async () => {
    const out = await service(
      applier({ importConfig: async () => ({ imported: 0, skipped: 0, errors: ["forced import error"] }) })
    ).apply({ desired: [rule()] });
    expect(out.ok).toBe(false);
    expect(out.dryRun).toBe(false);
    expect(out.plan.summary.hasErrors).toBe(true);
    expect(out.plan.errors.some((e) => e.code === "IMPORT_ERROR" && e.message === "forced import error")).toBe(true);
    expect(out.applied).toEqual({ add: 0, update: 0, remove: 0, unchanged: 0 });
  });

  it("re-throws an unexpected importConfig error (persist failure) → generic 500", async () => {
    const persistError = new Error("disk full");
    await expect(
      service(applier({ importConfig: async () => { throw persistError; } })).apply({ desired: [rule()] })
    ).rejects.toBe(persistError);
  });

  it("stamps appliedAt from the injected clock", async () => {
    const out = await service(applier()).apply({ desired: [rule()] });
    expect(out.appliedAt).toBe(APPLIED_AT);
  });
});
