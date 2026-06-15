import type { ForwardRuleResponse, ImportResult } from "@portier/shared";
import { describe, expect, it } from "vitest";
import {
  toConfigImportErrorResponseDto,
  toConfigImportResponseDto,
} from "./config-import.response.dto.js";

const RULE: ForwardRuleResponse = {
  id: "r1",
  name: "Web",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 48010,
  targetHost: "127.0.0.1",
  targetPort: 8080,
  enabled: false,
  advisories: [{ code: "COMMON_PORT", severity: "warning", message: "x" }],
};

describe("toConfigImportResponseDto (200)", () => {
  it("preserves the success body byte-for-byte without mutating the source, returning a fresh deep copy", () => {
    const body = { result: { imported: 1, skipped: 0, errors: [] } as ImportResult, rules: [RULE] };
    const snapshot = structuredClone(body);

    const dto = toConfigImportResponseDto(body);

    expect(dto).toEqual(body);
    expect(body).toEqual(snapshot); // source untouched
    expect(dto).not.toBe(body);
    expect(dto.result).not.toBe(body.result);
    expect(dto.rules).not.toBe(body.rules);
    expect(dto.rules[0]).not.toBe(body.rules[0]);
  });
});

describe("toConfigImportErrorResponseDto (422)", () => {
  it("preserves the error body (errors + result) byte-for-byte without mutating the source", () => {
    const body = { errors: ["bad"], result: { imported: 0, skipped: 1, errors: ["bad"] } as ImportResult };
    const snapshot = structuredClone(body);

    const dto = toConfigImportErrorResponseDto(body);

    expect(dto).toEqual(body);
    expect(body).toEqual(snapshot); // source untouched
    expect(dto).not.toBe(body);
    expect(dto.errors).not.toBe(body.errors);
    expect(dto.result).not.toBe(body.result);
  });
});
