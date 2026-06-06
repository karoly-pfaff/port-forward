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

  it("shows LAN exposure advisory when listenHost is 0.0.0.0", async () => {
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
      screen.getByText(/exposes this forwarded port on the LAN/)
    ).toBeInTheDocument();
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

  it("shows save error when onSave rejects", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockRejectedValue(new Error("Conflict: duplicate binding"));
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

    expect(await screen.findByText(/Conflict: duplicate binding/)).toBeInTheDocument();
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
