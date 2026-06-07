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
    ];
    for (const path of expectedPaths) {
      expect(screen.queryAllByText(path).length).toBeGreaterThan(0);
    }
  });
});
