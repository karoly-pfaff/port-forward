import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { ForwardStatus } from "@portier/shared";
import { ForwardStatusBadge } from "./ForwardStatusBadge.js";

const stoppedStatus: ForwardStatus = {
  ruleId: "r1",
  running: false,
  bytesIn: 0,
  bytesOut: 0
};

const runningStatus: ForwardStatus = {
  ruleId: "r1",
  running: true,
  bytesIn: 1024,
  bytesOut: 512
};

describe("ForwardStatusBadge", () => {
  it("shows Stopped when not running", () => {
    render(<ForwardStatusBadge status={stoppedStatus} />);
    expect(screen.getByText("Stopped")).toBeInTheDocument();
  });

  it("shows Running when running", () => {
    render(<ForwardStatusBadge status={runningStatus} />);
    expect(screen.getByText("Running")).toBeInTheDocument();
  });

  it("shows Stopped when status is undefined", () => {
    render(<ForwardStatusBadge status={undefined} />);
    expect(screen.getByText("Stopped")).toBeInTheDocument();
  });

  it("shows Error badge when stopped with a lastError", () => {
    render(
      <ForwardStatusBadge
        status={{ ...stoppedStatus, lastError: "bind EADDRINUSE" }}
      />
    );
    expect(screen.getByText("Error")).toBeInTheDocument();
  });

  it("shows Running (not Error) when running even if lastError is set", () => {
    render(
      <ForwardStatusBadge
        status={{ ...runningStatus, lastError: "previous error" }}
      />
    );
    expect(screen.getByText("Running")).toBeInTheDocument();
    expect(screen.queryByText("Error")).not.toBeInTheDocument();
  });

  it("shows warning icon with error message in tooltip when error is present", () => {
    render(
      <ForwardStatusBadge
        status={{ ...stoppedStatus, lastError: "bind EADDRINUSE" }}
      />
    );
    expect(screen.getByLabelText(/Error: bind EADDRINUSE/)).toBeInTheDocument();
  });

  it("does not show warning icon when no error is present", () => {
    render(<ForwardStatusBadge status={stoppedStatus} />);
    expect(screen.queryByLabelText(/Error:/)).not.toBeInTheDocument();
  });

  it("status text is readable without relying only on color", () => {
    const { container } = render(<ForwardStatusBadge status={runningStatus} />);
    expect(container.textContent).toContain("Running");
  });
});
