import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DiagnosticsExportSection } from "./DiagnosticsExportSection.js";
import * as diagnosticsExport from "./diagnosticsExport.js";
import type { DiagnosticsBundle } from "./diagnosticsExport.js";

// Mock the export helpers so each test can control whether the bundle build
// succeeds, fails, or carries errors — driving the section's own state branches
// directly instead of through the whole SettingsView tree.
vi.mock("./diagnosticsExport.js", () => ({
  buildDiagnosticsBundle: vi.fn(),
  buildDiagnosticsFilename: vi.fn(() => "portier-diagnostics-test.json"),
  downloadJson: vi.fn()
}));

function cleanBundle(): DiagnosticsBundle {
  return {
    schemaVersion: "1",
    exportedAt: "2026-01-01T00:00:00.000Z",
    app: { name: "Portier", version: "1.18.0" },
    runtime: null,
    rules: [],
    statuses: [],
    diagnostics: {},
    activity: { included: true, events: [], note: "" },
    metadata: { managementUrl: "127.0.0.1:47831", source: "client", generatedBy: "settings" }
    // no `errors` field → full success path
  };
}

describe("DiagnosticsExportSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // A non-Error rejection drives the `: "Export failed."` fallback (line 49) and
  // renders the error region (line 76) with role="alert".
  it("shows the generic fallback message when the bundle build rejects with a non-Error", async () => {
    vi.mocked(diagnosticsExport.buildDiagnosticsBundle).mockRejectedValue("kaboom");

    render(<DiagnosticsExportSection />);
    await userEvent.click(screen.getByRole("button", { name: /Download Diagnostics/ }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Export failed.");
    expect(diagnosticsExport.downloadJson).not.toHaveBeenCalled();
  });

  // An Error rejection surfaces its own message in the error region.
  it("shows the error message when the bundle build rejects with an Error", async () => {
    vi.mocked(diagnosticsExport.buildDiagnosticsBundle).mockRejectedValue(new Error("disk full"));

    render(<DiagnosticsExportSection />);
    await userEvent.click(screen.getByRole("button", { name: /Download Diagnostics/ }));

    expect(await screen.findByRole("alert")).toHaveTextContent("disk full");
  });

  // Two consecutive successful exports: the second success finds a non-null
  // success timer ref and clears it before scheduling a new one (lines 43-46),
  // covering the previously-unhit `clearTimeout` arm.
  it("clears the pending success timer on a second successful export", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const clearTimeoutSpy = vi.spyOn(globalThis, "clearTimeout");
      vi.mocked(diagnosticsExport.buildDiagnosticsBundle).mockResolvedValue(cleanBundle());

      render(<DiagnosticsExportSection />);
      const btn = screen.getByRole("button", { name: /Download Diagnostics/ });

      // First export: schedules the clear-success timer.
      await userEvent.click(btn);
      await waitFor(() =>
        expect(screen.getByRole("status")).toHaveTextContent("Diagnostics exported successfully.")
      );
      const clearCallsAfterFirst = clearTimeoutSpy.mock.calls.length;

      // Second export while the first timer is still pending: must clear it.
      await userEvent.click(btn);
      await waitFor(() =>
        expect(screen.getByRole("status")).toHaveTextContent("Diagnostics exported successfully.")
      );

      expect(clearTimeoutSpy.mock.calls.length).toBeGreaterThan(clearCallsAfterFirst);
      expect(diagnosticsExport.downloadJson).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
    }
  });

  // A bundle carrying a non-empty `errors` array shows the partial warning
  // instead of the success confirmation.
  it("shows the partial-data warning when the bundle reports errors", async () => {
    vi.mocked(diagnosticsExport.buildDiagnosticsBundle).mockResolvedValue({
      ...cleanBundle(),
      errors: [{ source: "runtime", message: "unreachable" }]
    });

    render(<DiagnosticsExportSection />);
    await userEvent.click(screen.getByRole("button", { name: /Download Diagnostics/ }));

    await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent(/partial data/));
    expect(screen.queryByText("Diagnostics exported successfully.")).not.toBeInTheDocument();
  });
});
