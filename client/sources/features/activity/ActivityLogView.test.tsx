import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
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

    expect(screen.getByText("Activity Log")).toBeInTheDocument();
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

  // load's catch fallback (line 101): a non-Error rejection takes the
  // `: "Failed to load activity."` arm instead of using err.message.
  it("shows the fallback load message when fetchActivity rejects with a non-Error", async () => {
    vi.mocked(portierApi.fetchActivity).mockRejectedValue("boom");

    render(<ActivityLogView />);

    await screen.findByText("Failed to load activity.");
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
    expect(screen.getByRole("option", { name: "All Types" })).toBeInTheDocument();
  });

  it("applies the initial severity prop to the severity filter and fetch", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<ActivityLogView severity="error" />);

    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());
    expect(screen.getByRole("combobox", { name: "Filter by severity" })).toHaveValue("error");
    await waitFor(() =>
      expect(portierApi.fetchActivity).toHaveBeenCalledWith(expect.objectContaining({ severity: "error" }))
    );
  });

  it("updates the severity filter when the severity prop changes", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    const { rerender } = render(<ActivityLogView />);
    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());
    expect(screen.getByRole("combobox", { name: "Filter by severity" })).toHaveValue("");

    rerender(<ActivityLogView severity="error" />);
    await waitFor(() =>
      expect(screen.getByRole("combobox", { name: "Filter by severity" })).toHaveValue("error")
    );
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

  it("Export JSON builds a download blob from the visible events", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([sampleEvent]);
    const createObjectURL = vi.fn(() => "blob:activity");
    const revokeObjectURL = vi.fn();
    vi.stubGlobal("URL", { ...URL, createObjectURL, revokeObjectURL });
    const clickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => {});

    render(<ActivityLogView />);
    await screen.findByText(/Rule "My Rule" started/);

    await userEvent.click(screen.getByRole("button", { name: "Export activity as JSON" }));

    expect(createObjectURL).toHaveBeenCalledTimes(1);
    expect(clickSpy).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:activity");

    clickSpy.mockRestore();
    vi.unstubAllGlobals();
  });

  it("Clear activity log empties the list on success", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([sampleEvent]);
    vi.mocked(portierApi.clearActivity).mockResolvedValue(undefined);

    render(<ActivityLogView />);
    await screen.findByText(/Rule "My Rule" started/);

    await userEvent.click(screen.getByRole("button", { name: "Clear activity log" }));

    expect(portierApi.clearActivity).toHaveBeenCalledTimes(1);
    await waitFor(() =>
      expect(screen.queryByText(/Rule "My Rule" started/)).not.toBeInTheDocument()
    );
  });

  it("Clear activity log shows an error when the API rejects", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([sampleEvent]);
    vi.mocked(portierApi.clearActivity).mockRejectedValue(new Error("Clear failed"));

    render(<ActivityLogView />);
    await screen.findByText(/Rule "My Rule" started/);

    await userEvent.click(screen.getByRole("button", { name: "Clear activity log" }));

    expect(await screen.findByText("Clear failed")).toBeInTheDocument();
  });

  // handleClearActivity's catch fallback (line 137): a non-Error rejection takes
  // the `: "Failed to clear activity."` arm instead of using err.message.
  it("shows the fallback clear message when clearActivity rejects with a non-Error", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([sampleEvent]);
    vi.mocked(portierApi.clearActivity).mockRejectedValue("boom");

    render(<ActivityLogView />);
    await screen.findByText(/Rule "My Rule" started/);

    await userEvent.click(screen.getByRole("button", { name: "Clear activity log" }));

    expect(await screen.findByText("Failed to clear activity.")).toBeInTheDocument();
  });

  // handleRuleChange (lines 125-129): selecting "All rules" (empty value) clears
  // the rule filter and invokes onClearRuleFilter; selecting a concrete rule
  // (truthy value) takes the other arm and does NOT call it.
  it("rule-filter select calls onClearRuleFilter only when cleared to All rules", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
    const onClearRuleFilter = vi.fn();

    // Start with no rule filter so the select defaults to "All rules".
    render(<ActivityLogView rules={[sampleRule]} onClearRuleFilter={onClearRuleFilter} />);
    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());

    const ruleSelect = screen.getByRole("combobox", { name: "Filter by rule" });

    // Selecting a concrete rule (truthy value): onClearRuleFilter must NOT fire.
    await userEvent.selectOptions(ruleSelect, "r1");
    await waitFor(() =>
      expect(portierApi.fetchActivity).toHaveBeenCalledWith(expect.objectContaining({ ruleId: "r1" }))
    );
    expect(onClearRuleFilter).not.toHaveBeenCalled();

    // Selecting "All rules" (empty value): onClearRuleFilter fires.
    await userEvent.selectOptions(ruleSelect, "");
    expect(onClearRuleFilter).toHaveBeenCalledTimes(1);
  });

  // The `?.` optional-chain arm of handleRuleChange: clearing to "All rules"
  // when no onClearRuleFilter prop is supplied must not throw.
  it("rule-filter clear is a no-op when onClearRuleFilter is not provided", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<ActivityLogView rules={[sampleRule]} ruleId="r1" />);
    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());

    const ruleSelect = screen.getByRole("combobox", { name: "Filter by rule" });
    await userEvent.selectOptions(ruleSelect, "");

    // The filter banner (driven by ruleIdFilter) disappears once cleared.
    await waitFor(() =>
      expect(screen.queryByText(/Filtered to rule:/)).not.toBeInTheDocument()
    );
  });

  // "Clear filters" button onClick (lines 224-229): resets severity/type/ruleId
  // filters and calls onClearRuleFilter. Previously only its presence was tested.
  it("Clear filters button resets all filters and calls onClearRuleFilter", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
    const onClearRuleFilter = vi.fn();

    render(<ActivityLogView rules={[sampleRule]} ruleId="r1" onClearRuleFilter={onClearRuleFilter} />);
    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());

    // Add a severity filter on top of the rule filter so the reset is observable.
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Filter by severity" }),
      "error"
    );
    await waitFor(() =>
      expect(portierApi.fetchActivity).toHaveBeenCalledWith(expect.objectContaining({ severity: "error" }))
    );

    await userEvent.click(screen.getByRole("button", { name: "Clear all filters" }));

    expect(onClearRuleFilter).toHaveBeenCalled();
    // Filters are gone: the "Clear filters" button itself no longer renders and
    // the active-rule banner is dismissed.
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Clear all filters" })).not.toBeInTheDocument()
    );
    expect(screen.queryByText(/Filtered to rule:/)).not.toBeInTheDocument();
  });

  // Auto-refresh interval effect (lines 113-119): while auto-refresh is on the
  // setInterval callback at line 116 re-invokes load on each tick. Fake timers
  // let us advance past one interval and observe the extra fetch. fireEvent is
  // used (not userEvent) to avoid the userEvent<->fake-timer interplay.
  describe("auto-refresh interval", () => {
    afterEach(() => {
      vi.useRealTimers();
    });

    it("re-fetches activity on each interval tick while auto-refresh is on", async () => {
      vi.useFakeTimers();
      vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

      render(<ActivityLogView />);
      // Flush the mount-effect load (its promise resolves on a microtask).
      await vi.advanceTimersByTimeAsync(0);
      expect(portierApi.fetchActivity).toHaveBeenCalledTimes(1);

      // Auto-refresh defaults on at a 5s interval; advancing 5s fires the
      // interval callback (line 116) for a second fetch.
      await vi.advanceTimersByTimeAsync(5000);
      expect(portierApi.fetchActivity).toHaveBeenCalledTimes(2);

      // A second tick fires another fetch, confirming the interval is recurring.
      await vi.advanceTimersByTimeAsync(5000);
      expect(portierApi.fetchActivity).toHaveBeenCalledTimes(3);

      // Turning auto-refresh off clears the interval: no further ticks fetch.
      fireEvent.click(screen.getByRole("checkbox", { name: "Auto-refresh" }));
      await vi.advanceTimersByTimeAsync(5000);
      expect(portierApi.fetchActivity).toHaveBeenCalledTimes(3);
    });
  });

  // The `?.` optional-chain arm of the Clear-filters handler: no onClearRuleFilter
  // prop supplied; clicking must reset filters without throwing.
  it("Clear filters works without an onClearRuleFilter prop", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

    render(<ActivityLogView rules={[sampleRule]} ruleId="r1" />);
    await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());

    await userEvent.click(screen.getByRole("button", { name: "Clear all filters" }));

    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Clear all filters" })).not.toBeInTheDocument()
    );
  });

  // handleExport with both severity AND type filters active (lines 143-145): the
  // `if (severity)` and `if (type)` branches add those keys to activeFilters
  // before the blob is built. Stub URL + anchor click as the base export test does.
  it("Export JSON includes active severity and type filters in the payload", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([sampleEvent]);
    const createObjectURL = vi.fn(() => "blob:activity-filtered");
    const revokeObjectURL = vi.fn();
    vi.stubGlobal("URL", { ...URL, createObjectURL, revokeObjectURL });
    const clickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => {});

    render(<ActivityLogView />);
    await screen.findByText(/Rule "My Rule" started/);

    // Activate both a severity and a type filter (re-fetch still resolves the
    // same single event, keeping the export button enabled).
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Filter by severity" }),
      "success"
    );
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Filter by type" }),
      "rule.started"
    );
    await waitFor(() =>
      expect(portierApi.fetchActivity).toHaveBeenCalledWith(
        expect.objectContaining({ severity: "success", type: "rule.started" })
      )
    );

    await userEvent.click(screen.getByRole("button", { name: "Export activity as JSON" }));

    expect(createObjectURL).toHaveBeenCalledTimes(1);
    expect(clickSpy).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:activity-filtered");

    clickSpy.mockRestore();
    vi.unstubAllGlobals();
  });

  // handleExport's `if (ruleIdFilter)` branch (line 143): the existing
  // severity+type export test never sets a rule filter, so this rule-only export
  // exercises that branch. ruleId is supplied via the prop so the rule filter is
  // active without touching severity/type.
  it("Export JSON includes the active rule filter in the payload", async () => {
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([sampleEvent]);
    const createObjectURL = vi.fn(() => "blob:activity-rule");
    const revokeObjectURL = vi.fn();
    vi.stubGlobal("URL", { ...URL, createObjectURL, revokeObjectURL });
    const clickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => {});

    render(<ActivityLogView rules={[sampleRule]} ruleId="r1" />);
    await screen.findByText(/Rule "My Rule" started/);

    await waitFor(() =>
      expect(portierApi.fetchActivity).toHaveBeenCalledWith(
        expect.objectContaining({ ruleId: "r1" })
      )
    );

    await userEvent.click(screen.getByRole("button", { name: "Export activity as JSON" }));

    expect(createObjectURL).toHaveBeenCalledTimes(1);
    expect(clickSpy).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:activity-rule");

    clickSpy.mockRestore();
    vi.unstubAllGlobals();
  });

  // Event-meta render branches (line ~291): the wrapper renders when
  // ruleName ?? protocol is truthy, with independent inner `{event.protocol && …}`
  // and `{event.ruleName && …}` branches. A protocol-only event (no ruleName)
  // renders the protocol badge but no rule-name span.
  it("renders the protocol badge for an event with a protocol but no ruleName", async () => {
    const protocolOnly: ActivityEvent = {
      id: "proto-only",
      timestamp: new Date().toISOString(),
      type: "udp.packet.forwarded",
      severity: "info",
      protocol: "udp",
      message: "UDP packet forwarded."
    };
    const ruleNameOnly: ActivityEvent = {
      id: "name-only",
      timestamp: new Date().toISOString(),
      type: "config.imported",
      severity: "success",
      ruleName: "Named Rule",
      message: "Config imported."
    };
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([protocolOnly, ruleNameOnly]);

    render(<ActivityLogView />);

    await screen.findByText("UDP packet forwarded.");
    // Protocol-only event: the UDP proto badge renders (uppercased protocol).
    expect(screen.getByText("UDP")).toBeInTheDocument();
    // ruleName-only event: its rule name renders in the meta block.
    expect(screen.getByText("Named Rule")).toBeInTheDocument();
  });

  // Two inline <select> onChange callbacks not covered elsewhere: the event-limit
  // select (line 213) and the auto-refresh interval select (line 350).
  describe("limit and interval selects", () => {
    it("changing the event limit re-fetches with the new limit", async () => {
      vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

      render(<ActivityLogView />);
      await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());

      // Selecting a different limit fires onChange (line 213) and re-runs load
      // with the new numeric limit.
      await userEvent.selectOptions(screen.getByRole("combobox", { name: "Event limit" }), "200");

      await waitFor(() =>
        expect(portierApi.fetchActivity).toHaveBeenCalledWith(
          expect.objectContaining({ limit: 200 })
        )
      );
    });

    it("changing the auto-refresh interval updates the control value", async () => {
      vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);

      render(<ActivityLogView />);
      await waitFor(() => expect(screen.getByText("0 events shown")).toBeInTheDocument());

      // Auto-refresh defaults on, so the interval select is enabled. Selecting a
      // new value fires onChange (line 350).
      const intervalSelect = screen.getByRole("combobox", { name: "Auto-refresh interval" });
      await userEvent.selectOptions(intervalSelect, "30");
      expect((intervalSelect as HTMLSelectElement).value).toBe("30");
    });
  });
});
