import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ApiDocsView } from "./ApiDocsView.js";

describe("ApiDocsView", () => {
  it("renders the API Reference heading", () => {
    render(<ApiDocsView />);
    expect(screen.getByText("API Reference")).toBeInTheDocument();
  });

  it("lists the GET and POST /api/forwards endpoints", () => {
    render(<ApiDocsView />);
    // Both GET and POST /api/forwards are listed
    expect(screen.getAllByText("/api/forwards").length).toBeGreaterThanOrEqual(2);
  });

  it("lists the POST /api/forwards/reorder endpoint", () => {
    render(<ApiDocsView />);
    expect(screen.getByText("/api/forwards/reorder")).toBeInTheDocument();
  });

  it("lists the GET /api/status endpoint", () => {
    render(<ApiDocsView />);
    expect(screen.getByText("/api/status")).toBeInTheDocument();
  });

  it("lists both GET and DELETE /api/activity endpoints", () => {
    render(<ApiDocsView />);
    expect(screen.getAllByText("/api/activity").length).toBeGreaterThanOrEqual(2);
  });

  it("describes DELETE /api/activity as clearing the log", () => {
    render(<ApiDocsView />);
    expect(screen.getByText(/Clear the in-memory activity log/)).toBeInTheDocument();
  });

  it("lists the config export and import endpoints", () => {
    render(<ApiDocsView />);
    expect(screen.getByText("/api/config/export")).toBeInTheDocument();
    expect(screen.getByText("/api/config/import")).toBeInTheDocument();
  });

  it("lists the port advisory endpoint", () => {
    render(<ApiDocsView />);
    expect(screen.getByText("/api/ports/advisory")).toBeInTheDocument();
  });

  it("shows GET method badges", () => {
    render(<ApiDocsView />);
    const gets = screen.getAllByText("GET");
    expect(gets.length).toBeGreaterThan(0);
  });

  it("shows POST method badges", () => {
    render(<ApiDocsView />);
    const posts = screen.getAllByText("POST");
    expect(posts.length).toBeGreaterThan(0);
  });

  it("shows the reorder endpoint does not restart running rules", () => {
    render(<ApiDocsView />);
    expect(screen.getByText(/Does not restart running rules/)).toBeInTheDocument();
  });

  it("lists the POST /api/forwards/:id/diagnose endpoint", () => {
    render(<ApiDocsView />);
    expect(screen.getByText("/api/forwards/:id/diagnose")).toBeInTheDocument();
  });

  it("describes that diagnostics do not mutate rule state", () => {
    render(<ApiDocsView />);
    expect(screen.getByText(/without changing its state/i)).toBeInTheDocument();
  });

  it("shows diagnostic status values including pass, warn, fail, and skip", () => {
    const { container } = render(<ApiDocsView />);
    expect(container.textContent).toMatch(/pass.*warn.*fail.*skip/i);
  });

  it("includes TCP and UDP diagnostic behavior notes", () => {
    const { container } = render(<ApiDocsView />);
    expect(container.textContent).toMatch(/TCP:.*target-connect/i);
    expect(container.textContent).toMatch(/UDP:.*target-connect.*skip/i);
  });

  it("lists the GET /api/runtime endpoint", () => {
    render(<ApiDocsView />);
    expect(screen.getByText("/api/runtime")).toBeInTheDocument();
  });

  it("documents all expected public API endpoints", () => {
    render(<ApiDocsView />);
    const expectedPaths = [
      "/api/forwards",
      "/api/forwards/:id/diagnose",
      "/api/forwards/reorder",
      "/api/status",
      "/api/runtime",
      "/api/activity",
      "/api/config/export",
      "/api/config/import",
      "/api/ports/advisory",
      "/api/connections",
      "/api/config/plan",
      "/api/config/apply",
    ];
    for (const path of expectedPaths) {
      expect(screen.queryAllByText(path).length).toBeGreaterThan(0);
    }
  });

  it("lists the GET /api/connections endpoint", () => {
    render(<ApiDocsView />);
    expect(screen.getByText("/api/connections")).toBeInTheDocument();
  });

  it("does not show a Planned badge for GET /api/connections", () => {
    render(<ApiDocsView />);
    expect(screen.queryByText("Planned — v1.4")).not.toBeInTheDocument();
  });

  it("describes GET /api/connections purpose", () => {
    render(<ApiDocsView />);
    expect(screen.getByText(/active TCP connections and UDP sessions/i)).toBeInTheDocument();
  });

  it("documents LiveConnectionsResponse fields for GET /api/connections", () => {
    const { container } = render(<ApiDocsView />);
    expect(container.textContent).toMatch(/LiveConnectionsResponse/);
    expect(container.textContent).toMatch(/tcpConnections/);
    expect(container.textContent).toMatch(/udpSessions/);
    expect(container.textContent).toMatch(/ruleSummaries/);
  });

  it("notes that GET /api/connections returns empty arrays when nothing is active", () => {
    const { container } = render(<ApiDocsView />);
    expect(container.textContent).toMatch(/empty arrays when nothing is active/i);
  });

  it("lists the POST /api/config/plan endpoint without a parity badge (both runtimes implement it)", () => {
    render(<ApiDocsView />);
    expect(screen.getByText("/api/config/plan")).toBeInTheDocument();
    expect(screen.queryByText(/TypeScript server.*Go service parity pending/i)).not.toBeInTheDocument();
  });

  it("describes POST /api/config/plan as read-only", () => {
    render(<ApiDocsView />);
    expect(screen.getByText(/does not modify state/i)).toBeInTheDocument();
  });

  it("documents ConfigPlanResponse fields for POST /api/config/plan", () => {
    const { container } = render(<ApiDocsView />);
    expect(container.textContent).toMatch(/ConfigPlanResponse/);
    expect(container.textContent).toMatch(/ConfigPlanSummary/);
    expect(container.textContent).toMatch(/ConfigPlanOperation/);
    expect(container.textContent).toMatch(/hasDrift/);
  });

  it("lists the POST /api/config/apply endpoint with a Planned v1.5 badge", () => {
    render(<ApiDocsView />);
    expect(screen.getByText("/api/config/apply")).toBeInTheDocument();
    expect(screen.getByText("Planned — v1.5")).toBeInTheDocument();
  });

  it("describes POST /api/config/apply as requiring yes: true", () => {
    const { container } = render(<ApiDocsView />);
    expect(container.textContent).toMatch(/yes: true/);
  });
});
