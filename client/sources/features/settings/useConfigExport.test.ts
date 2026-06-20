import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useConfigExport } from "./useConfigExport.js";
import * as portierApi from "../../api/portierApi.js";

vi.mock("../../api/portierApi.js", () => ({
  exportConfig: vi.fn()
}));

// Exercises the export hook directly so we can cover the catch branch (line 45),
// including both the `error instanceof Error` message and the generic fallback.
describe("useConfigExport — failure handling", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn(() => "blob:test") });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("sets exportError to the Error message when exportConfig rejects with an Error", async () => {
    vi.mocked(portierApi.exportConfig).mockRejectedValue(new Error("disk full"));

    const { result } = renderHook(() => useConfigExport());
    await act(async () => {
      await result.current.handleExport();
    });

    expect(result.current.exportError).toBe("disk full");
    expect(result.current.exportSuccess).toBe(false);
    // The finally block always clears the in-progress flag.
    expect(result.current.exporting).toBe(false);
  });

  it("falls back to a generic message when exportConfig rejects with a non-Error", async () => {
    vi.mocked(portierApi.exportConfig).mockRejectedValue("boom");

    const { result } = renderHook(() => useConfigExport());
    await act(async () => {
      await result.current.handleExport();
    });

    expect(result.current.exportError).toBe("Export failed.");
  });

  it("clears a previous error and reports success on a later successful export", async () => {
    vi.mocked(portierApi.exportConfig).mockRejectedValueOnce(new Error("temporary"));
    vi.mocked(portierApi.exportConfig).mockResolvedValue({
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: []
    });

    const { result } = renderHook(() => useConfigExport());
    await act(async () => {
      await result.current.handleExport();
    });
    expect(result.current.exportError).toBe("temporary");

    await act(async () => {
      await result.current.handleExport();
    });
    await waitFor(() => {
      expect(result.current.exportSuccess).toBe(true);
    });
    expect(result.current.exportError).toBeNull();
  });
});
