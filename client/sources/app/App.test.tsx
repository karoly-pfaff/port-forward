import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ForwardStatus } from "@portier/shared";
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
  importConfig: vi.fn()
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

    await userEvent.click(screen.getByRole("button", { name: "Edit" }));
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

    await userEvent.click(screen.getByRole("button", { name: "Edit" }));

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

    await userEvent.click(screen.getByRole("button", { name: "Diagnose" }));

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

    await userEvent.click(screen.getByRole("button", { name: "Diagnose" }));

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

    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
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
