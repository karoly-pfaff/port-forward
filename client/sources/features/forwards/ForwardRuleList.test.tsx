import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ForwardRuleResponse, ForwardStatus } from "@portier/shared";
import { ForwardRuleList } from "./ForwardRuleList.js";

const tcpRule: ForwardRuleResponse = {
  id: "r1",
  name: "Test Rule",
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
  listenHost: "0.0.0.0",
  listenPort: 48002,
  targetHost: "127.0.0.1",
  targetPort: 9001,
  enabled: false,
  advisories: []
};

const stoppedStatus: ForwardStatus = {
  ruleId: "r1",
  running: false,
  bytesIn: 0,
  bytesOut: 0
};

const runningStatus: ForwardStatus = {
  ruleId: "r1",
  running: true,
  bytesIn: 2048,
  bytesOut: 512,
  activeConnections: 3
};

const udpStatus: ForwardStatus = {
  ruleId: "r2",
  running: true,
  bytesIn: 0,
  bytesOut: 0,
  packetsIn: 10,
  packetsOut: 8
};

function makeMap<T extends { ruleId: string }>(...statuses: T[]): Map<string, T> {
  return new Map(statuses.map((s) => [s.ruleId, s]));
}

const noop = vi.fn();

function renderList(
  props: Partial<Parameters<typeof ForwardRuleList>[0]> & {
    rules: ForwardRuleResponse[];
    statusMap: Map<string, ForwardStatus>;
    busyRuleIds: Set<string>;
    loading: boolean;
  }
) {
  return render(
    <ForwardRuleList
      editingRuleId={null}
      onEdit={noop}
      onStart={noop}
      onStop={noop}
      onDelete={noop}
      onAddRule={noop}
      onRefresh={noop}
      autoRefresh={false}
      autoRefreshInterval={5}
      onToggleAutoRefresh={noop}
      onChangeAutoRefreshInterval={noop}
      {...props}
    />
  );
}

describe("ForwardRuleList", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows empty state when there are no rules", () => {
    renderList({ rules: [], statusMap: new Map(), busyRuleIds: new Set(), loading: false });
    expect(screen.getByText(/No forwarding rules yet/)).toBeInTheDocument();
  });

  it("shows loading message when loading with no rules", () => {
    renderList({ rules: [], statusMap: new Map(), busyRuleIds: new Set(), loading: true });
    expect(screen.getByText(/Loading rules/)).toBeInTheDocument();
  });

  it("renders rule name, protocol badge, listen and target endpoints", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false
    });
    expect(screen.getByText("Test Rule")).toBeInTheDocument();
    expect(screen.getByText("TCP")).toBeInTheDocument();
    expect(screen.getByText(":48001")).toBeInTheDocument();
    expect(screen.getByText(":3000")).toBeInTheDocument();
  });

  it("shows configured and running counts in section header", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(runningStatus),
      busyRuleIds: new Set(),
      loading: false
    });
    expect(screen.getByText(/1 configured/)).toBeInTheDocument();
    expect(screen.getByText("1 running")).toBeInTheDocument();
  });

  it("shows error count in header when a rule has lastError", () => {
    const errStatus: ForwardStatus = { ...stoppedStatus, lastError: "ECONNREFUSED" };
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(errStatus),
      busyRuleIds: new Set(),
      loading: false
    });
    expect(screen.getByText(/1 error/)).toBeInTheDocument();
  });

  it("shows Autostart Yes for an enabled rule", () => {
    renderList({
      rules: [tcpRule], // enabled: true
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false
    });
    expect(screen.getByText("Yes")).toBeInTheDocument();
  });

  it("shows Autostart No for a disabled rule", () => {
    renderList({
      rules: [udpRule], // enabled: false
      statusMap: makeMap(udpStatus),
      busyRuleIds: new Set(),
      loading: false
    });
    expect(screen.getByText("No")).toBeInTheDocument();
  });

  it("displays TCP active connections stat", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(runningStatus),
      busyRuleIds: new Set(),
      loading: false
    });
    expect(screen.getByText(/3 active/)).toBeInTheDocument();
  });

  it("displays traffic bytes for UDP rule", () => {
    renderList({
      rules: [udpRule],
      statusMap: makeMap(udpStatus),
      busyRuleIds: new Set(),
      loading: false
    });
    expect(screen.getByText("0 B / 0 B")).toBeInTheDocument();
    expect(screen.getByText("UDP")).toBeInTheDocument();
  });

  it("shows warning icon for an errored rule", () => {
    const errStatus: ForwardStatus = { ...stoppedStatus, lastError: "bind EADDRINUSE" };
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(errStatus),
      busyRuleIds: new Set(),
      loading: false
    });
    expect(screen.getByLabelText(/Error: bind EADDRINUSE/)).toBeInTheDocument();
  });

  it("requires a confirmation click before calling onDelete", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      onDelete
    });

    await user.click(screen.getByRole("button", { name: "Delete" }));
    expect(onDelete).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Confirm" }));
    expect(onDelete).toHaveBeenCalledWith(expect.objectContaining({ id: "r1" }));
  });

  it("cancels delete confirmation without calling onDelete", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      onDelete
    });

    await user.click(screen.getByRole("button", { name: "Delete" }));
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onDelete).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Delete" })).toBeInTheDocument();
  });

  it("calls onStart when Start is clicked for a stopped rule", async () => {
    const user = userEvent.setup();
    const onStart = vi.fn();
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      onStart
    });
    await user.click(screen.getByRole("button", { name: "Start" }));
    expect(onStart).toHaveBeenCalledWith(expect.objectContaining({ id: "r1" }));
  });

  it("calls onStop when Stop is clicked for a running rule", async () => {
    const user = userEvent.setup();
    const onStop = vi.fn();
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(runningStatus),
      busyRuleIds: new Set(),
      loading: false,
      onStop
    });
    await user.click(screen.getByRole("button", { name: "Stop" }));
    expect(onStop).toHaveBeenCalledWith(expect.objectContaining({ id: "r1" }));
  });

  it("disables only the busy rule's action buttons, not others", () => {
    const secondRule: ForwardRuleResponse = { ...tcpRule, id: "r3", name: "Other Rule" };
    const secondStatus: ForwardStatus = { ruleId: "r3", running: false, bytesIn: 0, bytesOut: 0 };
    renderList({
      rules: [tcpRule, secondRule],
      statusMap: makeMap(stoppedStatus, secondStatus),
      busyRuleIds: new Set(["r1"]),
      loading: false
    });

    const allButtons = screen.getAllByRole("button");
    const disabledButtons = allButtons.filter((btn) => btn.hasAttribute("disabled"));
    expect(disabledButtons.length).toBeGreaterThan(0);

    const enabledButtons = allButtons.filter(
      (btn) => !btn.hasAttribute("disabled") && btn.getAttribute("type") === "button"
    );
    expect(enabledButtons.length).toBeGreaterThan(0);
  });

  it("calls onAddRule when the Add Rule button is clicked", async () => {
    const user = userEvent.setup();
    const onAddRule = vi.fn();
    renderList({
      rules: [],
      statusMap: new Map(),
      busyRuleIds: new Set(),
      loading: false,
      onAddRule
    });
    await user.click(screen.getByRole("button", { name: "+ Add Rule" }));
    expect(onAddRule).toHaveBeenCalled();
  });

  it("highlights the row being edited", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      editingRuleId: "r1"
    });
    // The row should have row-selected class
    const row = screen.getByText("Test Rule").closest("tr");
    expect(row).toHaveClass("row-selected");
  });

  it("shows the auto-refresh toggle checkbox", () => {
    renderList({
      rules: [],
      statusMap: new Map(),
      busyRuleIds: new Set(),
      loading: false,
    });
    expect(screen.getByRole("checkbox", { name: "Auto-refresh" })).toBeInTheDocument();
  });

  it("shows the interval select when auto-refresh is enabled", () => {
    renderList({
      rules: [],
      statusMap: new Map(),
      busyRuleIds: new Set(),
      loading: false,
      autoRefresh: true,
      autoRefreshInterval: 5,
    });
    expect(screen.getByRole("combobox", { name: "Auto-refresh interval" })).toBeInTheDocument();
  });

  it("calls onToggleAutoRefresh when the auto-refresh checkbox is clicked", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    renderList({
      rules: [],
      statusMap: new Map(),
      busyRuleIds: new Set(),
      loading: false,
      onToggleAutoRefresh: onToggle,
    });
    await user.click(screen.getByRole("checkbox", { name: "Auto-refresh" }));
    expect(onToggle).toHaveBeenCalled();
  });
});
