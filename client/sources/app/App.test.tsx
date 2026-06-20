import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ForwardRuleResponse, ForwardStatus, RuntimeInfo } from "@portier/shared";
import { App } from "./App.js";
import * as portierApi from "../api/portierApi.js";

vi.mock("../api/portierApi.js", () => ({
  fetchForwardRules: vi.fn(),
  fetchForwardStatus: vi.fn(),
  saveForwardRule: vi.fn(),
  deleteForwardRule: vi.fn(),
  setForwardRuleRunning: vi.fn(),
  setGroupRunning: vi.fn(),
  reorderForwardRules: vi.fn(),
  fetchActivity: vi.fn().mockResolvedValue([]),
  diagnoseForwardRule: vi.fn(),
  exportConfig: vi.fn(),
  importConfig: vi.fn(),
  planConfig: vi.fn(),
  applyConfig: vi.fn(),
  fetchRuntimeInfo: vi.fn().mockResolvedValue(null),
  fetchLiveConnections: vi.fn().mockResolvedValue({
    generatedAt: new Date("2026-01-01T00:00:00.000Z").toISOString(),
    tcpConnections: [],
    udpSessions: [],
    ruleSummaries: []
  })
}));

describe("App", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the Portier brand name and purpose line", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);

    render(<App />);

    // Brand name appears in the header title
    expect(screen.getByText("Portier", { selector: ".app-header-title" })).toBeInTheDocument();
    // Subtitle appears in the header
    expect(
      screen.getByText(/TCP\/UDP port forwarding for local development/)
    ).toBeInTheDocument();

    await screen.findByText(/No forwarding rules yet/);
  });

  it("renders the Portier logo image with meaningful alt text", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);

    render(<App />);

    expect(screen.getByRole("img", { name: "Portier logo" })).toBeInTheDocument();
    await screen.findByText(/No forwarding rules yet/);
  });

  it("API Docs navigation opens the API docs view", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);

    render(<App />);
    await screen.findByText(/No forwarding rules yet/);

    // "API Docs" appears in both the sidebar and header — click either one
    const apiDocsBtns = screen.getAllByRole("button", { name: /API Docs/ });
    expect(apiDocsBtns.length).toBeGreaterThanOrEqual(1);
    await userEvent.click(apiDocsBtns[0]);
    expect(screen.getByText("API Reference")).toBeInTheDocument();
  });

  it("renders all sidebar navigation items as clickable buttons", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);

    render(<App />);

    // All nav items are now functional buttons — no "soon" badges
    expect(screen.queryAllByText("soon")).toHaveLength(0);
    // Sidebar nav buttons (use nav landmark to scope)
    const nav = screen.getByRole("navigation", { name: "Main navigation" });
    expect(nav.querySelector("button[aria-label], button")).toBeTruthy();
    expect(screen.getByRole("button", { name: /Dashboard/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Forward Rules/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Activity/ })).toBeInTheDocument();
    // "Settings" and "API Docs" appear in both sidebar and header
    expect(screen.getAllByRole("button", { name: /Settings/ }).length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByRole("button", { name: /API Docs/ }).length).toBeGreaterThanOrEqual(1);
    await screen.findByText(/No forwarding rules yet/);
  });

  it("clicking Activity nav item switches to the activity view", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<App />);
    await screen.findByText(/No forwarding rules yet/);

    await userEvent.click(screen.getByRole("button", { name: /Activity/ }));
    await screen.findByText("Recent forwarding and rule events");
  });

  it("shows server unavailable banner when the API fails with a network error", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockRejectedValue(
      new TypeError("Failed to fetch")
    );
    vi.mocked(portierApi.fetchForwardStatus).mockRejectedValue(
      new TypeError("Failed to fetch")
    );

    render(<App />);

    await screen.findByText(/Server unavailable/);
  });

  it("shows loaded rules and running count after a successful load", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([
      {
        id: "r1",
        name: "Test Rule",
        protocol: "tcp",
        listenHost: "127.0.0.1",
        listenPort: 48001,
        targetHost: "127.0.0.1",
        targetPort: 3000,
        enabled: true,
        advisories: []
      }
    ]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([
      {
        ruleId: "r1",
        running: true, health: "healthy",
        bytesIn: 0,
        bytesOut: 0
      }
    ]);

    render(<App />);

    await screen.findByText("Test Rule");
    // Running count span in the rule-list-section header
    expect(screen.getByText("1 running")).toBeInTheDocument();
  });

  it("Dashboard view renders stat cards when navigated to", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([
      {
        id: "r1",
        name: "My Rule",
        protocol: "tcp",
        listenHost: "127.0.0.1",
        listenPort: 48001,
        targetHost: "127.0.0.1",
        targetPort: 3000,
        enabled: true,
        advisories: []
      }
    ]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([
      { ruleId: "r1", running: true, health: "healthy", bytesIn: 0, bytesOut: 0 }
    ]);
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<App />);
    await screen.findByText("My Rule");

    await userEvent.click(screen.getByRole("button", { name: /Dashboard/ }));
    expect(screen.getByText("Total Rules")).toBeInTheDocument();
    expect(screen.getByText(/1 TCP/)).toBeInTheDocument();
  });

  it("mobile menu button opens the sidebar overlay", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);

    render(<App />);
    await screen.findByText(/No forwarding rules yet/);

    const menuBtn = screen.getByRole("button", { name: /Open navigation menu/ });
    expect(menuBtn).toBeInTheDocument();

    await userEvent.click(menuBtn);
    expect(document.querySelector(".sidebar--mobile-open")).toBeInTheDocument();
  });

  it("clicking a nav item closes the mobile sidebar", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<App />);
    await screen.findByText(/No forwarding rules yet/);

    await userEvent.click(screen.getByRole("button", { name: /Open navigation menu/ }));
    expect(document.querySelector(".sidebar--mobile-open")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Activity/ }));
    expect(document.querySelector(".sidebar--mobile-open")).not.toBeInTheDocument();
  });

  it("shows error count when a rule has lastError", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([
      {
        id: "r1",
        name: "Broken Rule",
        protocol: "tcp",
        listenHost: "127.0.0.1",
        listenPort: 48001,
        targetHost: "127.0.0.1",
        targetPort: 3000,
        enabled: true,
        advisories: []
      }
    ]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([
      {
        ruleId: "r1",
        running: false, health: "healthy",
        bytesIn: 0,
        bytesOut: 0,
        lastError: "ECONNREFUSED"
      }
    ]);

    render(<App />);

    await screen.findByText("Broken Rule");
    expect(screen.getByText("1 error")).toBeInTheDocument();
  });
});

const sampleRule = {
  id: "r1",
  name: "Test Rule",
  protocol: "tcp" as const,
  listenHost: "127.0.0.1",
  listenPort: 48001,
  targetHost: "127.0.0.1",
  targetPort: 3000,
  enabled: false,
  advisories: []
};

const stoppedStatus: ForwardStatus = { ruleId: "r1", running: false, health: "healthy", bytesIn: 0, bytesOut: 0 };

describe("App drawer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  it("Add Rule button opens the drawer; Cancel/close button closes it", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);

    render(<App />);
    await screen.findByText(/No forwarding rules yet/);

    await userEvent.click(screen.getByRole("button", { name: /\+ Add Rule/ }));
    expect(screen.getByRole("complementary", { name: "Add Forward Rule" })).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("complementary", { name: "Add Forward Rule" })).not.toBeInTheDocument();
  });

  it("Edit button opens the drawer pre-filled with rule data", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "More actions for Test Rule" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Edit" }));
    expect(screen.getByRole("complementary", { name: "Edit Forward Rule" })).toBeInTheDocument();
    expect(screen.getByDisplayValue("Test Rule")).toBeInTheDocument();
  });

  it("saving an edited rule calls update API and refreshes rule list", async () => {
    const updatedRule = { ...sampleRule, name: "Updated Rule" };
    vi.mocked(portierApi.fetchForwardRules)
      .mockResolvedValueOnce([sampleRule])
      .mockResolvedValue([updatedRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);
    vi.mocked(portierApi.saveForwardRule).mockResolvedValue({ ...updatedRule });

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "More actions for Test Rule" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Edit" }));

    const nameInput = screen.getByDisplayValue("Test Rule");
    await userEvent.clear(nameInput);
    await userEvent.type(nameInput, "Updated Rule");

    await userEvent.click(screen.getByRole("button", { name: "Save Changes" }));

    expect(portierApi.saveForwardRule).toHaveBeenCalledWith(
      "r1",
      expect.objectContaining({ name: "Updated Rule" })
    );
    await screen.findByText("Updated Rule");
  });

  it("Duplicate opens the create form pre-filled from the source rule", async () => {
    const groupedRule = { ...sampleRule, group: "backend" };
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([groupedRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "More actions for Test Rule" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Duplicate" }));

    // Create mode (a "Duplicate Forward Rule" drawer), pre-filled, not editing.
    expect(
      screen.getByRole("complementary", { name: "Duplicate Forward Rule" })
    ).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Duplicate Rule" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("Test Rule copy");
    expect(screen.getByRole("textbox", { name: "Group" })).toHaveValue("backend");
  });

  it("saving a duplicate calls create (id undefined) and leaves the source rule unchanged", async () => {
    const groupedRule = { ...sampleRule, group: "backend" };
    const createdRule = { ...groupedRule, id: "r2", name: "Test Rule copy" };
    vi.mocked(portierApi.fetchForwardRules)
      .mockResolvedValueOnce([groupedRule])
      .mockResolvedValue([groupedRule, createdRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);
    vi.mocked(portierApi.saveForwardRule).mockResolvedValue({ ...createdRule, advisories: [] });

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "More actions for Test Rule" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Duplicate" }));
    await userEvent.click(screen.getByRole("button", { name: "Add Rule" }));

    // Create path: id is undefined; group carried over; autostart forced off.
    expect(portierApi.saveForwardRule).toHaveBeenCalledWith(
      undefined,
      expect.objectContaining({ name: "Test Rule copy", group: "backend", enabled: false })
    );
    // No update call against the source rule.
    expect(portierApi.saveForwardRule).not.toHaveBeenCalledWith("r1", expect.anything());

    // Both the original and the duplicate are listed after refresh.
    await screen.findByText("Test Rule copy");
    expect(screen.getByText("Test Rule")).toBeInTheDocument();
  });

  it("Cancel after Duplicate makes no save call and keeps the source rule", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "More actions for Test Rule" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Duplicate" }));
    await userEvent.click(screen.getByRole("button", { name: "Close" }));

    expect(
      screen.queryByRole("complementary", { name: "Duplicate Forward Rule" })
    ).not.toBeInTheDocument();
    expect(portierApi.saveForwardRule).not.toHaveBeenCalled();
    expect(screen.getByText("Test Rule")).toBeInTheDocument();
  });
});

describe("App diagnostics", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  it("clicking Diagnose calls diagnoseForwardRule and shows result summary", async () => {
    const diagResult = {
      ruleId: "r1",
      ruleName: "Test Rule",
      protocol: "tcp" as const,
      summary: { status: "pass" as const, message: "All checks passed." },
      checks: [
        { id: "listen-bind", label: "Listen Bind", status: "pass" as const, message: "Port 48001 can be bound." },
      ],
      diagnosedAt: new Date("2026-01-01T12:00:00Z").toISOString(),
    };

    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);
    vi.mocked(portierApi.diagnoseForwardRule).mockResolvedValue(diagResult);

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "More actions for Test Rule" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Diagnose" }));

    expect(portierApi.diagnoseForwardRule).toHaveBeenCalledWith("r1");
    await screen.findByText("All checks passed.");
    expect(screen.getByText("Listen Bind")).toBeInTheDocument();
  });

  it("shows error in the diagnostics panel when diagnoseForwardRule rejects", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);
    vi.mocked(portierApi.diagnoseForwardRule).mockRejectedValue(new Error("Service unavailable"));

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "More actions for Test Rule" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Diagnose" }));

    await screen.findByText("Service unavailable");
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });
});

describe("App error banners", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  it("shows error banner when start API call fails", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);
    vi.mocked(portierApi.setForwardRuleRunning).mockRejectedValue(
      new Error("Port already in use")
    );

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "Start" }));

    await screen.findByText("Port already in use");
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("shows error banner when stop API call fails", async () => {
    const runningStatus = { ...stoppedStatus, running: true };
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([runningStatus]);
    vi.mocked(portierApi.setForwardRuleRunning).mockRejectedValue(
      new Error("Stop failed")
    );

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "Stop" }));

    await screen.findByText("Stop failed");
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("shows error banner when delete API call fails", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);
    vi.mocked(portierApi.deleteForwardRule).mockRejectedValue(
      new Error("Delete failed")
    );

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "More actions for Test Rule" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Delete" }));
    await userEvent.click(screen.getByRole("button", { name: "Confirm" }));

    await screen.findByText("Delete failed");
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });
});

describe("App group actions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const webRules = [
    { id: "r1", name: "Web One", protocol: "tcp" as const, listenHost: "127.0.0.1", listenPort: 48001, targetHost: "127.0.0.1", targetPort: 3000, enabled: false, group: "web", advisories: [] },
    { id: "r2", name: "Web Two", protocol: "tcp" as const, listenHost: "127.0.0.1", listenPort: 48002, targetHost: "127.0.0.1", targetPort: 3001, enabled: false, group: "web", advisories: [] }
  ];

  it("starts a group via the API and refreshes rules afterward", async () => {
    const user = userEvent.setup();
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue(webRules);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([
      { ruleId: "r1", running: false, health: "healthy", bytesIn: 0, bytesOut: 0 },
      { ruleId: "r2", running: false, health: "healthy", bytesIn: 0, bytesOut: 0 }
    ]);
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
    vi.mocked(portierApi.setGroupRunning).mockResolvedValue({
      group: "web", action: "start", total: 2, succeeded: 2, skipped: 0, failed: 0, results: []
    });

    render(<App />);
    await screen.findByText("Web One");

    const fetchCallsBefore = vi.mocked(portierApi.fetchForwardRules).mock.calls.length;

    await user.selectOptions(screen.getByRole("combobox", { name: "Filter by group" }), "web");
    await user.click(screen.getByRole("button", { name: 'Start group "web"' }));

    expect(portierApi.setGroupRunning).toHaveBeenCalledWith("web", true);
    // refreshAll re-fetches rules after the action.
    await screen.findByRole("status");
    expect(vi.mocked(portierApi.fetchForwardRules).mock.calls.length).toBeGreaterThan(fetchCallsBefore);
    expect(screen.getByRole("status")).toHaveTextContent('Started group "web"');
  });

  it("stops a group via the API", async () => {
    const user = userEvent.setup();
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue(webRules);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([
      { ruleId: "r1", running: true, health: "healthy", bytesIn: 0, bytesOut: 0 },
      { ruleId: "r2", running: true, health: "healthy", bytesIn: 0, bytesOut: 0 }
    ]);
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
    vi.mocked(portierApi.setGroupRunning).mockResolvedValue({
      group: "web", action: "stop", total: 2, succeeded: 2, skipped: 0, failed: 0, results: []
    });

    render(<App />);
    await screen.findByText("Web One");

    await user.selectOptions(screen.getByRole("combobox", { name: "Filter by group" }), "web");
    await user.click(screen.getByRole("button", { name: 'Stop group "web"' }));

    expect(portierApi.setGroupRunning).toHaveBeenCalledWith("web", false);
    await screen.findByRole("status");
  });
});

describe("App settings and auto-refresh", () => {
  const sampleRule = {
    id: "r1", name: "Test Rule", protocol: "tcp" as const, listenHost: "127.0.0.1",
    listenPort: 48001, targetHost: "127.0.0.1", targetPort: 3000, enabled: false, advisories: []
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  it("the header Settings button opens the Settings view", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);

    render(<App />);
    // The header (banner) Settings button is distinct from the sidebar nav item.
    const header = await screen.findByRole("banner");
    await userEvent.click(within(header).getByRole("button", { name: "Settings" }));

    expect(await screen.findByText("Management Endpoint")).toBeInTheDocument();
  });

  it("toggling Auto-refresh enables the interval control and applies a new interval", async () => {
    const user = userEvent.setup();
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([
      { ruleId: "r1", running: false, health: "healthy", bytesIn: 0, bytesOut: 0 }
    ]);

    render(<App />);
    await screen.findByText("Test Rule");

    const intervalSelect = screen.getByRole("combobox", { name: "Auto-refresh interval" });
    expect(intervalSelect).toBeDisabled();

    await user.click(screen.getByRole("checkbox", { name: "Auto-refresh" }));
    expect(intervalSelect).toBeEnabled();

    await user.selectOptions(intervalSelect, "10");
    expect((intervalSelect as HTMLSelectElement).value).toBe("10");
  });

  it("shows the server-unavailable banner for a network-named TypeError", async () => {
    // Exercises the isNetworkError "network" branch (distinct from the "fetch" branch).
    vi.mocked(portierApi.fetchForwardRules).mockRejectedValue(
      new TypeError("network request failed")
    );
    vi.mocked(portierApi.fetchForwardStatus).mockRejectedValue(
      new TypeError("network request failed")
    );

    render(<App />);

    expect(await screen.findByText(/Server unavailable/)).toBeInTheDocument();
  });
});

// Two ungrouped, unfiltered TCP rules so the list is drag-reorderable
// (canReorder requires no search/status/group filter active).
const reorderRuleA: ForwardRuleResponse = {
  id: "ra", name: "Rule A", protocol: "tcp", listenHost: "127.0.0.1",
  listenPort: 48001, targetHost: "127.0.0.1", targetPort: 3000, enabled: false, advisories: []
};
const reorderRuleB: ForwardRuleResponse = {
  id: "rb", name: "Rule B", protocol: "tcp", listenHost: "127.0.0.1",
  listenPort: 48002, targetHost: "127.0.0.1", targetPort: 3001, enabled: false, advisories: []
};
const reorderStatuses: ForwardStatus[] = [
  { ruleId: "ra", running: false, health: "healthy", bytesIn: 0, bytesOut: 0 },
  { ruleId: "rb", running: false, health: "healthy", bytesIn: 0, bytesOut: 0 }
];

function rowFor(name: string): HTMLTableRowElement {
  const cell = screen.getByRole("cell", { name });
  const row = cell.closest("tr");
  if (!row) throw new Error(`No row for ${name}`);
  return row;
}

describe("App reorder", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  it("dragging one rule onto another calls reorderForwardRules and renders the new order", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([reorderRuleA, reorderRuleB]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue(reorderStatuses);
    // Server returns the rules in the reordered order (B before A).
    vi.mocked(portierApi.reorderForwardRules).mockResolvedValue([reorderRuleB, reorderRuleA]);

    render(<App />);
    await screen.findByText("Rule A");

    // Drag Rule A (first) onto Rule B (second) → ["rb","ra"]? No: A moves to B's index.
    fireEvent.dragStart(rowFor("Rule A"));
    fireEvent.dragOver(rowFor("Rule B"));
    fireEvent.drop(rowFor("Rule B"));

    await waitFor(() => {
      expect(portierApi.reorderForwardRules).toHaveBeenCalledWith(["rb", "ra"]);
    });

    // The list reflects the server-returned order: Rule B now precedes Rule A.
    const ruleRows = screen
      .getAllByRole("row")
      .filter((r) => within(r).queryByRole("cell", { name: /^Rule [AB]$/ }));
    const orderedNames = ruleRows.map(
      (r) => within(r).getByRole("cell", { name: /^Rule [AB]$/ }).textContent
    );
    expect(orderedNames[0]).toBe("Rule B");
    expect(orderedNames[1]).toBe("Rule A");
  });

  it("shows an error banner when reorderForwardRules rejects", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([reorderRuleA, reorderRuleB]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue(reorderStatuses);
    vi.mocked(portierApi.reorderForwardRules).mockRejectedValue(new Error("Reorder failed"));

    render(<App />);
    await screen.findByText("Rule A");

    fireEvent.dragStart(rowFor("Rule A"));
    fireEvent.dragOver(rowFor("Rule B"));
    fireEvent.drop(rowFor("Rule B"));

    await screen.findByText("Reorder failed");
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });
});

describe("App go-to-activity", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  it("the row Activity action navigates to the Activity view filtered to that rule", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "More actions for Test Rule" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Activity" }));

    // Activity view is shown, scoped to the selected rule via the filter banner.
    await screen.findByText("Recent forwarding and rule events");
    const banner = await screen.findByText(/Filtered to rule:/);
    expect(within(banner).getByText("Test Rule")).toBeInTheDocument();
  });
});

describe("App auto-refresh interval", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("re-fetches rules and statuses on the interval once Auto-refresh is enabled", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);

    // Fake timers are installed BEFORE render, so every interval the effect ever
    // creates is the fake one — there is no real→fake handoff to race against.
    // All interactions use synchronous fireEvent (no userEvent timers), and
    // pending microtasks/effects are flushed by advancing the fake clock by 0.
    vi.useFakeTimers();
    try {
      render(<App />);
      // Flush the initial load Promise.all and resulting state updates. Several
      // microtask hops are involved, so advance the fake clock inside act()
      // until the rule row appears.
      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });
      expect(screen.getByText("Test Rule")).toBeInTheDocument();

      // Enable auto-refresh (default interval 5s). The effect creates a fake
      // setInterval; flush so the effect runs and the interval is registered.
      fireEvent.click(screen.getByRole("checkbox", { name: "Auto-refresh" }));
      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });

      const rulesBefore = vi.mocked(portierApi.fetchForwardRules).mock.calls.length;
      const statusBefore = vi.mocked(portierApi.fetchForwardStatus).mock.calls.length;

      // Advance one interval tick (default 5s); the interval callback re-fetches.
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });

      expect(vi.mocked(portierApi.fetchForwardRules).mock.calls.length).toBeGreaterThan(rulesBefore);
      expect(vi.mocked(portierApi.fetchForwardStatus).mock.calls.length).toBeGreaterThan(statusBefore);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("App save-rule error path", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  it("keeps the edit drawer open and surfaces the error when saveForwardRule rejects", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);
    // The save call rejects: App must set savingForm=false and re-throw so the
    // ForwardRuleForm catches it, renders the message, and the drawer stays open.
    vi.mocked(portierApi.saveForwardRule).mockRejectedValue(
      new Error("Service rejected the change.")
    );

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "More actions for Test Rule" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Edit" }));

    const drawer = screen.getByRole("complementary", { name: "Edit Forward Rule" });
    expect(drawer).toBeInTheDocument();

    const nameInput = screen.getByDisplayValue("Test Rule");
    await userEvent.clear(nameInput);
    await userEvent.type(nameInput, "Renamed Rule");

    await userEvent.click(screen.getByRole("button", { name: "Save Changes" }));

    expect(portierApi.saveForwardRule).toHaveBeenCalledWith(
      "r1",
      expect.objectContaining({ name: "Renamed Rule" })
    );

    // The form surfaces the thrown error inside the still-open drawer.
    await within(drawer).findByText("Service rejected the change.");
    expect(
      screen.getByRole("complementary", { name: "Edit Forward Rule" })
    ).toBeInTheDocument();
    // The Save button is interactable again (savingForm was reset to false).
    expect(screen.getByRole("button", { name: "Save Changes" })).toBeEnabled();
  });
});

// FileReader stub that synchronously delivers content via queueMicrotask. The
// real component reads the file with FileReader; jsdom's implementation is not
// reliable across runs, so we stub it deterministically (copied from
// SettingsView.test.tsx's import-flow tests).
function stubFileReader(content: string): void {
  vi.stubGlobal(
    "FileReader",
    class {
      result: string = content;
      onload: ((e: ProgressEvent) => void) | null = null;
      readAsText(): void {
        queueMicrotask(() => {
          this.onload?.({ target: { result: this.result } } as unknown as ProgressEvent);
        });
      }
    }
  );
}

describe("App rules-updated-from-settings", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("a successful Import in Settings updates the rule list and refetches status", async () => {
    // Drive handleRulesUpdatedFromSettings through the REAL Settings view: a
    // successful merge import returns updated rules, so App's onRulesUpdated runs
    // (setRules + a fresh fetchForwardStatus).
    const importedRule: ForwardRuleResponse = {
      id: "r9",
      name: "Imported From Settings",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48050,
      targetHost: "127.0.0.1",
      targetPort: 9100,
      enabled: false,
      advisories: []
    };

    // Initial load returns no rules; the import then introduces the new rule.
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);

    const configJson = JSON.stringify({
      version: "1",
      exportedAt: new Date("2026-01-01T00:00:00.000Z").toISOString(),
      rules: [
        {
          id: "r9",
          name: "Imported From Settings",
          protocol: "tcp",
          listenHost: "127.0.0.1",
          listenPort: 48050,
          targetHost: "127.0.0.1",
          targetPort: 9100,
          enabled: false
        }
      ]
    });
    stubFileReader(configJson);

    vi.mocked(portierApi.importConfig).mockResolvedValue({
      result: { imported: 1, skipped: 0, errors: [] },
      rules: [importedRule]
    });

    render(<App />);
    await screen.findByText(/No forwarding rules yet/);

    // Navigate to Settings via the header button (scoped to the banner).
    const header = screen.getByRole("banner");
    await userEvent.click(within(header).getByRole("button", { name: "Settings" }));
    await screen.findByText("Management Endpoint");

    const statusCallsBefore = vi.mocked(portierApi.fetchForwardStatus).mock.calls.length;

    // Upload a valid config and run a merge import.
    const file = new File([configJson], "portier-rules.json", { type: "application/json" });
    await userEvent.upload(screen.getByLabelText("Select config file"), file);
    await screen.findByText("File preview");
    await userEvent.click(screen.getByRole("button", { name: /Import Rules/ }));

    expect(portierApi.importConfig).toHaveBeenCalledWith(
      expect.objectContaining({ version: "1" }),
      "merge"
    );
    await screen.findByText(/Import complete/);

    // onRulesUpdated ran: App refetched status …
    await waitFor(() => {
      expect(vi.mocked(portierApi.fetchForwardStatus).mock.calls.length).toBeGreaterThan(
        statusCallsBefore
      );
    });

    // … and the new rule list is reflected back in the Forward Rules view.
    await userEvent.click(screen.getByRole("button", { name: /Forward Rules/ }));
    await screen.findByText("Imported From Settings");
  });
});

describe("App mobile sidebar dismissal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  it("pressing Escape closes the open mobile sidebar", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);

    render(<App />);
    await screen.findByText(/No forwarding rules yet/);

    await userEvent.click(screen.getByRole("button", { name: /Open navigation menu/ }));
    expect(document.querySelector(".sidebar--mobile-open")).toBeInTheDocument();

    // The mobile-sidebar Escape handler (App line 103) closes the overlay.
    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() =>
      expect(document.querySelector(".sidebar--mobile-open")).not.toBeInTheDocument()
    );
  });

  it("clicking the backdrop closes the open mobile sidebar", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);

    render(<App />);
    await screen.findByText(/No forwarding rules yet/);

    await userEvent.click(screen.getByRole("button", { name: /Open navigation menu/ }));
    const backdrop = document.querySelector(".sidebar-backdrop");
    expect(backdrop).toBeInTheDocument();

    // The backdrop onClick (App line 327) closes the overlay and removes itself.
    await userEvent.click(backdrop as Element);
    expect(document.querySelector(".sidebar--mobile-open")).not.toBeInTheDocument();
    expect(document.querySelector(".sidebar-backdrop")).not.toBeInTheDocument();
  });
});

describe("App dashboard navigation shortcuts", () => {
  const dashRule: ForwardRuleResponse = {
    id: "r1", name: "Dash Rule", protocol: "tcp", listenHost: "127.0.0.1",
    listenPort: 48001, targetHost: "127.0.0.1", targetPort: 3000, enabled: true, advisories: []
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  async function goToDashboard(): Promise<void> {
    render(<App />);
    await screen.findByText("Dash Rule");
    await userEvent.click(screen.getByRole("button", { name: /Dashboard/ }));
    await screen.findByText("Top Rules by Traffic");
  }

  it("'View All' in Top Rules navigates to the Forward Rules view", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([dashRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([
      { ruleId: "r1", running: true, health: "healthy", bytesIn: 0, bytesOut: 0 }
    ]);

    await goToDashboard();

    // The Top Rules panel's "View All" wires App's onGoToRules (App line 369).
    const topRulesHeader = screen.getByText("Top Rules by Traffic").closest<HTMLElement>(".dashboard-panel-header");
    if (!topRulesHeader) throw new Error("Top Rules header not found");
    await userEvent.click(within(topRulesHeader).getByRole("button", { name: "View All" }));

    // Forward Rules view is shown (its add button and summary appear).
    expect(await screen.findByRole("button", { name: /\+ Add Rule/ })).toBeInTheDocument();
  });

  it("'View All' in Recent Activity navigates to the Activity view", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([dashRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([
      { ruleId: "r1", running: true, health: "healthy", bytesIn: 0, bytesOut: 0 }
    ]);

    await goToDashboard();

    // The Recent Activity panel's "View All" wires App's onGoToActivity (App line 368).
    const activityHeader = screen.getByText("Recent Activity").closest<HTMLElement>(".dashboard-panel-header");
    if (!activityHeader) throw new Error("Recent Activity header not found");
    await userEvent.click(within(activityHeader).getByRole("button", { name: "View All" }));

    await screen.findByText("Recent forwarding and rule events");
  });
});

describe("App activity rule-filter clearing", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  it("clearing the rule filter in the Activity view dismisses the filter banner", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);

    render(<App />);
    await screen.findByText("Test Rule");

    // Open the Activity view filtered to the rule (sets activityRuleFilter).
    await userEvent.click(screen.getByRole("button", { name: "More actions for Test Rule" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Activity" }));
    const banner = await screen.findByText(/Filtered to rule:/);

    // ActivityLogView's "Clear" invokes App's onClearRuleFilter (App line 413),
    // which sets activityRuleFilter null → the banner disappears.
    await userEvent.click(within(banner.closest<HTMLElement>("div")!).getByRole("button", { name: "Clear" }));

    await waitFor(() =>
      expect(screen.queryByText(/Filtered to rule:/)).not.toBeInTheDocument()
    );
  });
});

describe("App load/refresh non-network error paths", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  // loadInitial's non-network error branch (App lines 124-128): a plain Error
  // (not a TypeError) is NOT a network error, so isNetworkError is false and the
  // error message is surfaced via setErrors — the "Server unavailable" banner
  // must NOT appear.
  it("shows the error banner (not Server unavailable) when initial load fails with a plain Error", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockRejectedValue(new Error("boom"));
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);

    render(<App />);

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("boom");
    expect(screen.queryByText(/Server unavailable/)).not.toBeInTheDocument();
  });

  // refreshAll's catch (App lines 145-147): a successful Start triggers
  // refreshAll(); when that re-fetch rejects, the error message surfaces in the
  // banner. The initial load resolves once, then the post-action fetch rejects.
  it("surfaces the refresh error when the post-Start refreshAll fails", async () => {
    vi.mocked(portierApi.fetchForwardRules)
      .mockResolvedValueOnce([sampleRule])
      .mockRejectedValueOnce(new Error("refresh boom"));
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);
    vi.mocked(portierApi.setForwardRuleRunning).mockResolvedValue({
      ...stoppedStatus,
      running: true
    });

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "Start" }));

    await screen.findByText("refresh boom");
    expect(screen.getByRole("alert")).toHaveTextContent("refresh boom");
  });
});

describe("App start/stop refresh and delete-while-editing", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  it("starting a rule succeeds and refreshes the running count", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    // First load: stopped. After a successful start, refreshAll re-fetches: running.
    vi.mocked(portierApi.fetchForwardStatus)
      .mockResolvedValueOnce([stoppedStatus])
      .mockResolvedValue([{ ...stoppedStatus, running: true }]);
    vi.mocked(portierApi.setForwardRuleRunning).mockResolvedValue({
      ...stoppedStatus,
      running: true
    });

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "Start" }));

    // refreshAll ran on success → the header running count updates.
    expect(await screen.findByText("1 running")).toBeInTheDocument();
    expect(portierApi.setForwardRuleRunning).toHaveBeenCalledWith(
      expect.objectContaining({ id: "r1" }),
      true
    );
  });

  it("stopping a rule succeeds and refreshes the running count to zero", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    vi.mocked(portierApi.fetchForwardStatus)
      .mockResolvedValueOnce([{ ...stoppedStatus, running: true }])
      .mockResolvedValue([stoppedStatus]);
    vi.mocked(portierApi.setForwardRuleRunning).mockResolvedValue(stoppedStatus);

    render(<App />);
    await screen.findByText("Test Rule");
    expect(screen.getByText("1 running")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Stop" }));

    expect(await screen.findByText("0 running")).toBeInTheDocument();
    expect(portierApi.setForwardRuleRunning).toHaveBeenCalledWith(
      expect.objectContaining({ id: "r1" }),
      false
    );
  });

  it("deleting the rule that is open in the edit drawer closes the drawer", async () => {
    vi.mocked(portierApi.fetchForwardRules)
      .mockResolvedValueOnce([sampleRule])
      .mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);
    vi.mocked(portierApi.deleteForwardRule).mockResolvedValue(undefined);

    render(<App />);
    await screen.findByText("Test Rule");

    // Open the edit drawer for the rule (editingRuleId === r1).
    await userEvent.click(screen.getByRole("button", { name: "More actions for Test Rule" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Edit" }));
    expect(
      screen.getByRole("complementary", { name: "Edit Forward Rule" })
    ).toBeInTheDocument();

    // Delete that same rule → handleDelete's editingRuleId===rule.id branch closes the drawer.
    await userEvent.click(screen.getByRole("button", { name: "More actions for Test Rule" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Delete" }));
    await userEvent.click(screen.getByRole("button", { name: "Confirm" }));

    await waitFor(() =>
      expect(
        screen.queryByRole("complementary", { name: "Edit Forward Rule" })
      ).not.toBeInTheDocument()
    );
  });
});

// A complete RuntimeInfo whose recovery block reports an active config-recovery
// state, used to exercise App's `runtimeInfo?.recovery` truthy branch (line 344).
const recoveringRuntimeInfo: RuntimeInfo = {
  name: "portier",
  version: "1.18.0",
  runtime: "go",
  platform: "windows",
  arch: "x64",
  uptimeSeconds: 42,
  startedAt: new Date("2026-01-01T00:00:00.000Z").toISOString(),
  managementHost: "127.0.0.1",
  managementPort: 47831,
  configPath: "C:/ProgramData/Portier/rules.json",
  staticDir: "C:/ProgramData/Portier/web",
  serviceMode: true,
  pid: 1234,
  recovery: {
    active: true,
    reason: "malformed",
    message: "Portier started without your saved configuration.",
    configPath: "C:/ProgramData/Portier/rules.json",
    quarantinePath: "C:/ProgramData/Portier/rules.json.quarantine",
    writesBlocked: true,
    detectedAt: new Date("2026-01-01T00:00:00.000Z").toISOString()
  }
};

describe("App recovery banner", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  it("renders the recovery banner when runtime info reports active recovery", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);
    // useRuntimeInfo resolves to a runtime whose recovery.active is true, taking
    // App's `runtimeInfo?.recovery` truthy branch (App line 344).
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValueOnce(recoveringRuntimeInfo);

    render(<App />);
    await screen.findByText(/No forwarding rules yet/);

    // RecoveryBanner renders its heading plus the writes-blocked and quarantine
    // detail paragraphs when recovery is active.
    await screen.findByText(/Configuration recovery mode/);
    expect(
      screen.getByText("Portier started without your saved configuration.")
    ).toBeInTheDocument();
    expect(screen.getByText(/Rule changes are blocked/)).toBeInTheDocument();
    expect(
      screen.getByText("C:/ProgramData/Portier/rules.json.quarantine")
    ).toBeInTheDocument();
  });
});

describe("App connections view", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  it("Live Connections nav item renders the connections view", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);

    render(<App />);
    await screen.findByText(/No forwarding rules yet/);

    // Navigating to the connections view mounts LiveConnectionsView (App line 425).
    await userEvent.click(screen.getByRole("button", { name: /Live Connections/ }));

    // The view's tablist and TCP tab confirm LiveConnectionsView rendered.
    expect(
      await screen.findByRole("tablist", { name: "Connection views" })
    ).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /^TCP/ })).toBeInTheDocument();
  });
});

describe("App showForm keydown non-Escape", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  it("a non-Escape keydown leaves the open Add Rule drawer untouched", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);

    render(<App />);
    await screen.findByText(/No forwarding rules yet/);

    // Open the drawer so the showForm keydown listener (App line 94) is attached.
    await userEvent.click(screen.getByRole("button", { name: /\+ Add Rule/ }));
    expect(
      screen.getByRole("complementary", { name: "Add Forward Rule" })
    ).toBeInTheDocument();

    // A non-Escape key takes the FALSE arm of `if (e.key === "Escape")`: no-op,
    // so the drawer stays open.
    fireEvent.keyDown(document, { key: "a" });
    expect(
      screen.getByRole("complementary", { name: "Add Forward Rule" })
    ).toBeInTheDocument();
  });

  it("pressing Escape closes the open Add Rule drawer", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([]);

    render(<App />);
    await screen.findByText(/No forwarding rules yet/);

    await userEvent.click(screen.getByRole("button", { name: /\+ Add Rule/ }));
    expect(
      screen.getByRole("complementary", { name: "Add Forward Rule" })
    ).toBeInTheDocument();

    // Escape takes the TRUE arm of `if (e.key === "Escape")` → handleCancel closes the drawer.
    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() =>
      expect(
        screen.queryByRole("complementary", { name: "Add Forward Rule" })
      ).not.toBeInTheDocument()
    );
  });
});

describe("App non-Error rejection error message", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
  });

  it("falls back to 'Unexpected error.' when a handler rejects with a non-Error", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([sampleRule]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([stoppedStatus]);
    // A non-Error rejection makes errorMessage take its `: "Unexpected error."`
    // arm (App line 459).
    vi.mocked(portierApi.deleteForwardRule).mockRejectedValue("plain string");

    render(<App />);
    await screen.findByText("Test Rule");

    await userEvent.click(screen.getByRole("button", { name: "More actions for Test Rule" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Delete" }));
    await userEvent.click(screen.getByRole("button", { name: "Confirm" }));

    await screen.findByText("Unexpected error.");
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });
});
