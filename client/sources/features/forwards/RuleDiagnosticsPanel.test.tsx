import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { RuleDiagnosticsResult } from "@portier/shared";
import { RuleDiagnosticsPanel } from "./RuleDiagnosticsPanel.js";

const passResult: RuleDiagnosticsResult = {
  ruleId: "r1",
  ruleName: "Test Rule",
  protocol: "tcp",
  summary: { status: "pass", message: "All checks passed." },
  checks: [
    { id: "listen-host", label: "Listen Host", status: "pass", message: "Listening on 127.0.0.1 (local only)." },
    { id: "listen-bind", label: "Listen Bind", status: "pass", message: "Port 48001 can be bound." },
    { id: "target-host", label: "Target Host", status: "pass", message: "localhost resolves." },
    { id: "target-connect", label: "Target Connect", status: "pass", message: "Port 3000 is reachable." },
  ],
  diagnosedAt: new Date("2026-01-01T12:00:00Z").toISOString(),
};

const warnResult: RuleDiagnosticsResult = {
  ruleId: "r1",
  ruleName: "LAN Rule",
  protocol: "tcp",
  summary: { status: "warn", message: "1 warning." },
  checks: [
    { id: "lan-exposure", label: "LAN Exposure", status: "warn", message: "Listening on 0.0.0.0 exposes this rule on the local network." },
    { id: "target-connect", label: "Target Connect", status: "pass", message: "Port 3000 is reachable." },
  ],
  diagnosedAt: new Date("2026-01-01T12:00:00Z").toISOString(),
};

const failResult: RuleDiagnosticsResult = {
  ruleId: "r1",
  ruleName: "Broken Rule",
  protocol: "tcp",
  summary: { status: "fail", message: "1 check failed." },
  checks: [
    { id: "target-connect", label: "Target Connect", status: "fail", message: "Connection refused on port 9999." },
  ],
  diagnosedAt: new Date("2026-01-01T12:00:00Z").toISOString(),
};

const udpResult: RuleDiagnosticsResult = {
  ruleId: "r2",
  ruleName: "UDP Rule",
  protocol: "udp",
  summary: { status: "pass", message: "All checks passed." },
  checks: [
    { id: "target-connect", label: "Target Connect", status: "skip", message: "UDP reachability cannot be verified without a protocol-specific response." },
    { id: "udp-mode", label: "UDP Mode", status: "warn", message: "bidirectional-last-client: only the most-recently-seen client address receives replies." },
  ],
  diagnosedAt: new Date("2026-01-01T12:00:00Z").toISOString(),
};

describe("RuleDiagnosticsPanel", () => {
  it("shows loading message while pending", () => {
    render(<RuleDiagnosticsPanel loading={true} onClear={vi.fn()} />);
    expect(screen.getByText("Running diagnostics…")).toBeInTheDocument();
  });

  it("does not show result or error while loading", () => {
    render(
      <RuleDiagnosticsPanel
        loading={true}
        result={passResult}
        error="Something failed"
        onClear={vi.fn()}
      />
    );
    expect(screen.queryByText("All checks passed.")).not.toBeInTheDocument();
    expect(screen.queryByText("Something failed")).not.toBeInTheDocument();
  });

  it("shows error message when not loading and error is set", () => {
    render(
      <RuleDiagnosticsPanel
        loading={false}
        error="Rule not found."
        onClear={vi.fn()}
      />
    );
    expect(screen.getByText("Rule not found.")).toBeInTheDocument();
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("renders pass summary and checks", () => {
    render(<RuleDiagnosticsPanel loading={false} result={passResult} onClear={vi.fn()} />);
    expect(screen.getByText("All checks passed.")).toBeInTheDocument();
    expect(screen.getByText("Listen Host")).toBeInTheDocument();
    expect(screen.getByText("Listening on 127.0.0.1 (local only).")).toBeInTheDocument();
    expect(screen.getByText("Target Connect")).toBeInTheDocument();
  });

  it("renders warn summary with amber indication", () => {
    render(<RuleDiagnosticsPanel loading={false} result={warnResult} onClear={vi.fn()} />);
    expect(screen.getByText("1 warning.")).toBeInTheDocument();
    const summary = document.querySelector(".diag-summary--warn");
    expect(summary).toBeInTheDocument();
    expect(screen.getByText("LAN Exposure")).toBeInTheDocument();
    expect(screen.getByText("Listening on 0.0.0.0 exposes this rule on the local network.")).toBeInTheDocument();
  });

  it("renders fail summary with red indication", () => {
    render(<RuleDiagnosticsPanel loading={false} result={failResult} onClear={vi.fn()} />);
    expect(screen.getByText("1 check failed.")).toBeInTheDocument();
    const summary = document.querySelector(".diag-summary--fail");
    expect(summary).toBeInTheDocument();
    expect(screen.getByText("Connection refused on port 9999.")).toBeInTheDocument();
  });

  it("renders skip status for UDP target-connect", () => {
    render(<RuleDiagnosticsPanel loading={false} result={udpResult} onClear={vi.fn()} />);
    const skipCheck = document.querySelector(".diag-check--skip");
    expect(skipCheck).toBeInTheDocument();
    expect(screen.getByText("UDP reachability cannot be verified without a protocol-specific response.")).toBeInTheDocument();
  });

  it("renders warn status for bidirectional-last-client UDP mode", () => {
    render(<RuleDiagnosticsPanel loading={false} result={udpResult} onClear={vi.fn()} />);
    expect(screen.getByText(/bidirectional-last-client/)).toBeInTheDocument();
    expect(document.querySelector(".diag-check--warn")).toBeInTheDocument();
  });

  it("shows the Diagnostics title header", () => {
    render(<RuleDiagnosticsPanel loading={false} onClear={vi.fn()} />);
    expect(screen.getByText("Diagnostics")).toBeInTheDocument();
  });

  it("calls onClear when the close button is clicked", async () => {
    const user = userEvent.setup();
    const onClear = vi.fn();
    render(<RuleDiagnosticsPanel loading={false} result={passResult} onClear={onClear} />);
    await user.click(screen.getByRole("button", { name: "Close diagnostics" }));
    expect(onClear).toHaveBeenCalledOnce();
  });

  it("shows diagnosedAt time for a completed result", () => {
    render(<RuleDiagnosticsPanel loading={false} result={passResult} onClear={vi.fn()} />);
    // The time is formatted with toLocaleTimeString; just verify a time element appears
    expect(document.querySelector(".diag-summary-time")).toBeInTheDocument();
  });
});
