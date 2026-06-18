import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ConfigPlanResponse, ForwardRuleResponse, ForwardStatus, RuntimeInfo } from "@portier/shared";
import { SettingsView } from "./SettingsView.js";
import * as portierApi from "../../api/portierApi.js";
import type { DiagnosisEntry } from "../forwards/ForwardRuleList.js";

vi.mock("../../api/portierApi.js", () => ({
  exportConfig: vi.fn(),
  importConfig: vi.fn(),
  fetchRuntimeInfo: vi.fn().mockRejectedValue(new Error("unavailable")),
  fetchForwardRules: vi.fn().mockResolvedValue([]),
  fetchForwardStatus: vi.fn().mockResolvedValue([]),
  fetchActivity: vi.fn().mockResolvedValue([]),
  planConfig: vi.fn(),
  applyConfig: vi.fn(),
}));

const testRuntimeInfo: RuntimeInfo = {
  name: "Portier",
  version: "1.1.0",
  runtime: "go",
  platform: "windows",
  arch: "x64",
  uptimeSeconds: 120,
  startedAt: "2026-01-01T00:00:00.000Z",
  managementHost: "127.0.0.1",
  managementPort: 47831,
  configPath: "C:\\ProgramData\\Portier\\rules.json",
  staticDir: "C:\\Program Files\\Portier\\web",
  serviceMode: true,
  pid: 1234,
  recovery: { active: false }
};

describe("SettingsView", () => {
  it("renders the Management Endpoint section with default address", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText("Management Endpoint")).toBeInTheDocument();
    // Address appears in multiple sections; check at least one is present
    expect(screen.getAllByText("127.0.0.1:47831").length).toBeGreaterThan(0);
  });

  it("renders the Recommended Forward Port Range section", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText("Recommended Forward Port Range")).toBeInTheDocument();
    expect(screen.getByText(/48000/)).toBeInTheDocument();
    expect(screen.getByText(/48999/)).toBeInTheDocument();
  });

  it("renders the Export Config section with a download button", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText("Export Config")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Download Config/ })).toBeInTheDocument();
  });

  it("export button mentions Activity Log is not included", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText(/Activity Log is not included/)).toBeInTheDocument();
  });

  it("renders the Import Config section with a file picker", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText("Import Config")).toBeInTheDocument();
    expect(screen.getByLabelText("Select config file")).toBeInTheDocument();
  });

  it("renders the About section", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText("About Portier")).toBeInTheDocument();
  });

  it("renders the Runtime / Environment section", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText("Runtime / Environment")).toBeInTheDocument();
  });

  it("explains that management UI is localhost-only by default", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText(/only bind to/)).toBeInTheDocument();
  });

  it("explains that import is atomic (all-or-nothing)", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText(/atomic/)).toBeInTheDocument();
  });

  it("import mode radios are visible without selecting a file", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByRole("radio", { name: /Merge/ })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Replace/ })).toBeInTheDocument();
  });

  it("merge mode explanation is visible", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText(/skips rules with conflicting listen port/)).toBeInTheDocument();
  });

  it("replace mode explanation mentions deleting current rules", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText(/stops and deletes all current rules/)).toBeInTheDocument();
  });
});

describe("SettingsView runtime info", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.runAllTimers();
    vi.useRealTimers();
  });

  it("shows loading state initially", () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockReturnValue(new Promise(() => {}));
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText(/Loading runtime info/)).toBeInTheDocument();
  });

  it("shows unavailable message when fetch fails", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockRejectedValue(new Error("unavailable"));
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await waitFor(() => {
      expect(screen.getByText(/Runtime information is unavailable from this backend/)).toBeInTheDocument();
    });
  });

  it("shows runtime info when fetch succeeds", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue(testRuntimeInfo);
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await waitFor(() => {
      expect(screen.getByText("Go service")).toBeInTheDocument();
    });
    expect(screen.getByText("1.1.0")).toBeInTheDocument();
    expect(screen.getByText(/windows \/ x64/)).toBeInTheDocument();
    expect(screen.getByText("2m 0s")).toBeInTheDocument();
  });

  it("shows config path and static dir when fetch succeeds", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue(testRuntimeInfo);
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await waitFor(() => {
      expect(screen.getByText(testRuntimeInfo.configPath)).toBeInTheDocument();
    });
    expect(screen.getByText(testRuntimeInfo.staticDir)).toBeInTheDocument();
  });

  it("shows copy buttons for config path, static dir, and management URL", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue(testRuntimeInfo);
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await waitFor(() => {
      expect(screen.getByText("Go service")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "Copy config path" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy static dir" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy management URL" })).toBeInTheDocument();
  });

  it("copy config path calls clipboard with correct value", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue(testRuntimeInfo);
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Go service")).toBeInTheDocument());

    await userEvent.click(screen.getByRole("button", { name: "Copy config path" }));
    expect(writeText).toHaveBeenCalledWith(testRuntimeInfo.configPath);
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Copy config path" })).toHaveTextContent("Copied!");
    });
  });

  it("copy static dir calls clipboard with correct value", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue(testRuntimeInfo);
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Go service")).toBeInTheDocument());

    await userEvent.click(screen.getByRole("button", { name: "Copy static dir" }));
    expect(writeText).toHaveBeenCalledWith(testRuntimeInfo.staticDir);
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Copy static dir" })).toHaveTextContent("Copied!");
    });
  });

  it("copy management URL calls clipboard with host:port", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue(testRuntimeInfo);
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Go service")).toBeInTheDocument());

    await userEvent.click(screen.getByRole("button", { name: "Copy management URL" }));
    expect(writeText).toHaveBeenCalledWith("127.0.0.1:47831");
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Copy management URL" })).toHaveTextContent("Copied!");
    });
  });

  it("clipboard failure shows Failed feedback without crashing", async () => {
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue(testRuntimeInfo);
    const writeText = vi.fn().mockRejectedValue(new Error("denied"));
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Go service")).toBeInTheDocument());

    await userEvent.click(screen.getByRole("button", { name: "Copy config path" }));
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Copy config path" })).toHaveTextContent("Failed");
    });
  });
});

describe("SettingsView export", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("export success shows confirmation message", async () => {
    vi.mocked(portierApi.exportConfig).mockResolvedValue({
      version: "1",
      exportedAt: new Date().toISOString(),
      rules: []
    });
    Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn(() => "blob:test") });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
    // Prevent jsdom navigation warning from anchor.click()
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /Download Config/ }));

    await waitFor(() => {
      expect(screen.getByText("Config exported successfully.")).toBeInTheDocument();
    });
  });

  it("export failure shows error message", async () => {
    vi.mocked(portierApi.exportConfig).mockRejectedValue(new Error("network error"));

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /Download Config/ }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("network error");
    });
  });
});

// FileReader stub that synchronously delivers content via queueMicrotask.
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

const validConfig = {
  version: "1",
  exportedAt: new Date().toISOString(),
  rules: [
    {
      id: "r1",
      name: "Imported Rule",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort: 48001,
      targetHost: "127.0.0.1",
      targetPort: 3000,
      enabled: false
    }
  ]
};

describe("SettingsView import flow", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("valid file shows preview and merge import calls API with correct args", async () => {
    const configJson = JSON.stringify(validConfig);
    stubFileReader(configJson);

    vi.mocked(portierApi.importConfig).mockResolvedValue({
      result: { imported: 1, skipped: 0, errors: [] },
      rules: []
    });

    const onRulesUpdated = vi.fn();
    render(<SettingsView onRulesUpdated={onRulesUpdated} />);

    const file = new File([configJson], "portier-rules.json", { type: "application/json" });
    await userEvent.upload(screen.getByLabelText("Select config file"), file);

    await screen.findByText("File preview");
    expect(screen.getByText(/1 TCP/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Import Rules/ }));

    expect(portierApi.importConfig).toHaveBeenCalledWith(
      expect.objectContaining({ version: "1" }),
      "merge"
    );
    await screen.findByText(/Import complete/);
    expect(onRulesUpdated).toHaveBeenCalled();
  });

  it("replace mode shows confirm dialog before calling API", async () => {
    const configJson = JSON.stringify(validConfig);
    stubFileReader(configJson);

    vi.mocked(portierApi.importConfig).mockResolvedValue({
      result: { imported: 1, skipped: 0, errors: [] },
      rules: []
    });

    render(<SettingsView onRulesUpdated={vi.fn()} />);

    const file = new File([configJson], "portier-rules.json", { type: "application/json" });
    await userEvent.upload(screen.getByLabelText("Select config file"), file);

    await screen.findByText("File preview");

    await userEvent.click(screen.getByRole("radio", { name: /Replace/ }));
    await userEvent.click(screen.getByRole("button", { name: /Replace All Rules/ }));

    expect(screen.getByText(/This will delete all existing rules/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Confirm Replace/ }));

    expect(portierApi.importConfig).toHaveBeenCalledWith(
      expect.objectContaining({ version: "1" }),
      "replace"
    );
  });

  it("replace confirm dialog shows export backup button", async () => {
    const configJson = JSON.stringify(validConfig);
    stubFileReader(configJson);

    render(<SettingsView onRulesUpdated={vi.fn()} />);

    const file = new File([configJson], "portier-rules.json", { type: "application/json" });
    await userEvent.upload(screen.getByLabelText("Select config file"), file);

    await screen.findByText("File preview");
    await userEvent.click(screen.getByRole("radio", { name: /Replace/ }));
    await userEvent.click(screen.getByRole("button", { name: /Replace All Rules/ }));

    expect(screen.getByRole("button", { name: /Export current config as backup/ })).toBeInTheDocument();
  });

  it("invalid JSON shows parse error and does not call import API", async () => {
    stubFileReader("{ not valid json {{{{");

    render(<SettingsView onRulesUpdated={vi.fn()} />);

    const file = new File(["{ not valid json {{{{"], "bad.json", { type: "application/json" });
    await userEvent.upload(screen.getByLabelText("Select config file"), file);

    await screen.findByRole("alert");
    expect(portierApi.importConfig).not.toHaveBeenCalled();
  });

  it("wrong config structure shows parse error", async () => {
    stubFileReader(JSON.stringify({ version: "2", rules: [] }));

    render(<SettingsView onRulesUpdated={vi.fn()} />);

    const file = new File(
      [JSON.stringify({ version: "2", rules: [] })],
      "portier-rules.json",
      { type: "application/json" }
    );
    await userEvent.upload(screen.getByLabelText("Select config file"), file);

    await screen.findByRole("alert");
    expect(portierApi.importConfig).not.toHaveBeenCalled();
  });

  it("import button not present until a valid file is selected", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /Import Rules/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Replace All Rules/ })).not.toBeInTheDocument();
  });
});

const testRuleForDiag: ForwardRuleResponse = {
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

const testStatusForDiag: ForwardStatus = {
  ruleId: "r1",
  running: true, health: "healthy",
  bytesIn: 0,
  bytesOut: 0
};

describe("SettingsView diagnostics export", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(portierApi.fetchRuntimeInfo).mockResolvedValue(testRuntimeInfo);
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([testRuleForDiag]);
    vi.mocked(portierApi.fetchForwardStatus).mockResolvedValue([testStatusForDiag]);
    vi.mocked(portierApi.fetchActivity).mockResolvedValue([]);
    Object.defineProperty(URL, "createObjectURL", { configurable: true, value: vi.fn(() => "blob:test") });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: vi.fn() });
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
  });

  it("renders Diagnostics Export section with button", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText("Diagnostics Export")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Download Diagnostics/ })).toBeInTheDocument();
  });

  it("helper text mentions what is included and excluded", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText(/runtime info, rules, statuses, recent activity/)).toBeInTheDocument();
    expect(screen.getByText(/Does not include logs, environment variables/)).toBeInTheDocument();
  });

  it("button is disabled while export is in progress", async () => {
    let resolveBundle!: () => void;
    vi.mocked(portierApi.fetchForwardRules).mockReturnValue(
      new Promise<ForwardRuleResponse[]>((resolve) => {
        resolveBundle = () => resolve([]);
      })
    );

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    const btn = screen.getByRole("button", { name: /Download Diagnostics/ });
    await userEvent.click(btn);

    expect(screen.getByRole("button", { name: /Generating/ })).toBeDisabled();
    resolveBundle();
  });

  it("clicking export fetches rules, statuses, and activity", async () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /Download Diagnostics/ }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Download Diagnostics/ })).not.toBeDisabled();
    });

    expect(portierApi.fetchForwardRules).toHaveBeenCalled();
    expect(portierApi.fetchForwardStatus).toHaveBeenCalled();
    expect(portierApi.fetchActivity).toHaveBeenCalledWith({ limit: 100 });
  });

  it("shows success message after successful export", async () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /Download Diagnostics/ }));

    await waitFor(() => {
      expect(screen.getByRole("status")).toHaveTextContent("Diagnostics exported successfully.");
    });
  });

  it("shows partial warning when some sources fail", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockRejectedValue(new Error("network error"));

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /Download Diagnostics/ }));

    await waitFor(() => {
      expect(screen.getByRole("status")).toHaveTextContent(/partial data/);
    });
  });

  it("triggers download even with partial data", async () => {
    vi.mocked(portierApi.fetchForwardRules).mockRejectedValue(new Error("network error"));

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /Download Diagnostics/ }));

    await waitFor(() => {
      expect(HTMLAnchorElement.prototype.click).toHaveBeenCalled();
    });
  });

  it("includes diagnostics from diagnosisMap prop when available", async () => {
    const diagResult = {
      ruleId: "r1",
      ruleName: "My Rule",
      protocol: "tcp" as const,
      summary: { status: "pass" as const, message: "OK" },
      checks: [],
      diagnosedAt: "2026-01-01T00:00:00.000Z"
    };
    const diagnosisMap = new Map<string, DiagnosisEntry>([
      ["r1", { state: "done", result: diagResult }]
    ]);

    let capturedData: unknown;
    vi.mocked(URL.createObjectURL).mockImplementation((blob) => {
      const reader = new FileReader();
      reader.readAsText(blob as Blob);
      reader.onload = () => { capturedData = JSON.parse(reader.result as string); };
      return "blob:test";
    });

    render(<SettingsView onRulesUpdated={vi.fn()} diagnosisMap={diagnosisMap} />);
    await userEvent.click(screen.getByRole("button", { name: /Download Diagnostics/ }));

    await waitFor(() => {
      expect(capturedData).toBeDefined();
    });

    const bundle = capturedData as Record<string, unknown>;
    const diagnostics = bundle["diagnostics"] as Record<string, unknown>;
    expect(diagnostics["r1"]).toBeDefined();
  });

  it("exports with empty diagnostics and includes a note in the bundle", async () => {
    let capturedJson = "";
    vi.mocked(URL.createObjectURL).mockImplementation((blob) => {
      const reader = new FileReader();
      reader.readAsText(blob as Blob);
      reader.onload = () => { capturedJson = reader.result as string; };
      return "blob:test";
    });

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /Download Diagnostics/ }));

    await waitFor(() => {
      expect(capturedJson).not.toBe("");
    });

    const bundle = JSON.parse(capturedJson) as Record<string, unknown>;
    expect(bundle["schemaVersion"]).toBe("1");
    expect(bundle["diagnostics"]).toEqual({});
    expect(bundle["diagnosticsNote"]).toContain("No rule diagnostics");
  });

  it("exported bundle contains required top-level keys", async () => {
    let capturedJson = "";
    vi.mocked(URL.createObjectURL).mockImplementation((blob) => {
      const reader = new FileReader();
      reader.readAsText(blob as Blob);
      reader.onload = () => { capturedJson = reader.result as string; };
      return "blob:test";
    });

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /Download Diagnostics/ }));

    await waitFor(() => {
      expect(capturedJson).not.toBe("");
    });

    const bundle = JSON.parse(capturedJson) as Record<string, unknown>;
    expect(bundle).toHaveProperty("schemaVersion");
    expect(bundle).toHaveProperty("exportedAt");
    expect(bundle).toHaveProperty("app");
    expect(bundle).toHaveProperty("rules");
    expect(bundle).toHaveProperty("statuses");
    expect(bundle).toHaveProperty("activity");
    expect(bundle).toHaveProperty("metadata");
  });
});

// ── Plan & Apply fixtures ──────────────────────────────────────────────────────

const noDriftPlan: ConfigPlanResponse = {
  generatedAt: "2026-01-01T00:00:00.000Z",
  mode: "plan",
  summary: { add: 0, update: 0, remove: 0, unchanged: 1, destructive: 0, hasDrift: false, hasErrors: false },
  operations: [
    { type: "unchanged", ruleId: "r1", ruleName: "My Rule", protocol: "tcp", destructive: false }
  ],
  errors: [],
  warnings: []
};

const addOnlyPlan: ConfigPlanResponse = {
  generatedAt: "2026-01-01T00:00:00.000Z",
  mode: "plan",
  summary: { add: 1, update: 0, remove: 0, unchanged: 0, destructive: 0, hasDrift: true, hasErrors: false },
  operations: [
    { type: "add", ruleName: "New Rule", protocol: "tcp", destructive: false,
      desired: { name: "New Rule", protocol: "tcp", listenHost: "127.0.0.1", listenPort: 48100,
                 targetHost: "127.0.0.1", targetPort: 9000, enabled: false } }
  ],
  errors: [],
  warnings: []
};

const destructivePlan: ConfigPlanResponse = {
  generatedAt: "2026-01-01T00:00:00.000Z",
  mode: "plan",
  summary: { add: 0, update: 0, remove: 1, unchanged: 0, destructive: 1, hasDrift: true, hasErrors: false },
  operations: [
    { type: "remove", ruleId: "r1", ruleName: "Old Rule", protocol: "tcp", destructive: true }
  ],
  errors: [],
  warnings: []
};

const errorPlan: ConfigPlanResponse = {
  generatedAt: "2026-01-01T00:00:00.000Z",
  mode: "plan",
  summary: { add: 0, update: 0, remove: 0, unchanged: 0, destructive: 0, hasDrift: false, hasErrors: true },
  operations: [],
  errors: [{ code: "INVALID_DESIRED_RULE", message: "rule 1: name is required" }],
  warnings: []
};

const warningPlan: ConfigPlanResponse = {
  generatedAt: "2026-01-01T00:00:00.000Z",
  mode: "plan",
  summary: { add: 1, update: 0, remove: 0, unchanged: 0, destructive: 0, hasDrift: true, hasErrors: false },
  operations: [
    { type: "add", ruleName: "LAN Rule", protocol: "tcp", destructive: false,
      desired: { name: "LAN Rule", protocol: "tcp", listenHost: "0.0.0.0", listenPort: 48100,
                 targetHost: "127.0.0.1", targetPort: 9000, enabled: false } }
  ],
  errors: [],
  warnings: [{ code: "LAN_EXPOSURE", message: "Rule listens on 0.0.0.0 — exposed on LAN" }]
};

const updatePlan: ConfigPlanResponse = {
  generatedAt: "2026-01-01T00:00:00.000Z",
  mode: "plan",
  summary: { add: 0, update: 1, remove: 0, unchanged: 0, destructive: 0, hasDrift: true, hasErrors: false },
  operations: [
    { type: "update", ruleId: "r1", ruleName: "My Rule", protocol: "tcp", destructive: false,
      changes: [{ field: "targetPort", before: 3000, after: 4000 }] }
  ],
  errors: [],
  warnings: []
};

// A group-only update: material (drift) but non-destructive (no forwarding
// field changed). The preview should mark it "Metadata only" and not imply a
// restart.
const groupOnlyUpdatePlan: ConfigPlanResponse = {
  generatedAt: "2026-01-01T00:00:00.000Z",
  mode: "plan",
  summary: { add: 0, update: 1, remove: 0, unchanged: 0, destructive: 0, hasDrift: true, hasErrors: false },
  operations: [
    { type: "update", ruleId: "r1", ruleName: "My Rule", protocol: "tcp", destructive: false,
      changes: [{ field: "group", before: undefined, after: "backend" }] }
  ],
  errors: [],
  warnings: []
};

const validPlanConfig = {
  version: "1" as const,
  exportedAt: new Date().toISOString(),
  rules: [
    { id: "r1", name: "My Rule", protocol: "tcp" as const, listenHost: "127.0.0.1",
      listenPort: 48001, targetHost: "127.0.0.1", targetPort: 3000, enabled: false }
  ]
};

describe("SettingsView Plan & Apply section", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders Plan & Apply Config section heading", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText("Plan & Apply Config")).toBeInTheDocument();
  });

  it("renders file picker for plan section with distinct label", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByLabelText("Select config file for plan")).toBeInTheDocument();
  });

  it("preview button is not shown before a file is selected", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /Preview changes/ })).not.toBeInTheDocument();
  });

  it("preview button appears after a valid file is selected", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    const file = new File([configJson], "desired.json", { type: "application/json" });
    await userEvent.upload(screen.getByLabelText("Select config file for plan"), file);

    await screen.findByRole("button", { name: /Preview changes/ });
  });

  it("parse error shown for invalid JSON in plan file picker", async () => {
    stubFileReader("{ not valid {{");
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    const file = new File(["{ not valid {{"], "bad.json", { type: "application/json" });
    await userEvent.upload(screen.getByLabelText("Select config file for plan"), file);
    await screen.findByRole("alert");
    expect(screen.queryByRole("button", { name: /Preview changes/ })).not.toBeInTheDocument();
  });

  it("plan file with wrong version shows parse error", async () => {
    const wrongVersion = JSON.stringify({ version: "99", rules: [] });
    stubFileReader(wrongVersion);
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    const file = new File([wrongVersion], "wrong.json", { type: "application/json" });
    await userEvent.upload(screen.getByLabelText("Select config file for plan"), file);
    await screen.findByRole("alert");
    expect(screen.queryByRole("button", { name: /Preview changes/ })).not.toBeInTheDocument();
  });

  it("clicking preview calls planConfig with desired rules from the file", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(noDriftPlan);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "desired.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await waitFor(() => {
      expect(portierApi.planConfig).toHaveBeenCalledWith(
        expect.objectContaining({ rules: expect.any(Array) })
      );
    });
  });

  it("preview shows plan summary counts", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(addOnlyPlan);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await screen.findByText("Plan preview");
    expect(screen.getByText(/Add: 1/)).toBeInTheDocument();
    expect(screen.getByText(/Update: 0/)).toBeInTheDocument();
    expect(screen.getByText(/Remove: 0/)).toBeInTheDocument();
  });

  it("no-drift state shows in-sync message", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(noDriftPlan);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await screen.findByText(/No drift detected/);
    expect(screen.getByText(/already in sync/)).toBeInTheDocument();
  });

  it("no-drift apply button is disabled (no changes to apply)", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(noDriftPlan);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await screen.findByText("Plan preview");
    const applyBtn = screen.getByRole("button", { name: /No changes to apply/ });
    expect(applyBtn).toBeDisabled();
  });

  it("preview shows operation list with rule names", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(addOnlyPlan);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await screen.findByText("Plan preview");
    expect(screen.getByText(/New Rule/)).toBeInTheDocument();
    expect(screen.getAllByText(/TCP/).length).toBeGreaterThan(0);
  });

  it("update operation shows field changes", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(updatePlan);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await screen.findByText("Plan preview");
    expect(screen.getByText("Target port")).toBeInTheDocument();
    expect(screen.getByText(/3000 → 4000/)).toBeInTheDocument();
    // A forwarding-field update is marked as restarting the forwarder.
    expect(screen.getByText(/the forwarder will restart/)).toBeInTheDocument();
  });

  it("group-only update is shown as a metadata-only change with no restart", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(groupOnlyUpdatePlan);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await screen.findByText("Plan preview");
    // The changed field is the Group, shown with its before → after values.
    expect(screen.getByText("Group")).toBeInTheDocument();
    expect(screen.getByText(/\(none\) → backend/)).toBeInTheDocument();
    // It is labelled metadata-only and explicitly does not restart the forwarder.
    expect(screen.getByText("Metadata only")).toBeInTheDocument();
    expect(screen.getByText(/the forwarder is not restarted/)).toBeInTheDocument();
    // Non-destructive: no confirmation checkbox, apply enabled.
    expect(screen.queryByLabelText("Confirm destructive changes")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Apply changes/ })).not.toBeDisabled();
  });

  it("destructive remove is clearly labelled (not by colour alone)", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(destructivePlan);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await screen.findByText("Plan preview");
    // The remove operation carries a text "Destructive" tag and an impact note,
    // and the summary surfaces the destructive count — none of which is colour-only.
    expect(screen.getByText("Destructive")).toBeInTheDocument();
    expect(screen.getByText(/Removes this existing rule/)).toBeInTheDocument();
    expect(screen.getByText(/Destructive: 1/)).toBeInTheDocument();
  });

  it("plan errors disable apply and show error message", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(errorPlan);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await screen.findByText(/Apply is disabled until plan errors are resolved/);
    expect(screen.getByText(/INVALID_DESIRED_RULE/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Apply changes/ })).not.toBeInTheDocument();
  });

  it("warnings are shown in plan preview", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(warningPlan);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await screen.findByText("Plan preview");
    expect(screen.getByText(/LAN_EXPOSURE/)).toBeInTheDocument();
    expect(screen.getAllByText(/0\.0\.0\.0/).length).toBeGreaterThan(0);
  });

  it("destructive plan shows confirmation checkbox", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(destructivePlan);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await screen.findByText("Plan preview");
    expect(screen.getByLabelText("Confirm destructive changes")).toBeInTheDocument();
  });

  it("apply button is disabled for destructive plan until confirmation checked", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(destructivePlan);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await screen.findByText("Plan preview");
    expect(screen.getByRole("button", { name: /Apply changes/ })).toBeDisabled();
  });

  it("apply button enabled after destructive confirmation checked", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(destructivePlan);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await screen.findByText("Plan preview");
    await userEvent.click(screen.getByLabelText("Confirm destructive changes"));
    expect(screen.getByRole("button", { name: /Apply changes/ })).not.toBeDisabled();
  });

  it("non-destructive apply button is enabled without confirmation", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(addOnlyPlan);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await screen.findByText("Plan preview");
    expect(screen.getByRole("button", { name: /Apply changes/ })).not.toBeDisabled();
  });

  it("apply sends yes:true and calls onRulesUpdated on success", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(addOnlyPlan);
    vi.mocked(portierApi.applyConfig).mockResolvedValue({
      ok: true, dryRun: false, appliedAt: "2026-01-01T00:00:00.000Z",
      plan: addOnlyPlan,
      applied: { add: 1, update: 0, remove: 0, unchanged: 0 }
    });
    const updatedRule: ForwardRuleResponse = {
      id: "r2", name: "New Rule", protocol: "tcp", listenHost: "127.0.0.1",
      listenPort: 48100, targetHost: "127.0.0.1", targetPort: 9000, enabled: false, advisories: []
    };
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([updatedRule]);
    const onRulesUpdated = vi.fn();

    render(<SettingsView onRulesUpdated={onRulesUpdated} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));
    await screen.findByText("Plan preview");
    await userEvent.click(screen.getByRole("button", { name: /Apply changes/ }));

    await waitFor(() => {
      expect(portierApi.applyConfig).toHaveBeenCalledWith(
        expect.objectContaining({ yes: true })
      );
    });
    await screen.findByText(/Config applied/);
    expect(onRulesUpdated).toHaveBeenCalledWith([updatedRule]);
  });

  it("apply API error shows error message", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(addOnlyPlan);
    vi.mocked(portierApi.applyConfig).mockRejectedValue(new Error("network error"));

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));
    await screen.findByText("Plan preview");
    await userEvent.click(screen.getByRole("button", { name: /Apply changes/ }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("network error");
    });
  });

  it("apply with ok:false shows error message", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(addOnlyPlan);
    vi.mocked(portierApi.applyConfig).mockResolvedValue({
      ok: false, dryRun: false, appliedAt: "2026-01-01T00:00:00.000Z",
      plan: addOnlyPlan,
      applied: { add: 0, update: 0, remove: 0, unchanged: 0 }
    });

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));
    await screen.findByText("Plan preview");
    await userEvent.click(screen.getByRole("button", { name: /Apply changes/ }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/Apply failed/);
    });
  });

  it("preview API error shows error message without plan result", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockRejectedValue(new Error("service unavailable"));

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("service unavailable");
    });
    expect(screen.queryByText("Plan preview")).not.toBeInTheDocument();
  });

  it("successful apply clears the form and shows success", async () => {
    const configJson = JSON.stringify(validPlanConfig);
    stubFileReader(configJson);
    vi.mocked(portierApi.planConfig).mockResolvedValue(addOnlyPlan);
    vi.mocked(portierApi.applyConfig).mockResolvedValue({
      ok: true, dryRun: false, appliedAt: "2026-01-01T00:00:00.000Z",
      plan: addOnlyPlan,
      applied: { add: 1, update: 0, remove: 0, unchanged: 0 }
    });
    vi.mocked(portierApi.fetchForwardRules).mockResolvedValue([]);

    render(<SettingsView onRulesUpdated={vi.fn()} />);
    await userEvent.upload(screen.getByLabelText("Select config file for plan"),
      new File([configJson], "d.json", { type: "application/json" }));
    await userEvent.click(await screen.findByRole("button", { name: /Preview changes/ }));
    await screen.findByText("Plan preview");
    await userEvent.click(screen.getByRole("button", { name: /Apply changes/ }));

    await screen.findByText(/Config applied/);
    // Plan preview cleared after success
    expect(screen.queryByText("Plan preview")).not.toBeInTheDocument();
  });

  it("existing import section still works after Plan & Apply section added", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText("Import Config")).toBeInTheDocument();
    expect(screen.getByLabelText("Select config file")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Merge/ })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Replace/ })).toBeInTheDocument();
  });
});
