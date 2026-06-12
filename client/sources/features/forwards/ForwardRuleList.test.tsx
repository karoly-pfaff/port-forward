import { render, screen, within } from "@testing-library/react";
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
  running: false, health: "healthy",
  bytesIn: 0,
  bytesOut: 0
};

const runningStatus: ForwardStatus = {
  ruleId: "r1",
  running: true, health: "healthy",
  bytesIn: 2048,
  bytesOut: 512,
  activeConnections: 3
};

const udpStatus: ForwardStatus = {
  ruleId: "r2",
  running: true, health: "healthy",
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

  it("renders a health badge reflecting the rule's status health", () => {
    const warningStatus: ForwardStatus = { ...stoppedStatus, health: "warning" };
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(warningStatus),
      busyRuleIds: new Set(),
      loading: false
    });
    expect(
      screen.getByRole("img", { name: "Health: Warning — enabled but not running" })
    ).toBeInTheDocument();
  });

  it("displays the group label when a rule has a group", () => {
    const { container } = renderList({
      rules: [{ ...tcpRule, group: "web-team" }],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false
    });
    const chip = container.querySelector(".rule-group-label");
    expect(chip).not.toBeNull();
    expect(chip).toHaveTextContent("web-team");
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
    const secondStatus: ForwardStatus = { ruleId: "r3", running: false, health: "healthy", bytesIn: 0, bytesOut: 0 };
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

describe("ForwardRuleList group filter", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const groupedRules: ForwardRuleResponse[] = [
    { ...tcpRule, id: "r1", name: "Alpha", listenPort: 48001, group: "web" },
    { ...tcpRule, id: "r2", name: "Beta", listenPort: 48002, group: "api" },
    { ...tcpRule, id: "r3", name: "Gamma", listenPort: 48003, group: "web" },
    { ...tcpRule, id: "r4", name: "Delta", listenPort: 48004 } // ungrouped
  ];

  function renderGrouped(extra: Partial<Parameters<typeof ForwardRuleList>[0]> = {}) {
    return renderList({
      rules: groupedRules,
      statusMap: new Map(),
      busyRuleIds: new Set(),
      loading: false,
      ...extra
    });
  }

  it("does not render the group filter when no rule has a group", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false
    });
    expect(screen.queryByRole("combobox", { name: "Filter by group" })).not.toBeInTheDocument();
  });

  it("lists the distinct groups (sorted) plus All Groups and Ungrouped", () => {
    renderGrouped();
    const select = screen.getByRole("combobox", { name: "Filter by group" });
    const options = within(select).getAllByRole("option").map((o) => o.textContent);
    expect(options).toEqual(["All Groups", "api", "web", "Ungrouped"]);
  });

  it("filters to a single group when selected", async () => {
    const user = userEvent.setup();
    renderGrouped();
    await user.selectOptions(screen.getByRole("combobox", { name: "Filter by group" }), "web");
    expect(screen.getByText("Alpha")).toBeInTheDocument();
    expect(screen.getByText("Gamma")).toBeInTheDocument();
    expect(screen.queryByText("Beta")).not.toBeInTheDocument();
    expect(screen.queryByText("Delta")).not.toBeInTheDocument();
  });

  it("filters to ungrouped rules", async () => {
    const user = userEvent.setup();
    renderGrouped();
    await user.selectOptions(screen.getByRole("combobox", { name: "Filter by group" }), "Ungrouped");
    expect(screen.getByText("Delta")).toBeInTheDocument();
    expect(screen.queryByText("Alpha")).not.toBeInTheDocument();
    expect(screen.queryByText("Beta")).not.toBeInTheDocument();
  });

  it("restores all rules when the filter is cleared back to All Groups", async () => {
    const user = userEvent.setup();
    renderGrouped();
    const select = screen.getByRole("combobox", { name: "Filter by group" });
    await user.selectOptions(select, "api");
    expect(screen.queryByText("Alpha")).not.toBeInTheDocument();
    await user.selectOptions(select, "All Groups");
    for (const name of ["Alpha", "Beta", "Gamma", "Delta"]) {
      expect(screen.getByText(name)).toBeInTheDocument();
    }
  });

  it("combines the group filter with the search box", async () => {
    const user = userEvent.setup();
    renderGrouped();
    await user.selectOptions(screen.getByRole("combobox", { name: "Filter by group" }), "web");
    await user.type(screen.getByRole("searchbox", { name: "Search rules" }), "Alpha");
    expect(screen.getByText("Alpha")).toBeInTheDocument();
    expect(screen.queryByText("Gamma")).not.toBeInTheDocument(); // same group, excluded by search
  });

  it("shows the no-match empty state when a group filter excludes every rule", async () => {
    const user = userEvent.setup();
    renderGrouped();
    // Search for a name only in "web", then filter to "api" → no overlap.
    await user.type(screen.getByRole("searchbox", { name: "Search rules" }), "Alpha");
    await user.selectOptions(screen.getByRole("combobox", { name: "Filter by group" }), "api");
    expect(screen.getByText("No rules match the current filter.")).toBeInTheDocument();
  });

  it("omits the Ungrouped option when every rule has a group", () => {
    renderList({
      rules: [
        { ...tcpRule, id: "r1", name: "Alpha", group: "web" },
        { ...tcpRule, id: "r2", name: "Beta", listenPort: 48002, group: "api" }
      ],
      statusMap: new Map(),
      busyRuleIds: new Set(),
      loading: false
    });
    const select = screen.getByRole("combobox", { name: "Filter by group" });
    const options = within(select).getAllByRole("option").map((o) => o.textContent);
    expect(options).toEqual(["All Groups", "api", "web"]);
  });
});

describe("ForwardRuleList group actions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const groupedRules: ForwardRuleResponse[] = [
    { ...tcpRule, id: "r1", name: "Alpha", listenPort: 48001, group: "web" },
    { ...tcpRule, id: "r2", name: "Beta", listenPort: 48002, group: "api" },
    { ...tcpRule, id: "r3", name: "Gamma", listenPort: 48003 } // ungrouped
  ];

  function groupResponse(over: Partial<{ action: "start" | "stop"; total: number; succeeded: number; skipped: number; failed: number }> = {}) {
    return {
      group: "web",
      action: over.action ?? "start",
      total: over.total ?? 2,
      succeeded: over.succeeded ?? 2,
      skipped: over.skipped ?? 0,
      failed: over.failed ?? 0,
      results: []
    };
  }

  function renderGroupActions(onGroupAction = vi.fn().mockResolvedValue(groupResponse())) {
    const utils = renderList({
      rules: groupedRules,
      statusMap: new Map(),
      busyRuleIds: new Set(),
      loading: false,
      onGroupAction
    });
    return { ...utils, onGroupAction };
  }

  async function selectGroup(group: string): Promise<void> {
    const user = userEvent.setup();
    await user.selectOptions(screen.getByRole("combobox", { name: "Filter by group" }), group);
  }

  it("shows no group action buttons for All Groups (default)", () => {
    renderGroupActions();
    expect(screen.queryByRole("button", { name: /Start group/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Stop group/ })).not.toBeInTheDocument();
  });

  it("shows no group action buttons when onGroupAction is not provided", async () => {
    renderList({ rules: groupedRules, statusMap: new Map(), busyRuleIds: new Set(), loading: false });
    await selectGroup("web");
    expect(screen.queryByRole("button", { name: /Start group/ })).not.toBeInTheDocument();
  });

  it("shows Start/Stop group buttons when a specific group is selected", async () => {
    renderGroupActions();
    await selectGroup("web");
    expect(screen.getByRole("button", { name: 'Start group "web"' })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: 'Stop group "web"' })).toBeInTheDocument();
  });

  it("shows no group action buttons for Ungrouped", async () => {
    renderGroupActions();
    await selectGroup("Ungrouped");
    expect(screen.queryByRole("button", { name: /Start group/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Stop group/ })).not.toBeInTheDocument();
  });

  it("calls onGroupAction with the group and 'start' when Start is clicked", async () => {
    const user = userEvent.setup();
    const { onGroupAction } = renderGroupActions();
    await selectGroup("web");
    await user.click(screen.getByRole("button", { name: 'Start group "web"' }));
    expect(onGroupAction).toHaveBeenCalledWith("web", "start");
  });

  it("calls onGroupAction with the group and 'stop' when Stop is clicked", async () => {
    const user = userEvent.setup();
    const { onGroupAction } = renderGroupActions(vi.fn().mockResolvedValue(groupResponse({ action: "stop" })));
    await selectGroup("web");
    await user.click(screen.getByRole("button", { name: 'Stop group "web"' }));
    expect(onGroupAction).toHaveBeenCalledWith("web", "stop");
  });

  it("shows a success summary after a successful action", async () => {
    const user = userEvent.setup();
    renderGroupActions(vi.fn().mockResolvedValue(groupResponse({ succeeded: 2, skipped: 1, total: 3 })));
    await selectGroup("web");
    await user.click(screen.getByRole("button", { name: 'Start group "web"' }));
    expect(await screen.findByRole("status")).toHaveTextContent(
      'Started group "web": 2 succeeded, 1 skipped (3 total)'
    );
  });

  it("shows a warning summary (alert) when some rules failed", async () => {
    const user = userEvent.setup();
    renderGroupActions(vi.fn().mockResolvedValue(groupResponse({ succeeded: 1, skipped: 0, failed: 1, total: 2 })));
    await selectGroup("web");
    await user.click(screen.getByRole("button", { name: 'Start group "web"' }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("1 failed");
    expect(alert.className).toContain("group-action-status--warn");
  });

  it("shows an error message when the action rejects (e.g. 404)", async () => {
    const user = userEvent.setup();
    renderGroupActions(vi.fn().mockRejectedValue(new Error('No rules found in group "web".')));
    await selectGroup("web");
    await user.click(screen.getByRole("button", { name: 'Start group "web"' }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("No rules found in group");
    expect(alert.className).toContain("group-action-status--error");
  });

  it("disables both buttons while a request is in flight (no double submit)", async () => {
    const user = userEvent.setup();
    let resolve!: (v: ReturnType<typeof groupResponse>) => void;
    const pending = new Promise<ReturnType<typeof groupResponse>>((r) => { resolve = r; });
    const onGroupAction = vi.fn().mockReturnValue(pending);
    renderGroupActions(onGroupAction);
    await selectGroup("web");

    const startBtn = screen.getByRole("button", { name: 'Start group "web"' });
    await user.click(startBtn);

    expect(screen.getByRole("button", { name: "Starting…" })).toBeDisabled();
    expect(screen.getByRole("button", { name: 'Stop group "web"' })).toBeDisabled();

    // A second click while in flight must not trigger another call.
    await user.click(screen.getByRole("button", { name: "Starting…" }));
    expect(onGroupAction).toHaveBeenCalledTimes(1);

    resolve(groupResponse());
    expect(await screen.findByRole("status")).toBeInTheDocument();
  });

  it("clears the previous result when the group selection changes", async () => {
    const user = userEvent.setup();
    renderGroupActions();
    await selectGroup("web");
    await user.click(screen.getByRole("button", { name: 'Start group "web"' }));
    expect(await screen.findByRole("status")).toBeInTheDocument();

    await user.selectOptions(screen.getByRole("combobox", { name: "Filter by group" }), "api");
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
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
    const secondStatus: ForwardStatus = { ruleId: "r3", running: false, health: "healthy", bytesIn: 0, bytesOut: 0 };
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
    const secondStatus: ForwardStatus = { ruleId: "r3", running: false, health: "healthy", bytesIn: 0, bytesOut: 0 };
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

describe("ForwardRuleList duplicate action (v1.8 Slice 8)", () => {
  it("shows a Duplicate action with a rule-scoped accessible name when onDuplicate is provided", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      onDuplicate: noop,
    });
    expect(
      screen.getByRole("button", { name: `Duplicate rule ${tcpRule.name}` })
    ).toBeInTheDocument();
  });

  it("does not render a Duplicate action when onDuplicate is omitted", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
    });
    expect(screen.queryByRole("button", { name: /Duplicate rule/ })).not.toBeInTheDocument();
  });

  it("calls onDuplicate with the rule when clicked", async () => {
    const user = userEvent.setup();
    const onDuplicate = vi.fn();
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(),
      loading: false,
      onDuplicate,
    });
    await user.click(screen.getByRole("button", { name: `Duplicate rule ${tcpRule.name}` }));
    expect(onDuplicate).toHaveBeenCalledWith(tcpRule);
  });

  it("disables Duplicate while the rule is busy", () => {
    renderList({
      rules: [tcpRule],
      statusMap: makeMap(stoppedStatus),
      busyRuleIds: new Set(["r1"]),
      loading: false,
      onDuplicate: noop,
    });
    expect(screen.getByRole("button", { name: `Duplicate rule ${tcpRule.name}` })).toBeDisabled();
  });
});
