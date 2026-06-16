import type { ExportedConfig } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { ConfigExportController } from "./config-export.controller.js";
import type { ConfigExportService } from "./config-export.service.js";

const CONFIG: ExportedConfig = {
  version: "1",
  exportedAt: "2026-06-14T12:00:00.000Z",
  rules: [
    {
      id: "r1",
      name: "Web",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48010,
      targetHost: "127.0.0.1",
      targetPort: 8080,
      enabled: false,
    },
  ],
};

describe("ConfigExportController.export", () => {
  it("delegates to the service and maps the result to the response DTO", () => {
    const controller = new ConfigExportController({
      export: () => CONFIG,
    } as unknown as ConfigExportService);

    const result = controller.export();

    expect(result).toEqual(CONFIG); // byte-for-byte
    expect(result).not.toBe(CONFIG); // mapped copy
    expect(result.rules[0]).not.toBe(CONFIG.rules[0]); // rules freshly copied
  });
});
