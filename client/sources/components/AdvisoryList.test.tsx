import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { PortAdvisory } from "@portier/shared";
import { AdvisoryList } from "./AdvisoryList.js";

const lanAdvisory: PortAdvisory = {
  code: "LAN_EXPOSURE",
  severity: "warning",
  message: "Listening on 0.0.0.0 exposes this forwarded port on all interfaces."
};

const privilegedAdvisory: PortAdvisory = {
  code: "PRIVILEGED_PORT",
  severity: "danger",
  message: "Port 80 is privileged and may require elevated permissions."
};

const infoAdvisory: PortAdvisory = {
  code: "OUTSIDE_RECOMMENDED_RANGE",
  severity: "info",
  message: "Port 3000 is outside the recommended forwarding range."
};

describe("AdvisoryList", () => {
  it("renders null for empty advisories", () => {
    const { container } = render(<AdvisoryList advisories={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders a single advisory with severity class", () => {
    render(<AdvisoryList advisories={[lanAdvisory]} />);
    const p = screen.getByText(lanAdvisory.message);
    expect(p).toBeInTheDocument();
    expect(p.className).toContain("warning");
  });

  it("renders advisory code as part of class", () => {
    render(<AdvisoryList advisories={[privilegedAdvisory]} />);
    const p = screen.getByText(privilegedAdvisory.message);
    expect(p.className).toContain("danger");
  });

  it("renders info severity advisory", () => {
    render(<AdvisoryList advisories={[infoAdvisory]} />);
    const p = screen.getByText(infoAdvisory.message);
    expect(p.className).toContain("info");
  });

  it("renders multiple advisories", () => {
    render(<AdvisoryList advisories={[lanAdvisory, privilegedAdvisory, infoAdvisory]} />);
    expect(screen.getByText(lanAdvisory.message)).toBeInTheDocument();
    expect(screen.getByText(privilegedAdvisory.message)).toBeInTheDocument();
    expect(screen.getByText(infoAdvisory.message)).toBeInTheDocument();
  });

  it("adds compact class when compact prop is true", () => {
    render(<AdvisoryList advisories={[lanAdvisory]} compact={true} />);
    const p = screen.getByText(lanAdvisory.message);
    expect(p.className).toContain("compact");
  });

  it("does not add compact class by default", () => {
    render(<AdvisoryList advisories={[lanAdvisory]} />);
    const p = screen.getByText(lanAdvisory.message);
    expect(p.className).not.toContain("compact");
  });
});
