import type { ExportedConfig, ForwardRule, ImportMode, ImportResult } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ApiBadRequestException } from "../common/api-errors.js";
import { ConfigImportService } from "./config-import.service.js";
import type { ConfigImporter } from "./config-import.writer.js";

function rule(id: string): ForwardRule {
  return { id, name: id, protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48010, targetHost: "127.0.0.1", targetPort: 8080, enabled: false };
}

function importer(overrides: Partial<ConfigImporter> = {}): ConfigImporter {
  return {
    importConfig: overrides.importConfig ?? (async () => ({ imported: 0, skipped: 0, errors: [] })),
    listRules: overrides.listRules ?? (() => []),
  };
}

const VALID_BODY = { mode: "replace", config: { version: "1", rules: [] } };

describe("ConfigImportService.import — request validation (mirrors Express's short-circuit order)", () => {
  it.each([
    ["a missing mode", { config: { version: "1", rules: [] } }],
    ["an unknown mode", { mode: "wipe", config: { version: "1", rules: [] } }],
    ["a non-object body", "nope"],
    ["an undefined body", undefined],
  ])("throws 400 'mode must be replace or merge.' for %s", async (_label, body) => {
    await expect(new ConfigImportService(importer()).import(body)).rejects.toBeInstanceOf(ApiBadRequestException);
    try {
      await new ConfigImportService(importer()).import(body);
    } catch (error) {
      expect((error as ApiBadRequestException).getResponse()).toEqual({ errors: ["mode must be replace or merge."] });
    }
  });

  it.each([
    ["a missing config", { mode: "replace" }],
    ["a config without version 1", { mode: "merge", config: { version: "2", rules: [] } }],
    ["a config whose rules is not an array", { mode: "replace", config: { version: "1", rules: "x" } }],
  ])("throws 400 'config must be ...' for %s (only after a valid mode — short-circuit)", async (_label, body) => {
    await expect(new ConfigImportService(importer()).import(body)).rejects.toBeInstanceOf(ApiBadRequestException);
    try {
      await new ConfigImportService(importer()).import(body);
    } catch (error) {
      expect((error as ApiBadRequestException).getResponse()).toEqual({
        errors: ["config must be a valid Portier config object with version 1 and a rules array."],
      });
    }
  });

  it("checks mode BEFORE config (an invalid mode AND invalid config → the mode error)", async () => {
    try {
      await new ConfigImportService(importer()).import({ mode: "wipe", config: null });
    } catch (error) {
      expect((error as ApiBadRequestException).getResponse()).toEqual({ errors: ["mode must be replace or merge."] });
    }
  });
});

describe("ConfigImportService.import — outcomes", () => {
  it("returns 200 with the result + the decorated rule list on success", async () => {
    let receivedMode: ImportMode | undefined;
    let receivedConfig: ExportedConfig | undefined;
    const result: ImportResult = { imported: 1, skipped: 0, errors: [] };
    const outcome = await new ConfigImportService(
      importer({
        importConfig: async (config, mode) => {
          receivedConfig = config;
          receivedMode = mode;
          return result;
        },
        listRules: () => [rule("r1")],
      })
    ).import({ mode: "replace", config: { version: "1", rules: [rule("r1")] } });

    expect(receivedMode).toBe("replace");
    expect(receivedConfig?.version).toBe("1");
    expect(outcome.status).toBe(200);
    expect(outcome.body).toEqual({
      result,
      rules: [{ ...rule("r1"), advisories: expect.any(Array) }],
    });
  });

  it("returns 422 with the errors + result when the import reports errors (no mutation)", async () => {
    const result: ImportResult = { imported: 0, skipped: 0, errors: ["Rule \"X\": bad"] };
    const outcome = await new ConfigImportService(
      importer({ importConfig: async () => result })
    ).import(VALID_BODY);

    expect(outcome.status).toBe(422);
    expect(outcome.body).toEqual({ errors: ["Rule \"X\": bad"], result });
  });

  it("re-throws an unexpected importConfig error (e.g. persist failure) → generic 500", async () => {
    const persistError = new Error("disk full");
    await expect(
      new ConfigImportService(importer({ importConfig: async () => { throw persistError; } })).import(VALID_BODY)
    ).rejects.toBe(persistError);
  });
});
