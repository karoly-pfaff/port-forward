import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ActivityEvent } from "@portier/shared";
import { ActivityLogView } from "./ActivityLogView.js";
import * as portierApi from "../../api/portierApi.js";

vi.mock("../../api/portierApi.js", () => ({
  fetchActivity: vi.fn()
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
});
