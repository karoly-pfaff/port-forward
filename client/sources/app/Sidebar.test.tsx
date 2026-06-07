import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { PORTIER_APP_VERSION } from "@portier/shared";
import { Sidebar } from "./Sidebar.js";

const defaultProps = {
  open: false,
  currentView: "rules" as const,
  onNavClick: vi.fn()
};

describe("Sidebar", () => {
  it("renders without crashing", () => {
    render(<Sidebar {...defaultProps} />);
    expect(screen.getByRole("navigation")).toBeInTheDocument();
  });

  it("shows Portier in the footer", () => {
    render(<Sidebar {...defaultProps} />);
    expect(screen.getByText(/Portier/)).toBeInTheDocument();
  });

  it("shows the management host in the footer", () => {
    render(<Sidebar {...defaultProps} />);
    expect(screen.getByText(/127\.0\.0\.1:47831/)).toBeInTheDocument();
  });

  it("shows the app version in the footer", () => {
    render(<Sidebar {...defaultProps} />);
    expect(screen.getByText(`v${PORTIER_APP_VERSION}`)).toBeInTheDocument();
  });

  it("applies open class when open prop is true", () => {
    const { container } = render(<Sidebar {...defaultProps} open={true} />);
    expect(container.querySelector(".sidebar--mobile-open")).toBeInTheDocument();
  });

  it("calls onNavClick when a nav item is clicked", async () => {
    const onNavClick = vi.fn();
    render(<Sidebar {...defaultProps} onNavClick={onNavClick} />);
    const buttons = screen.getAllByRole("button");
    await userEvent.click(buttons[0]);
    expect(onNavClick).toHaveBeenCalled();
  });
});
