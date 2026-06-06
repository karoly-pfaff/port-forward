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

  it("lists the GET /api/activity endpoint", () => {
    render(<ApiDocsView />);
    expect(screen.getByText("/api/activity")).toBeInTheDocument();
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
});
