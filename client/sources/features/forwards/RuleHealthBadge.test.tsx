import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { RuleHealth } from "@portier/shared";
import { RuleHealthBadge } from "./RuleHealthBadge.js";

describe("RuleHealthBadge", () => {
  it.each<[RuleHealth, string]>([
    ["healthy", "Health: Healthy"],
    ["warning", "Health: Warning — enabled but not running"],
    ["error", "Health: Error — see last error"]
  ])("renders an accessible %s badge", (health, label) => {
    const { container } = render(<RuleHealthBadge health={health} />);
    expect(screen.getByRole("img", { name: label })).toBeInTheDocument();
    expect(container.querySelector(`.health-badge--${health}`)).not.toBeNull();
  });
});
