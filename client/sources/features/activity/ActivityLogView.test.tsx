import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ActivityEvent, ForwardRuleResponse } from "@portier/shared";
import { ActivityLogView } from "./ActivityLogView.js";
import * as portierApi from "../../api/portierApi.js";

vi.mock("../../api/portierApi.js", () => ({
  fetchActivity: vi.fn(),
  clearActivity: vi.fn()
}));

const sampleEvent: ActivityEvent = {
  id: "ev1",
  timestamp: new Date().toISOString(),
  type: "rule.started",
  severity: "success",
  ruleId: "r1",
  ruleName: "My Rule",
  protocol: "tcp",
  message: 'Rule "My Rule" started.'
};

const sampleRule: ForwardRuleResponse = {
  id: "r1",
  name: "My Rule",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 48001,
  targetHost: "127.0.0.1",
  targetPort: 3000,
  enabled: true,
  advisories: []
};

describe("ActivityLogView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the section title and subtitle", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<ActivityLogView />);

    expect(screen.getByText("Activity")).toBeInTheDocument();
    expect(screen.getByText(/Recent forwarding and rule events/)).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());
  });

  it("shows empty state when no events are returned", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<ActivityLogView />);

    await screen.findByText(/No activity yet/);
    expect(screen.getByText(/Start a rule or send traffic/)).toBeInTheDocument();
  });

  it("renders activity events", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([sampleEvent]);

    render(<ActivityLogView />);

    await screen.findByText(/Rule "My Rule" started/);
    expect(screen.getByText("My Rule")).toBeInTheDocument();
    // "Success" also appears in the severity filter dropdown, so use getAllByText
    const successLabels = screen.getAllByText("Success");
    expect(successLabels.length).toBeGreaterThanOrEqual(1);
  });

  it("renders severity badges for different severities", async () => {
    const events: ActivityEvent[] = [
      { ...sampleEvent, id: "e1", severity: "info", message: "Info event." },
      { ...sampleEvent, id: "e2", severity: "error", message: "Error event." },
      { ...sampleEvent, id: "e3", severity: "warning", message: "Warning event." }
    ];
    vi.mocked(portierApi.fetchActivity).mockResolvedValue(events);

    render(<ActivityLogView />);

    await screen.findByText("Info event.");
    // Severity badges (text appears in badges; "Info"/"Warning" also appear in the filter select)
    const infoBadges = screen.getAllByText("Info");
    expect(infoBadges.length).toBeGreaterThanOrEqual(1);
    const errorBadges = screen.getAllByText("Error");
    expect(errorBadges.length).toBeGreaterThanOrEqual(1);
    const warningBadges = screen.getAllByText("Warning");
    expect(warningBadges.length).toBeGreaterThanOrEqual(1);
  });

  it("shows event count in footer", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([sampleEvent]);

    render(<ActivityLogView />);

    await waitFor(() => expect(screen.getByText("1 event shown")).toBeInTheDocument());
  });

  it("refresh button calls fetchActivity again", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<ActivityLogView />);
    await screen.findByText(/No activity yet/);

    // Disable auto-refresh so the refresh button is enabled
    await userEvent.click(screen.getByTitle("Auto-refresh"));

    vi.mocked(portierApi.fetchActivity).mockResolvedValue([sampleEvent]);
    await userEvent.click(screen.getByRole("button", { name: "Refresh" }));

    await screen.findByText(/Rule "My Rule" started/);
    expect(portierApi.fetchActivity).toHaveBeenCalledTimes(2);
  });

  it("shows an error when fetchActivity rejects", async () => {
    vi.mocked(portierApi.fetchActivity).mockRejectedValue(new Error("Network error"));

    render(<ActivityLogView />);

    await screen.findByText("Network error");
  });

  it("renders the TCP protocol badge", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([sampleEvent]);

    render(<ActivityLogView />);

    await screen.findByText("TCP");
  });

  it("shows type filter select with all-types option", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<ActivityLogView />);

    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());
    expect(screen.getByRole("combobox", { name: "Filter by type" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "All types" })).toBeInTheDocument();
  });

  it("passes type param to fetchActivity when type filter is changed", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<ActivityLogView />);
    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());

    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Filter by type" }),
      "rule.started"
    );

    await waitFor(() => {
      expect(portierApi.fetchActivity).toHaveBeenCalledWith(
        expect.objectContaining({ type: "rule.started" })
      );
    });
  });

  it("shows filter banner with rule name when ruleId prop is set", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<ActivityLogView rules={[sampleRule]} ruleId="r1" />);

    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());
    expect(screen.getByRole("status")).toBeInTheDocument();
    // Banner contains the rule name in a <strong> element
    expect(screen.getByRole("status").textContent).toContain("My Rule");
  });

  it("shows filter banner with raw id when rule is not in rules list", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<ActivityLogView rules={[]} ruleId="unknown-id" />);

    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());
    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(screen.getByText("unknown-id")).toBeInTheDocument();
  });

  it("calls onClearRuleFilter and hides banner when clear is clicked in banner", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
    const onClearRuleFilter = vi.fn();

    render(<ActivityLogView rules={[sampleRule]} ruleId="r1" onClearRuleFilter={onClearRuleFilter} />);

    await waitFor(() => expect(screen.getByRole("status")).toBeInTheDocument());

    const bannerClearButtons = screen.getAllByRole("button", { name: /clear/i });
    // Click the first "Clear" button in the banner (not "Clear filters")
    const bannerClear = bannerClearButtons.find((btn) => btn.textContent === "Clear");
    expect(bannerClear).toBeTruthy();
    await userEvent.click(bannerClear!);

    expect(onClearRuleFilter).toHaveBeenCalled();
  });

  it("shows Clear filters button when any filter is active", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<ActivityLogView rules={[sampleRule]} ruleId="r1" />);

    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Clear all filters" })).toBeInTheDocument();
  });

  it("clear activity button calls clearActivity and empties events", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([sampleEvent]);
    vi.mocked(portierApi.clearActivity).mockResolvedValue(undefined);

    render(<ActivityLogView />);
    await screen.findByText(/Rule "My Rule" started/);

    await userEvent.click(screen.getByRole("button", { name: "Clear activity log" }));

    await waitFor(() => expect(portierApi.clearActivity).toHaveBeenCalled());
    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());
  });

  it("shows error when clearActivity rejects", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([sampleEvent]);
    vi.mocked(portierApi.clearActivity).mockRejectedValue(new Error("Clear failed"));

    render(<ActivityLogView />);
    await screen.findByText(/Rule "My Rule" started/);

    await userEvent.click(screen.getByRole("button", { name: "Clear activity log" }));

    await screen.findByText("Clear failed");
  });

  it("export button is disabled when no events are loaded", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<ActivityLogView />);

    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Export activity as JSON" })).toBeDisabled();
  });

  it("export button is enabled when events are present", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([sampleEvent]);

    render(<ActivityLogView />);

    await screen.findByText(/Rule "My Rule" started/);
    expect(screen.getByRole("button", { name: "Export activity as JSON" })).not.toBeDisabled();
  });

  it("shows throttle note", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<ActivityLogView />);

    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());
    expect(screen.getByText(/High-frequency packet events/)).toBeInTheDocument();
  });
});
