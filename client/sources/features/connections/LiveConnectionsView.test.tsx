import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { LiveConnectionsResponse, RuleLiveSummary, TcpConnectionInfo, UdpSessionInfo } from "@portier/shared";
import { LiveConnectionsView } from "./LiveConnectionsView.js";
import * as portierApi from "../../api/portierApi.js";

vi.mock("../../api/portierApi.js", () => ({
  fetchLiveConnections: vi.fn(),
}));

const tcpConn: TcpConnectionInfo = {
  id: "tcp-1",
  ruleId: "rule-1",
  ruleName: "Web Dev",
  protocol: "tcp",
  clientAddress: "127.0.0.1",
  clientPort: 55001,
  targetAddress: "127.0.0.1",
  targetPort: 3000,
  startedAt: "2026-01-01T12:00:00.000Z",
  durationMs: 65000,
  bytesIn: 1024,
  bytesOut: 2048,
  status: "active",
};

const udpSession: UdpSessionInfo = {
  id: "udp-1",
  ruleId: "rule-2",
  ruleName: "Game Server",
  protocol: "udp",
  mode: "bidirectional-last-client",
  clientAddress: "192.168.1.5",
  clientPort: 44001,
  targetAddress: "192.168.1.10",
  targetPort: 27015,
  startedAt: "2026-01-01T12:00:00.000Z",
  lastSeenAt: "2026-01-01T12:01:00.000Z",
  idleMs: 2000,
  packetsIn: 100,
  packetsOut: 95,
  bytesIn: 5120,
  bytesOut: 4096,
  status: "active",
};

const idleUdpSession: UdpSessionInfo = {
  ...udpSession,
  id: "udp-2",
  status: "idle",
};

const tcpSummary: RuleLiveSummary = {
  ruleId: "rule-1",
  ruleName: "Web Dev",
  protocol: "tcp",
  activeTcpConnections: 1,
  activeUdpSessions: 0,
  bytesIn: 1024,
  bytesOut: 2048,
  packetsIn: 0,
  packetsOut: 0,
  lastTrafficAt: "2026-01-01T12:01:00.000Z",
};

const idleSummary: RuleLiveSummary = {
  ruleId: "rule-3",
  ruleName: "Idle Rule",
  protocol: "tcp",
  activeTcpConnections: 0,
  activeUdpSessions: 0,
  bytesIn: 512,
  bytesOut: 256,
  packetsIn: 5,
  packetsOut: 3,
  lastTrafficAt: null,
};

const emptyData: LiveConnectionsResponse = {
  generatedAt: "2026-01-01T12:01:05.000Z",
  tcpConnections: [],
  udpSessions: [],
  ruleSummaries: [],
};

const fullData: LiveConnectionsResponse = {
  generatedAt: "2026-01-01T12:01:05.000Z",
  tcpConnections: [tcpConn],
  udpSessions: [udpSession],
  ruleSummaries: [tcpSummary, idleSummary],
};

describe("LiveConnectionsView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches connections on mount", async () => {
    vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(emptyData);
    render(<LiveConnectionsView />);
    await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalledTimes(1));
  });

  it("shows loading state before fetch resolves", () => {
    let resolve!: (v: LiveConnectionsResponse) => void;
    vi.mocked(portierApi.fetchLiveConnections).mockReturnValueOnce(
      new Promise((r) => { resolve = r; })
    );
    render(<LiveConnectionsView />);
    expect(screen.getByText("Loading connections…")).toBeInTheDocument();
    resolve(emptyData);
  });

  it("shows error when fetch rejects", async () => {
    vi.mocked(portierApi.fetchLiveConnections).mockRejectedValue(new Error("Network error"));
    render(<LiveConnectionsView />);
    await screen.findByText("Network error");
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("displays generatedAt timestamp after load", async () => {
    vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(emptyData);
    render(<LiveConnectionsView />);
    await waitFor(() => expect(screen.getByText(/Updated at/)).toBeInTheDocument());
  });

  describe("summary strip", () => {
    it("shows TCP connection count", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());
      expect(screen.getByText("TCP Connections")).toBeInTheDocument();
    });

    it("shows UDP session count", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());
      expect(screen.getByText("UDP Sessions")).toBeInTheDocument();
    });

    it("shows active rules count and total traffic label", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());
      expect(screen.getByText("Active Rules")).toBeInTheDocument();
      expect(screen.getByText("Total Traffic")).toBeInTheDocument();
    });

    it("shows 0 for active rules when no traffic", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(emptyData);
      render(<LiveConnectionsView />);
      const strip = await screen.findByLabelText("Live connections summary");
      expect(strip).toBeInTheDocument();
    });
  });

  describe("tabs", () => {
    it("renders all three tab buttons", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(emptyData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());
      expect(screen.getByRole("tab", { name: /TCP/ })).toBeInTheDocument();
      expect(screen.getByRole("tab", { name: /UDP/ })).toBeInTheDocument();
      expect(screen.getByRole("tab", { name: /Rule Summary/ })).toBeInTheDocument();
    });

    it("TCP tab is active by default", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(emptyData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());
      expect(screen.getByRole("tab", { name: /TCP/ })).toHaveAttribute("aria-selected", "true");
    });

    it("switches to UDP tab on click", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());
      await userEvent.click(screen.getByRole("tab", { name: /UDP/ }));
      expect(screen.getByRole("tab", { name: /UDP/ })).toHaveAttribute("aria-selected", "true");
    });

    it("switches to Rule Summary tab on click", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());
      await userEvent.click(screen.getByRole("tab", { name: /Rule Summary/ }));
      expect(screen.getByRole("tab", { name: /Rule Summary/ })).toHaveAttribute("aria-selected", "true");
    });
  });

  describe("TCP tab", () => {
    it("shows empty state when no TCP connections", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(emptyData);
      render(<LiveConnectionsView />);
      await screen.findByText("No active TCP connections.");
    });

    it("renders TCP connection row with rule name", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      // "Web Dev" appears in both the rule filter dropdown and the table cell
      await screen.findByRole("table", { name: "TCP connections" });
      const table = screen.getByRole("table", { name: "TCP connections" });
      expect(within(table).getByText("Web Dev")).toBeInTheDocument();
    });

    it("renders client endpoint", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await screen.findByText("127.0.0.1:55001");
    });

    it("renders target endpoint", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await screen.findByText("127.0.0.1:3000");
    });

    it("renders formatted duration", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await screen.findByText("1m 05s");
    });

    it("renders formatted bytes in and out", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await screen.findByText("1.0 KB");
      expect(screen.getByText("2.0 KB")).toBeInTheDocument();
    });

    it("renders Active status badge", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      // "Active" also appears as a status filter option; confirm at least 2 elements
      await waitFor(() => expect(screen.getAllByText("Active").length).toBeGreaterThanOrEqual(2));
    });

    it("renders TCP table with accessible label", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await screen.findByRole("table", { name: "TCP connections" });
    });

    it("shows 1 connection in footer", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await screen.findByText("1 connection");
    });
  });

  describe("UDP tab", () => {
    async function renderAndSwitchToUdp(data: LiveConnectionsResponse): Promise<void> {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(data);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());
      await userEvent.click(screen.getByRole("tab", { name: /UDP/ }));
    }

    it("shows empty state when no UDP sessions", async () => {
      await renderAndSwitchToUdp(emptyData);
      expect(screen.getByText("No active or recent UDP sessions.")).toBeInTheDocument();
    });

    it("renders UDP session row with rule name", async () => {
      await renderAndSwitchToUdp(fullData);
      expect(screen.getByText("Game Server")).toBeInTheDocument();
    });

    it("renders mode label", async () => {
      await renderAndSwitchToUdp(fullData);
      expect(screen.getByText("Bidir – last")).toBeInTheDocument();
    });

    it("renders client and target endpoints", async () => {
      await renderAndSwitchToUdp(fullData);
      expect(screen.getByText("192.168.1.5:44001")).toBeInTheDocument();
      expect(screen.getByText("192.168.1.10:27015")).toBeInTheDocument();
    });

    it("renders packet counts", async () => {
      await renderAndSwitchToUdp(fullData);
      expect(screen.getByText("100 / 95")).toBeInTheDocument();
    });

    it("renders Active status badge for active session", async () => {
      await renderAndSwitchToUdp(fullData);
      // "Active" also appears as a status filter option
      expect(screen.getAllByText("Active").length).toBeGreaterThanOrEqual(2);
    });

    it("renders Idle status badge for idle session", async () => {
      const data: LiveConnectionsResponse = { ...emptyData, udpSessions: [idleUdpSession] };
      await renderAndSwitchToUdp(data);
      // "Idle" also appears as a status filter option
      expect(screen.getAllByText("Idle").length).toBeGreaterThanOrEqual(2);
    });

    it("renders UDP table with accessible label", async () => {
      await renderAndSwitchToUdp(fullData);
      expect(screen.getByRole("table", { name: "UDP sessions" })).toBeInTheDocument();
    });

    it("shows session count in footer", async () => {
      await renderAndSwitchToUdp(fullData);
      expect(screen.getByText("1 session")).toBeInTheDocument();
    });
  });

  describe("Rule Summary tab", () => {
    async function renderAndSwitchToSummary(data: LiveConnectionsResponse): Promise<void> {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(data);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());
      await userEvent.click(screen.getByRole("tab", { name: /Rule Summary/ }));
    }

    it("shows empty state when no rule summaries", async () => {
      await renderAndSwitchToSummary(emptyData);
      expect(screen.getByText("No rule summaries available.")).toBeInTheDocument();
    });

    it("renders rule name and protocol badge", async () => {
      await renderAndSwitchToSummary(fullData);
      expect(screen.getAllByText("Web Dev").length).toBeGreaterThanOrEqual(1);
      // "TCP" appears in both the table column header and the protocol badge
      expect(screen.getAllByText("TCP").length).toBeGreaterThanOrEqual(2);
    });

    it("renders lastTrafficAt null as Never", async () => {
      await renderAndSwitchToSummary(fullData);
      expect(screen.getByText("Never")).toBeInTheDocument();
    });

    it("renders lastTrafficAt non-null as a formatted time", async () => {
      await renderAndSwitchToSummary(fullData);
      const nevers = screen.getAllByText("Never");
      expect(nevers).toHaveLength(1);
    });

    it("sorts active rules before idle rules", async () => {
      const activeSummary: RuleLiveSummary = { ...tcpSummary, ruleId: "r-active", ruleName: "ZZZ Active", activeTcpConnections: 2 };
      const data: LiveConnectionsResponse = {
        ...emptyData,
        ruleSummaries: [idleSummary, activeSummary],
      };
      await renderAndSwitchToSummary(data);
      const rows = screen.getAllByRole("row");
      const names = rows.slice(1).map((r) => r.textContent ?? "");
      const activeIdx = names.findIndex((t) => t.includes("ZZZ Active"));
      const idleIdx = names.findIndex((t) => t.includes("Idle Rule"));
      expect(activeIdx).toBeLessThan(idleIdx);
    });

    it("renders rule summary table with accessible label", async () => {
      await renderAndSwitchToSummary(fullData);
      expect(screen.getByRole("table", { name: "Rule summaries" })).toBeInTheDocument();
    });

    it("shows rule count in footer", async () => {
      await renderAndSwitchToSummary(fullData);
      expect(screen.getByText("2 rules")).toBeInTheDocument();
    });
  });

  describe("filters", () => {
    it("protocol filter TCP hides UDP sessions from summary", async () => {
      const udpSummary: RuleLiveSummary = {
        ...tcpSummary,
        ruleId: "rule-2",
        ruleName: "UDP Rule",
        protocol: "udp",
        activeTcpConnections: 0,
        activeUdpSessions: 1,
      };
      const data: LiveConnectionsResponse = {
        ...emptyData,
        ruleSummaries: [tcpSummary, udpSummary],
      };
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(data);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());

      await userEvent.click(screen.getByRole("tab", { name: /Rule Summary/ }));
      await userEvent.selectOptions(screen.getByRole("combobox", { name: "Filter by protocol" }), "tcp");

      // Use within the table to avoid matching the rule filter dropdown options
      const table = screen.getByRole("table", { name: "Rule summaries" });
      expect(within(table).getByText("Web Dev")).toBeInTheDocument();
      expect(within(table).queryByText("UDP Rule")).not.toBeInTheDocument();
    });

    it("status filter idle hides active connections from TCP tab", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await screen.findByRole("table", { name: "TCP connections" });

      await userEvent.selectOptions(screen.getByRole("combobox", { name: "Filter by status" }), "idle");

      expect(screen.getByText("No active TCP connections.")).toBeInTheDocument();
    });

    it("status filter active hides idle UDP sessions", async () => {
      const data: LiveConnectionsResponse = {
        ...emptyData,
        udpSessions: [udpSession, idleUdpSession],
        ruleSummaries: [tcpSummary],
      };
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(data);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());

      await userEvent.click(screen.getByRole("tab", { name: /UDP/ }));
      await userEvent.selectOptions(screen.getByRole("combobox", { name: "Filter by status" }), "active");

      expect(screen.getByText("1 session")).toBeInTheDocument();
    });

    it("rule filter shows only matching connections", async () => {
      const secondConn: TcpConnectionInfo = {
        ...tcpConn,
        id: "tcp-2",
        ruleId: "rule-99",
        ruleName: "Other Rule",
      };
      const data: LiveConnectionsResponse = {
        ...emptyData,
        tcpConnections: [tcpConn, secondConn],
        ruleSummaries: [tcpSummary, { ...tcpSummary, ruleId: "rule-99", ruleName: "Other Rule" }],
      };
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(data);
      render(<LiveConnectionsView />);
      await screen.findByRole("table", { name: "TCP connections" });

      await userEvent.selectOptions(screen.getByRole("combobox", { name: "Filter by rule" }), "rule-1");

      // Use within the table to avoid matching the rule filter dropdown options
      const table = screen.getByRole("table", { name: "TCP connections" });
      expect(within(table).getByText("Web Dev")).toBeInTheDocument();
      expect(within(table).queryByText("Other Rule")).not.toBeInTheDocument();
    });

    it("shows Clear filters button when a filter is active", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());

      await userEvent.selectOptions(screen.getByRole("combobox", { name: "Filter by protocol" }), "tcp");

      expect(screen.getByRole("button", { name: "Clear all filters" })).toBeInTheDocument();
    });

    it("Clear filters button resets all filters", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());

      await userEvent.selectOptions(screen.getByRole("combobox", { name: "Filter by protocol" }), "tcp");
      const clearBtn = screen.getByRole("button", { name: "Clear all filters" });
      await userEvent.click(clearBtn);

      expect(screen.queryByRole("button", { name: "Clear all filters" })).not.toBeInTheDocument();
    });
  });

  describe("actions", () => {
    it("refresh button calls fetchLiveConnections again", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(emptyData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalledTimes(1));

      await userEvent.click(screen.getByTitle("Auto-refresh"));

      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      await userEvent.click(screen.getByRole("button", { name: "Refresh" }));

      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalledTimes(2));
    });

    it("auto-refresh toggle checkbox is present and checked by default", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(emptyData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());
      const toggle = screen.getByRole("checkbox", { name: "Auto-refresh" });
      expect(toggle).toBeInTheDocument();
      expect(toggle).toBeChecked();
    });

    it("auto-refresh interval select is present", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(emptyData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());
      expect(screen.getByRole("combobox", { name: "Auto-refresh interval" })).toBeInTheDocument();
    });

    it("auto-refresh interval select is disabled when auto-refresh is off", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(emptyData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());

      await userEvent.click(screen.getByTitle("Auto-refresh"));

      expect(screen.getByRole("combobox", { name: "Auto-refresh interval" })).toBeDisabled();
    });
  });

  describe("footer count", () => {
    it("shows 'Loading…' in footer while loading", () => {
      let resolve!: (v: LiveConnectionsResponse) => void;
      vi.mocked(portierApi.fetchLiveConnections).mockReturnValueOnce(
        new Promise((r) => { resolve = r; })
      );
      render(<LiveConnectionsView />);
      expect(screen.getByText("Loading…")).toBeInTheDocument();
      resolve(emptyData);
    });

    it("shows singular 'connection' for 1 TCP connection", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await screen.findByText("1 connection");
    });

    it("shows plural 'sessions' for multiple UDP sessions", async () => {
      const data: LiveConnectionsResponse = {
        ...emptyData,
        udpSessions: [udpSession, { ...udpSession, id: "udp-x" }],
        ruleSummaries: [tcpSummary],
      };
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(data);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());
      await userEvent.click(screen.getByRole("tab", { name: /UDP/ }));
      expect(screen.getByText("2 sessions")).toBeInTheDocument();
    });
  });

  // Cross-protocol filtering: selecting a protocol that does not match the
  // active tab hides every row in that tab. This exercises the protocol guards
  // in filterTcp / filterUdp that the existing same-protocol filter tests miss.
  describe("cross-protocol filtering", () => {
    it("protocol filter UDP hides all rows on the TCP tab", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await screen.findByRole("table", { name: "TCP connections" });

      await userEvent.selectOptions(
        screen.getByRole("combobox", { name: "Filter by protocol" }),
        "udp"
      );

      expect(screen.getByText("No active TCP connections.")).toBeInTheDocument();
      expect(screen.queryByRole("table", { name: "TCP connections" })).not.toBeInTheDocument();
    });

    it("protocol filter TCP hides all rows on the UDP tab", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());

      await userEvent.click(screen.getByRole("tab", { name: /UDP/ }));
      await userEvent.selectOptions(
        screen.getByRole("combobox", { name: "Filter by protocol" }),
        "tcp"
      );

      expect(screen.getByText("No active or recent UDP sessions.")).toBeInTheDocument();
      expect(screen.queryByRole("table", { name: "UDP sessions" })).not.toBeInTheDocument();
    });

    it("status filter Active hides idle rules from the Rule Summary tab", async () => {
      // idleSummary has zero active TCP/UDP, so the "active" status filter must
      // drop it, leaving the empty state.
      const data: LiveConnectionsResponse = {
        ...emptyData,
        ruleSummaries: [idleSummary],
      };
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(data);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());

      await userEvent.click(screen.getByRole("tab", { name: /Rule Summary/ }));
      await userEvent.selectOptions(
        screen.getByRole("combobox", { name: "Filter by status" }),
        "active"
      );

      expect(screen.getByText("No rule summaries available.")).toBeInTheDocument();
    });
  });

  // filterUdp ruleId guard (line 43): a rule filter applied while on the UDP tab
  // must drop sessions whose ruleId does not match. The existing rule-filter test
  // only exercises the TCP path, leaving this UDP branch uncovered.
  describe("UDP rule filter", () => {
    it("rule filter drops UDP sessions from other rules", async () => {
      const otherSession: UdpSessionInfo = {
        ...udpSession,
        id: "udp-9",
        ruleId: "rule-99",
        ruleName: "Other UDP Rule",
      };
      const data: LiveConnectionsResponse = {
        ...emptyData,
        udpSessions: [udpSession, otherSession],
        ruleSummaries: [
          { ...tcpSummary, ruleId: "rule-2", ruleName: "Game Server", protocol: "udp" },
          { ...tcpSummary, ruleId: "rule-99", ruleName: "Other UDP Rule", protocol: "udp" },
        ],
      };
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(data);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());

      await userEvent.click(screen.getByRole("tab", { name: /UDP/ }));
      // Filter to the first UDP rule; the other rule's session must disappear.
      await userEvent.selectOptions(
        screen.getByRole("combobox", { name: "Filter by rule" }),
        "rule-2"
      );

      const table = screen.getByRole("table", { name: "UDP sessions" });
      expect(within(table).getByText("Game Server")).toBeInTheDocument();
      expect(within(table).queryByText("Other UDP Rule")).not.toBeInTheDocument();
      expect(screen.getByText("1 session")).toBeInTheDocument();
    });
  });

  // Auto-refresh interval effect (lines 101-105): the setInterval callback at
  // line 103 re-invokes load on each tick. Fake timers advance past one interval
  // and observe the extra fetch, covering that callback function. fireEvent is
  // used (not userEvent) to avoid the userEvent<->fake-timer interplay.
  describe("auto-refresh interval", () => {
    afterEach(() => {
      vi.useRealTimers();
    });

    it("re-fetches connections on each interval tick while auto-refresh is on", async () => {
      vi.useFakeTimers();
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(emptyData);

      render(<LiveConnectionsView />);
      // Flush the mount-effect load (its promise resolves on a microtask).
      await vi.advanceTimersByTimeAsync(0);
      expect(portierApi.fetchLiveConnections).toHaveBeenCalledTimes(1);

      // Auto-refresh defaults on at 2s; advancing fires the interval callback.
      await vi.advanceTimersByTimeAsync(2000);
      expect(portierApi.fetchLiveConnections).toHaveBeenCalledTimes(2);

      await vi.advanceTimersByTimeAsync(2000);
      expect(portierApi.fetchLiveConnections).toHaveBeenCalledTimes(3);

      // Disabling auto-refresh clears the interval: no further ticks fetch.
      fireEvent.click(screen.getByRole("checkbox", { name: "Auto-refresh" }));
      await vi.advanceTimersByTimeAsync(4000);
      expect(portierApi.fetchLiveConnections).toHaveBeenCalledTimes(3);
    });
  });

  describe("load resilience", () => {
    it("non-Error rejection falls back to a generic error message", async () => {
      // fetchLiveConnections rejects with a non-Error value; the view must not
      // crash and should show the fallback copy rather than "[object …]".
      vi.mocked(portierApi.fetchLiveConnections).mockRejectedValue("boom");
      render(<LiveConnectionsView />);
      await screen.findByText("Failed to load connections.");
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });

    it("ignores a refresh while a fetch is already in flight", async () => {
      // First load never resolves until we release it; clicking Refresh while it
      // is pending must be a no-op (the in-flight guard), so no second call fires
      // until the first settles.
      let release!: (v: LiveConnectionsResponse) => void;
      vi.mocked(portierApi.fetchLiveConnections).mockReturnValueOnce(
        new Promise((r) => { release = r; })
      );
      render(<LiveConnectionsView />);
      // The initial load is in flight (loading copy visible).
      expect(screen.getByText("Loading connections…")).toBeInTheDocument();
      expect(portierApi.fetchLiveConnections).toHaveBeenCalledTimes(1);

      await userEvent.click(screen.getByRole("button", { name: "Refresh" }));
      // Guard: still only the one in-flight call.
      expect(portierApi.fetchLiveConnections).toHaveBeenCalledTimes(1);

      // Release the first load; subsequent refreshes are allowed again.
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(emptyData);
      release(emptyData);
      await waitFor(() =>
        expect(screen.queryByText("Loading connections…")).not.toBeInTheDocument()
      );

      await userEvent.click(screen.getByRole("button", { name: "Refresh" }));
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalledTimes(2));
    });
  });

  // Two inline JSX callbacks not exercised elsewhere: the TCP tab button's
  // onClick (the tab is already active on mount, so clicking another tab and
  // returning is the only way to invoke it) and the auto-refresh interval
  // <select> onChange.
  describe("tab and interval callbacks", () => {
    it("clicking the TCP tab after switching away re-activates the TCP view", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(fullData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());

      // Switch off TCP first, then click TCP to fire its onClick (line 206).
      await userEvent.click(screen.getByRole("tab", { name: /UDP/ }));
      expect(screen.getByRole("tab", { name: /UDP/ })).toHaveAttribute("aria-selected", "true");

      await userEvent.click(screen.getByRole("tab", { name: /TCP/ }));
      expect(screen.getByRole("tab", { name: /TCP/ })).toHaveAttribute("aria-selected", "true");
      expect(screen.getByRole("table", { name: "TCP connections" })).toBeInTheDocument();
    });

    it("selecting a new auto-refresh interval updates the control value", async () => {
      vi.mocked(portierApi.fetchLiveConnections).mockResolvedValue(emptyData);
      render(<LiveConnectionsView />);
      await waitFor(() => expect(portierApi.fetchLiveConnections).toHaveBeenCalled());

      const intervalSelect = screen.getByRole("combobox", { name: "Auto-refresh interval" });
      // Default interval is 2s; choosing 10s fires onChange (line 383).
      await userEvent.selectOptions(intervalSelect, "10");
      expect((intervalSelect as HTMLSelectElement).value).toBe("10");
    });
  });
});
