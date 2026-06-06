import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { ActivityEvent, ForwardRuleResponse, ForwardStatus } from "@portier/shared";
import { DashboardView } from "./DashboardView.js";

const tcpRule: ForwardRuleResponse = {
  id: "r1",
  name: "TCP Service",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 48001,
  targetHost: "192.168.1.5",
  targetPort: 3000,
  enabled: true,
  advisories: []
};

const udpRule: ForwardRuleResponse = {
  id: "r2",
  name: "UDP Metrics",
  protocol: "udp",
  udpMode: "one-way",
  listenHost: "127.0.0.1",
  listenPort: 48002,
  targetHost: "127.0.0.1",
  targetPort: 9001,
  enabled: false,
  advisories: []
};

const runningStatus: ForwardStatus = {
  ruleId: "r1",
  running: true,
  bytesIn: 2048,
  bytesOut: 1024
};

const stoppedStatus: ForwardStatus = {
  ruleId: "r2",
  running: false,
  bytesIn: 0,
  bytesOut: 0
};

const activityEvent: ActivityEvent = {
  id: "e1",
  type: "rule.started",
  severity: "success",
  timestamp: new Date().toISOString(),
  message: "Rule started."
};

function makeMap(...statuses: ForwardStatus[]): Map<string, ForwardStatus> {
  return new Map(statuses.map((s) => [s.ruleId, s]));
}

describe("DashboardView", () => {
  it("shows total, running, stopped, and error stat cards", () => {
    render(
      <DashboardView
        rules={[tcpRule, udpRule]}
        statusMap={makeMap(runningStatus, stoppedStatus)}
        recentActivity={[]}
        onGoToActivity={vi.fn()}
        onGoToRules={vi.fn()}
      />
    );
    expect(screen.getByText("Total Rules")).toBeInTheDocument();
    expect(screen.getByText("Running")).toBeInTheDocument();
    expect(screen.getByText("Stopped")).toBeInTheDocument();
    expect(screen.getByText("Error")).toBeInTheDocument();
  });

  it("shows TCP and UDP breakdown in the Total Rules card", () => {
    render(
      <DashboardView
        rules={[tcpRule, udpRule]}
        statusMap={makeMap(runningStatus, stoppedStatus)}
        recentActivity={[]}
        onGoToActivity={vi.fn()}
        onGoToRules={vi.fn()}
      />
    );
    expect(screen.getByText(/1 TCP/)).toBeInTheDocument();
    expect(screen.getByText(/1 UDP/)).toBeInTheDocument();
  });

  it("shows 'No traffic recorded yet' when no rule has traffic", () => {
    render(
      <DashboardView
        rules={[tcpRule]}
        statusMap={makeMap({ ...runningStatus, bytesIn: 0, bytesOut: 0 })}
        recentActivity={[]}
        onGoToActivity={vi.fn()}
        onGoToRules={vi.fn()}
      />
    );
    expect(screen.getByText(/No traffic recorded yet/)).toBeInTheDocument();
  });

  it("shows rule name in top active list when traffic exists", () => {
    render(
      <DashboardView
        rules={[tcpRule]}
        statusMap={makeMap(runningStatus)}
        recentActivity={[]}
        onGoToActivity={vi.fn()}
        onGoToRules={vi.fn()}
      />
    );
    expect(screen.getByText("TCP Service")).toBeInTheDocument();
  });

  it("shows 'No activity yet' when recentActivity is empty", () => {
    render(
      <DashboardView
        rules={[]}
        statusMap={new Map()}
        recentActivity={[]}
        onGoToActivity={vi.fn()}
        onGoToRules={vi.fn()}
      />
    );
    expect(screen.getByText(/No activity yet/)).toBeInTheDocument();
  });

  it("renders recent activity event messages", () => {
    render(
      <DashboardView
        rules={[]}
        statusMap={new Map()}
        recentActivity={[activityEvent]}
        onGoToActivity={vi.fn()}
        onGoToRules={vi.fn()}
      />
    );
    expect(screen.getByText("Rule started.")).toBeInTheDocument();
  });

  it("calls onGoToRules when top rules View All is clicked", async () => {
    const user = userEvent.setup();
    const onGoToRules = vi.fn();
    render(
      <DashboardView
        rules={[tcpRule]}
        statusMap={makeMap(runningStatus)}
        recentActivity={[]}
        onGoToActivity={vi.fn()}
        onGoToRules={onGoToRules}
      />
    );
    const viewAllBtns = screen.getAllByRole("button", { name: "View All" });
    await user.click(viewAllBtns[0]);
    expect(onGoToRules).toHaveBeenCalled();
  });

  it("calls onGoToActivity when activity View All is clicked", async () => {
    const user = userEvent.setup();
    const onGoToActivity = vi.fn();
    render(
      <DashboardView
        rules={[]}
        statusMap={new Map()}
        recentActivity={[activityEvent]}
        onGoToActivity={onGoToActivity}
        onGoToRules={vi.fn()}
      />
    );
    const viewAllBtns = screen.getAllByRole("button", { name: "View All" });
    await user.click(viewAllBtns[1]);
    expect(onGoToActivity).toHaveBeenCalled();
  });
});
