import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { RuleHealth } from "@portier/shared";
import { RuleHealthBadge, healthShortLabel } from "./RuleHealthBadge.js";

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

describe("healthShortLabel", () => {
  it.each<[RuleHealth, string]>([
    ["healthy", "Healthy"],
    ["warning", "Warning"],
    ["error", "Error"]
  ])("maps %s to its one-word column label", (health, label) => {
    expect(healthShortLabel(health)).toBe(label);
  });
});
