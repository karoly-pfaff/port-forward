import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ForwardRuleResponse, ForwardStatus, RuntimeInfo } from "@portier/shared";
import { SettingsView } from "./SettingsView.js";
import * as portierApi from "../../api/portierApi.js";
import type { DiagnosisEntry } from "../forwards/ForwardRuleList.js";

vi.mock("../../api/portierApi.js", () => ({
  exportConfig: vi.fn(),
  importConfig: vi.fn(),
  fetchRuntimeInfo: vi.fn().mockRejectedValue(new Error("unavailable")),
  fetchForwardRules: vi.fn().mockResolvedValue([]),
  fetchForwardStatus: vi.fn().mockResolvedValue([]),
  fetchActivity: vi.fn().mockResolvedValue([])
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
  pid: 1234
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
  running: true,
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
