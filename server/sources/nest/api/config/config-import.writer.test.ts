import type { ExportedConfig } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { createDefaultConfigImporter } from "./config-import.writer.js";

const CONFIG: ExportedConfig = {
  version: "1",
  exportedAt: "2026-06-15T00:00:00.000Z",
  rules: [
    { id: "g1", name: "Web", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48010, targetHost: "127.0.0.1", targetPort: 8080, enabled: false },
  ],
};

describe("createDefaultConfigImporter", () => {
  it("creates an isolated in-memory importer that replaces rules and lists them back", async () => {
    const importer = createDefaultConfigImporter();
    const result = await importer.importConfig(CONFIG, "replace");
    expect(result).toEqual({ imported: 1, skipped: 0, errors: [] });
    expect(importer.listRules().map((r) => r.id)).toEqual(["g1"]);
  });

  it("returns a fresh, independent importer each call", async () => {
    const a = createDefaultConfigImporter();
    const b = createDefaultConfigImporter();
    await a.importConfig(CONFIG, "replace");
    // b has not seen a's import.
    expect(b.listRules()).toEqual([]);
  });
});
