import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ForwardRule } from "@portier/shared";
import { ForwardRuleForm } from "./ForwardRuleForm.js";

const existingRule: ForwardRule = {
  id: "r1",
  name: "Existing Rule",
  protocol: "tcp",
  listenHost: "127.0.0.1",
  listenPort: 48001,
  targetHost: "127.0.0.1",
  targetPort: 3000,
  enabled: false
};

describe("ForwardRuleForm", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders in add mode when no editingRule is provided", () => {
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    expect(screen.getByRole("heading", { name: "Add Rule" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Add Rule" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Cancel" })).not.toBeInTheDocument();
  });

  it("renders in edit mode when editingRule is provided", () => {
    render(
      <ForwardRuleForm
        editingRule={existingRule}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    expect(screen.getByRole("heading", { name: "Edit Rule" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save Changes" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
  });

  it("shows name field pre-filled when editing", () => {
    render(
      <ForwardRuleForm
        editingRule={existingRule}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("Existing Rule");
  });

  it("disables submit and shows field error for an invalid listen port", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    const portInput = screen.getByRole("spinbutton", { name: /Listen Port/ });
    await user.clear(portInput);
    await user.type(portInput, "99999");

    expect(screen.getByRole("button", { name: "Add Rule" })).toBeDisabled();
    expect(screen.getByText(/Must be 1/)).toBeInTheDocument();
  });

  it("disables submit and shows field error for an invalid target port", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    const portInput = screen.getByRole("spinbutton", { name: /Target Port/ });
    await user.clear(portInput);
    await user.type(portInput, "0");

    expect(screen.getByRole("button", { name: "Add Rule" })).toBeDisabled();
  });

  it("submits a valid TCP rule with expected payload", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={onSave}
        onCancel={vi.fn()}
        saving={false}
      />
    );

    await user.type(screen.getByRole("textbox", { name: "Name" }), "My App");
    await user.click(screen.getByRole("button", { name: "Add Rule" }));

    expect(onSave).toHaveBeenCalledWith(
      undefined,
      expect.objectContaining({
        name: "My App",
        protocol: "tcp",
        listenPort: 48000,
        targetPort: 80
      })
    );
  });

  it("renders an optional Group field", () => {
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    const group = screen.getByRole("textbox", { name: "Group" });
    expect(group).toBeInTheDocument();
    expect(group).toHaveValue("");
    expect(group).toHaveAttribute("maxLength", "64");
  });

  it("submits the group when set on create", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={onSave}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    await user.type(screen.getByRole("textbox", { name: "Name" }), "Grouped App");
    await user.type(screen.getByRole("textbox", { name: "Group" }), "web-team");
    await user.click(screen.getByRole("button", { name: "Add Rule" }));

    expect(onSave).toHaveBeenCalledWith(
      undefined,
      expect.objectContaining({ name: "Grouped App", group: "web-team" })
    );
  });

  it("pre-fills the group when editing a grouped rule", () => {
    render(
      <ForwardRuleForm
        editingRule={{ ...existingRule, group: "api-team" }}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    expect(screen.getByRole("textbox", { name: "Group" })).toHaveValue("api-team");
  });

  it("sends an empty group to clear it on edit", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <ForwardRuleForm
        editingRule={{ ...existingRule, group: "api-team" }}
        onSave={onSave}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    await user.clear(screen.getByRole("textbox", { name: "Group" }));
    await user.click(screen.getByRole("button", { name: "Save Changes" }));

    expect(onSave).toHaveBeenCalledWith(
      "r1",
      expect.objectContaining({ group: "" })
    );
  });

  it("rejects a group containing control characters on submit", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={onSave}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    await user.type(screen.getByRole("textbox", { name: "Name" }), "Bad Group");
    // Control characters can only arrive via paste (maxLength blocks typing length,
    // not control chars), so paste a tab into the group field.
    const group = screen.getByRole("textbox", { name: "Group" });
    group.focus();
    await user.paste("bad\tgroup");
    await user.click(screen.getByRole("button", { name: "Add Rule" }));

    expect(onSave).not.toHaveBeenCalled();
    expect(
      screen.getByText(/group must not contain control characters/i)
    ).toBeInTheDocument();
  });

  it("disables UDP Mode when protocol is TCP and enables it when switched to UDP", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    expect(screen.getByRole("combobox", { name: /UDP Mode/ })).toBeDisabled();

    await user.selectOptions(screen.getByRole("combobox", { name: "Protocol" }), "udp");

    expect(screen.getByRole("combobox", { name: /UDP Mode/ })).toBeEnabled();
  });

  it("shows LAN exposure warning when listenHost is 0.0.0.0", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    const hostInput = screen.getByRole("textbox", { name: "Listen Host" });
    await user.clear(hostInput);
    await user.type(hostInput, "0.0.0.0");

    expect(
      screen.getByText(/listens on all network interfaces/)
    ).toBeInTheDocument();
  });

  it("does not show LAN warning when listenHost is 127.0.0.1", () => {
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    expect(screen.queryByText(/listens on all network interfaces/)).not.toBeInTheDocument();
  });

  it("sets listenHost to 127.0.0.1 when Local only preset is clicked", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    const hostInput = screen.getByRole("textbox", { name: "Listen Host" });
    await user.clear(hostInput);
    await user.type(hostInput, "0.0.0.0");

    await user.click(screen.getByRole("button", { name: "Local only" }));
    expect(hostInput).toHaveValue("127.0.0.1");
  });

  it("sets listenHost to 0.0.0.0 when LAN exposed preset is clicked", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    await user.click(screen.getByRole("button", { name: "LAN exposed" }));
    expect(screen.getByRole("textbox", { name: "Listen Host" })).toHaveValue("0.0.0.0");
  });

  it("allows manual entry of a custom listen host", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    const hostInput = screen.getByRole("textbox", { name: "Listen Host" });
    await user.clear(hostInput);
    await user.type(hostInput, "192.168.1.10");
    expect(hostInput).toHaveValue("192.168.1.10");
    expect(screen.queryByText(/listens on all network interfaces/)).not.toBeInTheDocument();
  });

  it("shows LAN hint text when 0.0.0.0 is entered", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    await user.click(screen.getByRole("button", { name: "LAN exposed" }));
    expect(
      screen.getByText(/Other devices on your network may be able to connect/)
    ).toBeInTheDocument();
  });

  it("shows local only hint text when 127.0.0.1 is selected", () => {
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    expect(
      screen.getByText(/Only this computer can connect/)
    ).toBeInTheDocument();
  });

  it("shows generic firewall note when listenHost is 0.0.0.0 and no platform provided", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    await user.click(screen.getByRole("button", { name: "LAN exposed" }));
    expect(
      screen.getByText(/operating system firewall may still block LAN connections/)
    ).toBeInTheDocument();
  });

  it("shows Windows-specific firewall note when platform is windows", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
        runtimePlatform="windows"
      />
    );
    await user.click(screen.getByRole("button", { name: "LAN exposed" }));
    expect(
      screen.getByText(/Windows may ask for firewall permission/)
    ).toBeInTheDocument();
  });

  it("shows generic firewall note when platform is linux", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
        runtimePlatform="linux"
      />
    );
    await user.click(screen.getByRole("button", { name: "LAN exposed" }));
    expect(
      screen.getByText(/operating system firewall may still block LAN connections/)
    ).toBeInTheDocument();
  });

  it("does not block save when listenHost is 0.0.0.0", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={onSave}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    await user.click(screen.getByRole("button", { name: "LAN exposed" }));
    await user.type(screen.getByRole("textbox", { name: "Name" }), "LAN Rule");
    await user.click(screen.getByRole("button", { name: "Add Rule" }));
    expect(onSave).toHaveBeenCalled();
  });

  it("shows a privileged port advisory for ports below 1024", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    const portInput = screen.getByRole("spinbutton", { name: /Listen Port/ });
    await user.clear(portInput);
    await user.type(portInput, "80");

    expect(screen.getByText(/may require elevated permissions/i)).toBeInTheDocument();
  });

  it("shows a common port advisory for a well-known port", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    const portInput = screen.getByRole("spinbutton", { name: /Listen Port/ });
    await user.clear(portInput);
    await user.type(portInput, "5173");

    expect(screen.getByText(/Vite dev server/)).toBeInTheDocument();
  });

  it("shows form-level errors when name is empty on submit", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    await user.click(screen.getByRole("button", { name: "Add Rule" }));

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText(/name is required/i)).toBeInTheDocument();
  });

  it("preserves port input value while showing name validation error", async () => {
    const user = userEvent.setup();
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    await user.click(screen.getByRole("button", { name: "Add Rule" }));

    expect(screen.getByText(/name is required/i)).toBeInTheDocument();
    expect(
      screen.getByRole("spinbutton", { name: /Listen Port/ })
    ).toHaveValue(48000);
  });

  it("calls onCancel when Cancel is clicked in edit mode", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    render(
      <ForwardRuleForm
        editingRule={existingRule}
        onSave={vi.fn()}
        onCancel={onCancel}
        saving={false}
      />
    );
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onCancel).toHaveBeenCalled();
  });

  it("disables submit button while saving", () => {
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        saving={true}
      />
    );
    expect(screen.getByRole("button", { name: "Add Rule" })).toBeDisabled();
  });

  it("shows save error when onSave rejects with a non-conflict error", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockRejectedValue(new Error("Internal server error"));
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={onSave}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    await user.type(screen.getByRole("textbox", { name: "Name" }), "My App");
    await user.click(screen.getByRole("button", { name: "Add Rule" }));

    expect(await screen.findByText(/Internal server error/)).toBeInTheDocument();
  });

  it("shows friendly conflict message when save fails with a duplicate binding error", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockRejectedValue(new Error("A TCP rule is already listening on 127.0.0.1:48000."));
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={onSave}
        onCancel={vi.fn()}
        saving={false}
      />
    );
    await user.type(screen.getByRole("textbox", { name: "Name" }), "My App");
    await user.click(screen.getByRole("button", { name: "Add Rule" }));

    expect(
      await screen.findByText(/Another rule is already using this protocol/)
    ).toBeInTheDocument();
  });

  it("shows Delete Rule button in edit mode when onDelete is provided", () => {
    render(
      <ForwardRuleForm
        editingRule={existingRule}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        onDelete={vi.fn()}
        saving={false}
      />
    );
    expect(screen.getByRole("button", { name: "Delete Rule" })).toBeInTheDocument();
  });

  it("does not show Delete Rule button in add mode", () => {
    render(
      <ForwardRuleForm
        editingRule={undefined}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        onDelete={vi.fn()}
        saving={false}
      />
    );
    expect(screen.queryByRole("button", { name: "Delete Rule" })).not.toBeInTheDocument();
  });

  it("requires confirmation before deleting in edit mode", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(
      <ForwardRuleForm
        editingRule={existingRule}
        onSave={vi.fn()}
        onCancel={vi.fn()}
        onDelete={onDelete}
        saving={false}
      />
    );
    await user.click(screen.getByRole("button", { name: "Delete Rule" }));
    expect(onDelete).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Confirm Delete" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Confirm Delete" }));
    expect(onDelete).toHaveBeenCalledWith(existingRule);
  });
});
