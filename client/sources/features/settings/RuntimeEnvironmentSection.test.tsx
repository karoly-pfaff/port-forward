import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RuntimeInfo } from "@portier/shared";
import { RuntimeEnvironmentSection } from "./RuntimeEnvironmentSection.js";
import * as portierApi from "../../api/portierApi.js";

vi.mock("../../api/portierApi.js", () => ({
  fetchRuntimeInfo: vi.fn()
}));

// Base runtime info; individual tests override only the field under test so the
// uptime-formatting and runtime-name branches can be exercised in isolation.
const baseRuntimeInfo: RuntimeInfo = {
  name: "Portier",
  version: "1.18.0",
  runtime: "go",
  platform: "linux",
  arch: "arm64",
  uptimeSeconds: 120,
  startedAt: "2026-01-01T00:00:00.000Z",
  managementHost: "127.0.0.1",
  managementPort: 47831,
  configPath: "/etc/portier/rules.json",
  staticDir: "/usr/share/portier/web",
  serviceMode: false,
  pid: 4321,
  recovery: { active: false }
};

describe("RuntimeEnvironmentSection formatUptime branches", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // seconds < 60 → the "Ns" branch (line 7). The other tests in the suite cover
  // the minutes and hours branches, so all three formatUptime arms are hit.
  it("formats sub-minute uptime as plain seconds", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue({
      ...baseRuntimeInfo,
      uptimeSeconds: 45
    });

    render(<RuntimeEnvironmentSection />);

    await waitFor(() => expect(screen.getByText("45s")).toBeInTheDocument());
  });

  // 60 <= seconds < 3600 → the "Nm Ns" branch (line 8).
  it("formats sub-hour uptime as minutes and seconds", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue({
      ...baseRuntimeInfo,
      uptimeSeconds: 125 // 2m 5s
    });

    render(<RuntimeEnvironmentSection />);

    await waitFor(() => expect(screen.getByText("2m 5s")).toBeInTheDocument());
  });

  // seconds >= 3600 → the hours/minutes branch (lines 9-12), previously
  // uncovered. 7384s = 2h 3m (remainder seconds dropped).
  it("formats multi-hour uptime as hours and minutes", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue({
      ...baseRuntimeInfo,
      uptimeSeconds: 7384
    });

    render(<RuntimeEnvironmentSection />);

    await waitFor(() => expect(screen.getByText("2h 3m")).toBeInTheDocument());
  });

  // The runtime-name ternary (line 31): a "node" runtime renders "Node server"
  // (the "go" side is covered by SettingsView tests).
  it("labels a node runtime as Node server", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue({
      ...baseRuntimeInfo,
      runtime: "node"
    });

    render(<RuntimeEnvironmentSection />);

    await waitFor(() => expect(screen.getByText("Node server")).toBeInTheDocument());
  });

  // serviceMode false renders "No" (the "Yes" side is covered by SettingsView).
  it("shows service mode No when not running as a service", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue({
      ...baseRuntimeInfo,
      serviceMode: false
    });

    render(<RuntimeEnvironmentSection />);

    await screen.findByText("Go service");
    expect(screen.getByText("No")).toBeInTheDocument();
  });
});
