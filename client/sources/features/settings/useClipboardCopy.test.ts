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

// The success path schedules its own 2s reset. These tests install a resolving
// clipboard stub and drive the timer with fake timers, covering both arms of the
// reset's `k === key ? null : k` guard.
describe("useClipboardCopy — successful copy", () => {
  const originalClipboard = Object.getOwnPropertyDescriptor(navigator, "clipboard");

  afterEach(() => {
    if (originalClipboard) {
      Object.defineProperty(navigator, "clipboard", originalClipboard);
    } else {
      Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });
    }
    vi.useRealTimers();
  });

  function stubClipboard(): void {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: vi.fn().mockResolvedValue(undefined) }
    });
  }

  it("reverts the Copied! label to Copy after the 2s timeout", async () => {
    vi.useFakeTimers();
    stubClipboard();

    const { result } = renderHook(() => useClipboardCopy());

    await act(async () => {
      await result.current.copyToClipboard("C:/x", "path");
    });
    expect(result.current.copyLabel("path")).toBe("Copied!");

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(result.current.copyLabel("path")).toBe("Copy");
  });

  it("does not let an earlier copy's reset clear a newer copy's label", async () => {
    vi.useFakeTimers();
    stubClipboard();

    const { result } = renderHook(() => useClipboardCopy());

    await act(async () => {
      await result.current.copyToClipboard("C:/x", "path");
    });
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    await act(async () => {
      await result.current.copyToClipboard("http://127.0.0.1:47831", "url");
    });
    expect(result.current.copyLabel("url")).toBe("Copied!");

    // The first copy's reset fires now, but the key has moved on to "url":
    // the stale timer must leave the newer label in place.
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(result.current.copyLabel("url")).toBe("Copied!");
    expect(result.current.copyLabel("path")).toBe("Copy");

    // The newer copy's own reset then clears it.
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(result.current.copyLabel("url")).toBe("Copy");
  });
});
