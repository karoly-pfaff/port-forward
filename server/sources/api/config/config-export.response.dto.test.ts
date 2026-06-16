import type { ExportedConfig } from "@portier/shared";
import { describe, expect, it } from "vitest";
import { toConfigExportResponseDto } from "./config-export.response.dto.js";

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

describe("toConfigExportResponseDto", () => {
  it("preserves the exported-config shape byte-for-byte without mutating the source", () => {
    const snapshot = structuredClone(CONFIG);

    const dto = toConfigExportResponseDto(CONFIG);

    expect(dto).toEqual(CONFIG);
    expect(dto).not.toBe(CONFIG); // fresh object
    expect(dto.rules).not.toBe(CONFIG.rules); // fresh array
    expect(dto.rules[0]).not.toBe(CONFIG.rules[0]); // fresh rule objects
    expect(CONFIG).toEqual(snapshot); // source untouched
  });

  it("maps an empty rule list to an empty array", () => {
    const empty: ExportedConfig = { version: "1", exportedAt: "2026-06-14T12:00:00.000Z", rules: [] };
    expect(toConfigExportResponseDto(empty)).toEqual(empty);
  });
});
