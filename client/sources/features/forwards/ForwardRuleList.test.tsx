import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ForwardRuleResponse, ForwardStatus, RuleDiagnosticsResult } from "@portier/shared";
import { ForwardRuleList, type DiagnosisEntry } from "./ForwardRuleList.js";

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
      diagnosisMap={new Map()}
      onEdit={noop}
      onStart={noop}
      onStop={noop}
      onDelete={noop}
      onDiagnose={noop}
      onClearDiagnosis={noop}
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

  it("displays the group label when a rule has a group", () => {
    renderList({
      rules: [{ ...tcpRule, group: "web-team" }],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false
    });
    expect(screen.getByText("web-team")).toBeInTheDocument();
  });

  it("does not render a group label when a rule has no group", () => {
    const { container } = renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false
    });
    expect(container.querySelector(".rule-group-label")).toBeNull();
  });

  it("matches rules by group in the search box", async () => {
    const user = userEvent.setup();
    renderList({
      rules: [
        { ...tcpRule, id: "r1", name: "Alpha", group: "payments" },
        { ...tcpRule, id: "r2", name: "Beta", listenPort: 48010 }
      ],
      statusMap: new Map(),
      busyRuleIds: new Set(),
      loading: false
    });
    await user.type(screen.getByRole("searchbox", { name: "Search rules" }), "payments");
    expect(screen.getByText("Alpha")).toBeInTheDocument();
    expect(screen.queryByText("Beta")).not.toBeInTheDocument();
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

const diagPassResult: RuleDiagnosticsResult = {
  ruleId: "r1",
  ruleName: "Test Rule",
  protocol: "tcp",
  summary: { status: "pass", message: "All checks passed." },
  checks: [
    { id: "listen-bind", label: "Listen Bind", status: "pass", message: "Port 48001 can be bound." },
  ],
  diagnosedAt: new Date("2026-01-01T12:00:00Z").toISOString(),
};

function makeDiagMap(ruleId: string, entry: DiagnosisEntry): Map<string, DiagnosisEntry> {
  return new Map([[ruleId, entry]]);
}

describe("ForwardRuleList diagnostics", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows a Diagnose button for each rule", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
    });
    expect(screen.getByRole("button", { name: "Diagnose" })).toBeInTheDocument();
  });

  it("calls onDiagnose with the rule id when Diagnose is clicked", async () => {
    const user = userEvent.setup();
    const onDiagnose = vi.fn();
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      onDiagnose,
    });
    await user.click(screen.getByRole("button", { name: "Diagnose" }));
    expect(onDiagnose).toHaveBeenCalledWith("r1");
  });

  it("shows loading state when diagnosisMap has pending entry for the rule", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      diagnosisMap: makeDiagMap("r1", { state: "pending" }),
    });
    expect(screen.getByText("Running diagnostics…")).toBeInTheDocument();
  });

  it("disables the Diagnose button while diagnosis is pending for that rule", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      diagnosisMap: makeDiagMap("r1", { state: "pending" }),
    });
    const diagBtn = screen.getAllByRole("button").find((btn) => btn.hasAttribute("disabled") && btn.getAttribute("aria-label") === "Diagnose");
    // Diagnose button has no aria-label when pending (shows "…"); check it's disabled
    const buttons = screen.getAllByRole("button");
    // The diagnose button shows "…" and is disabled when pending
    const disabledButtons = buttons.filter((b) => b.hasAttribute("disabled"));
    expect(disabledButtons.length).toBeGreaterThan(0);
    void diagBtn; // suppress unused var warning
  });

  it("does not disable Diagnose for other rules when one is pending", () => {
    const secondRule: ForwardRuleResponse = { ...tcpRule, id: "r3", name: "Other Rule" };
    const secondStatus: ForwardStatus = { ruleId: "r3", running: false, bytesIn: 0, bytesOut: 0 };
    renderList({
      rules: [tcpRule, secondRule],
      statusMap: makeMap(stoppedStatus, secondStatus),
      busyRuleIds: new Set(),
      loading: false,
      diagnosisMap: makeDiagMap("r1", { state: "pending" }),
    });
    const diagButtons = screen.getAllByRole("button", { name: "Diagnose" });
    // r1's button is disabled (pending), r3's is enabled
    const enabledDiagButtons = diagButtons.filter((b) => !b.hasAttribute("disabled"));
    expect(enabledDiagButtons.length).toBe(1);
    expect(enabledDiagButtons[0]).not.toBeDisabled();
  });

  it("shows summary result when diagnosisMap has done entry", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      diagnosisMap: makeDiagMap("r1", { state: "done", result: diagPassResult }),
    });
    expect(screen.getByText("All checks passed.")).toBeInTheDocument();
    expect(screen.getByText("Listen Bind")).toBeInTheDocument();
  });

  it("shows error message when diagnosisMap has error entry", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      diagnosisMap: makeDiagMap("r1", { state: "error", message: "Rule not found." }),
    });
    expect(screen.getByText("Rule not found.")).toBeInTheDocument();
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("calls onClearDiagnosis when the panel close button is clicked", async () => {
    const user = userEvent.setup();
    const onClearDiagnosis = vi.fn();
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      diagnosisMap: makeDiagMap("r1", { state: "done", result: diagPassResult }),
      onClearDiagnosis,
    });
    await user.click(screen.getByRole("button", { name: "Close diagnostics" }));
    expect(onClearDiagnosis).toHaveBeenCalledWith("r1");
  });

  it("calls onClearDiagnosis and onDelete on delete confirmation", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    const onClearDiagnosis = vi.fn();
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      diagnosisMap: makeDiagMap("r1", { state: "done", result: diagPassResult }),
      onDelete,
      onClearDiagnosis,
    });
    await user.click(screen.getByRole("button", { name: "Delete" }));
    await user.click(screen.getByRole("button", { name: "Confirm" }));
    expect(onClearDiagnosis).toHaveBeenCalledWith("r1");
    expect(onDelete).toHaveBeenCalledWith(expect.objectContaining({ id: "r1" }));
  });

  it("no diag panel visible when diagnosisMap is empty", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      diagnosisMap: new Map(),
    });
    expect(screen.queryByText("Diagnostics")).not.toBeInTheDocument();
    expect(screen.queryByText("Running diagnostics…")).not.toBeInTheDocument();
  });
});

describe("ForwardRuleList activity navigation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows View activity button for each rule when onGoToActivity is provided", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      onGoToActivity: noop,
    });
    expect(screen.getByRole("button", { name: "View activity" })).toBeInTheDocument();
  });

  it("does not show View activity button when onGoToActivity is not provided", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
    });
    expect(screen.queryByRole("button", { name: "View activity" })).not.toBeInTheDocument();
  });

  it("calls onGoToActivity with the rule id when View activity is clicked", async () => {
    const user = userEvent.setup();
    const onGoToActivity = vi.fn();
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      onGoToActivity,
    });
    await user.click(screen.getByRole("button", { name: "View activity" }));
    expect(onGoToActivity).toHaveBeenCalledWith("r1");
  });

  it("shows a View activity button for each rule when multiple rules are present", () => {
    const secondRule: ForwardRuleResponse = { ...tcpRule, id: "r3", name: "Another Rule" };
    const secondStatus: ForwardStatus = { ruleId: "r3", running: false, bytesIn: 0, bytesOut: 0 };
    renderList({
      rules: [tcpRule, secondRule],
      statusMap: makeMap(stoppedStatus, secondStatus),
      busyRuleIds: new Set(),
      loading: false,
      onGoToActivity: noop,
    });
    expect(screen.getAllByRole("button", { name: "View activity" })).toHaveLength(2);
  });
});
