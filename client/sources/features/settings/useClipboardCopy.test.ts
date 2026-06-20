import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useClipboardCopy } from "./useClipboardCopy.js";

// These tests focus on the no-clipboard branch (`if (!navigator.clipboard)
// throw`), which the SettingsView-level tests never reach because they always
// install a clipboard stub. We toggle navigator.clipboard directly and restore
// it after each test so other suites keep a real clipboard.
describe("useClipboardCopy — missing clipboard API", () => {
  const originalClipboard = Object.getOwnPropertyDescriptor(navigator, "clipboard");

  afterEach(() => {
    if (originalClipboard) {
      Object.defineProperty(navigator, "clipboard", originalClipboard);
    } else {
      Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });
    }
    vi.useRealTimers();
  });

  it("treats an absent clipboard API as a failure (label becomes Failed)", async () => {
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });

    const { result } = renderHook(() => useClipboardCopy());
    // Before any copy, the label is the neutral "Copy".
    expect(result.current.copyLabel("path")).toBe("Copy");

    await act(async () => {
      await result.current.copyToClipboard("C:/x", "path");
    });

    // The catch branch sets the `${key}:fail` key, surfaced as "Failed".
    expect(result.current.copyLabel("path")).toBe("Failed");
    // A different key is unaffected.
    expect(result.current.copyLabel("other")).toBe("Copy");
  });

  it("reverts the failed label to Copy after the 2s timeout", async () => {
    vi.useFakeTimers();
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });

    const { result } = renderHook(() => useClipboardCopy());

    await act(async () => {
      await result.current.copyToClipboard("C:/x", "path");
    });
    expect(result.current.copyLabel("path")).toBe("Failed");

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(result.current.copyLabel("path")).toBe("Copy");
  });
});
