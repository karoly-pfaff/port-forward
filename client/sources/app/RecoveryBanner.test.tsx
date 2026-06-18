import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { RuntimeRecovery } from "@portier/shared";
import { RecoveryBanner } from "./RecoveryBanner.js";

describe("RecoveryBanner", () => {
  it("renders nothing when recovery is undefined", () => {
    const { container } = render(<RecoveryBanner recovery={undefined} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when recovery is inactive", () => {
    const { container } = render(<RecoveryBanner recovery={{ active: false }} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows an alert with the message, write-block note, and quarantine path when active", () => {
    const recovery: RuntimeRecovery = {
      active: true,
      reason: "malformed",
      message: "Configuration file could not be parsed; started with no active rules. The original file was quarantined.",
      configPath: "/data/rules.json",
      quarantinePath: "/data/rules.json.corrupt-2026-06-18T142530Z",
      writesBlocked: true,
      detectedAt: "2026-06-18T14:25:30.000Z",
    };

    render(<RecoveryBanner recovery={recovery} />);

    const banner = screen.getByRole("alert");
    expect(banner).toHaveTextContent("Configuration recovery mode.");
    expect(banner).toHaveTextContent("could not be parsed");
    expect(banner).toHaveTextContent("Rule changes are blocked");
    expect(banner).toHaveTextContent("/data/rules.json.corrupt-2026-06-18T142530Z");
  });

  it("omits the write-block note and quarantine path when not present (e.g. unreadable)", () => {
    const recovery: RuntimeRecovery = {
      active: true,
      reason: "unreadable",
      message: "Configuration file could not be read; started with no active rules. The original file was left untouched.",
      configPath: "/data/rules.json",
      writesBlocked: false,
    };

    render(<RecoveryBanner recovery={recovery} />);

    const banner = screen.getByRole("alert");
    expect(banner).toHaveTextContent("could not be read");
    expect(banner).not.toHaveTextContent("Rule changes are blocked");
    expect(banner).not.toHaveTextContent("preserved at");
  });

  it("falls back to a default message when none is provided", () => {
    render(<RecoveryBanner recovery={{ active: true }} />);
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Portier started without your saved configuration."
    );
  });
});
