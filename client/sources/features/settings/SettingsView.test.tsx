import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SettingsView } from "./SettingsView.js";
import * as portierApi from "../../api/portierApi.js";

vi.mock("../../api/portierApi.js", () => ({
  exportConfig: vi.fn(),
  importConfig: vi.fn()
}));

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
    expect(screen.getByRole("button", { name: /Download/ })).toBeInTheDocument();
  });

  it("renders the Import Config section with a file picker", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText("Import Config")).toBeInTheDocument();
    expect(screen.getByLabelText("Select config file")).toBeInTheDocument();
  });

  it("renders the About section with version number", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText("About Portier")).toBeInTheDocument();
    expect(screen.getByText("1.0.0")).toBeInTheDocument();
  });

  it("explains that management UI is localhost-only by default", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText(/only bind to/)).toBeInTheDocument();
  });

  it("explains that import is atomic (all-or-nothing)", () => {
    render(<SettingsView onRulesUpdated={vi.fn()} />);
    expect(screen.getByText(/atomic/)).toBeInTheDocument();
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
});
