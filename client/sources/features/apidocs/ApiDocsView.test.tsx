import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ApiDocsView } from "./ApiDocsView.js";
import type { ApiDocsModel } from "./openApiDocs.js";

describe("ApiDocsView — driven by the real OpenAPI artifact", () => {
  it("renders the API Reference heading with the contract title and version", () => {
    render(<ApiDocsView />);
    expect(screen.getByText("API Reference")).toBeInTheDocument();
    expect(screen.getByText(/Portier API · v1\.19 — generated from the OpenAPI contract/)).toBeInTheDocument();
  });

  it("groups operations by tag (forwards, config, connections, health)", () => {
    render(<ApiDocsView />);
    for (const tag of ["forwards", "config", "connections", "health"]) {
      expect(screen.getByRole("heading", { name: tag })).toBeInTheDocument();
    }
  });

  it("renders representative endpoint paths from the contract", () => {
    render(<ApiDocsView />);
    expect(screen.getAllByText("/api/forwards").length).toBeGreaterThanOrEqual(2); // GET + POST
    expect(screen.getByText("/api/connections")).toBeInTheDocument();
    expect(screen.getByText("/api/config/plan")).toBeInTheDocument();
    expect(screen.getByText("/health")).toBeInTheDocument();
  });

  it("renders method badges", () => {
    render(<ApiDocsView />);
    expect(screen.getAllByText("GET").length).toBeGreaterThan(0);
    expect(screen.getAllByText("POST").length).toBeGreaterThan(0);
    expect(screen.getAllByText("PATCH").length).toBeGreaterThan(0);
    expect(screen.getAllByText("DELETE").length).toBeGreaterThan(0);
  });

  it("renders operation summaries and descriptions", () => {
    render(<ApiDocsView />);
    expect(screen.getByText("Create a forward rule")).toBeInTheDocument();
    expect(screen.getByText(/Creates a forward rule\./)).toBeInTheDocument();
  });

  it("renders query/path parameters for endpoints that declare them", () => {
    render(<ApiDocsView />);
    // GET /api/activity declares severity/type/ruleId/limit query params.
    const severity = screen.getByText("severity");
    expect(severity).toBeInTheDocument();
    expect(severity.closest("li")?.textContent).toMatch(/query/);
    // PATCH /api/forwards/{id} declares a required path param `id`.
    const idParams = screen.getAllByText("id");
    expect(idParams.some((el) => /path.*required/.test(el.closest("li")?.textContent ?? ""))).toBe(true);
  });

  it("summarises request bodies by schema name", () => {
    render(<ApiDocsView />);
    expect(screen.getByText("CreateForwardRuleBodyDto")).toBeInTheDocument();
  });

  it("renders response status codes and schema names", () => {
    render(<ApiDocsView />);
    expect(screen.getAllByText("201").length).toBeGreaterThan(0);
    expect(screen.getAllByText("400").length).toBeGreaterThan(0);
    expect(screen.getAllByText("ForwardRuleResponseDto").length).toBeGreaterThan(0);
  });

  it("does not render raw JSON (no $ref strings, no document blob)", () => {
    const { container } = render(<ApiDocsView />);
    expect(container.textContent).not.toMatch(/#\/components\/schemas\//);
    expect(container.textContent).not.toMatch(/\{\s*"openapi"/);
  });

  it("no longer shows the removed hand-maintained planned/parity badges", () => {
    render(<ApiDocsView />);
    expect(screen.queryByText(/Planned — v1\.4/)).not.toBeInTheDocument();
    expect(screen.queryByText(/parity pending/i)).not.toBeInTheDocument();
  });
});

describe("ApiDocsView — optional-field rendering (crafted model)", () => {
  // A valid model whose operation omits a summary, has a typeless query param, and a
  // non-required request body with a null response schema — shapes a valid OpenAPI doc
  // can produce but the Portier contract happens not to.
  const model: ApiDocsModel = {
    title: "Crafted",
    version: "2",
    description: "Crafted doc.",
    groups: [
      {
        name: "group-one",
        operations: [
          {
            method: "GET",
            path: "/x",
            summary: null,
            description: "Reads X.",
            parameters: [{ name: "p", location: "query", required: false, type: null, description: "A param." }],
            requestBody: { schema: "BodyDto", required: false },
            responses: [{ status: "200", description: "OK", schema: null }]
          }
        ]
      }
    ]
  };

  it("renders an operation without a summary (description still shown)", () => {
    render(<ApiDocsView model={model} />);
    expect(screen.getByText("/x")).toBeInTheDocument();
    expect(screen.getByText("Reads X.")).toBeInTheDocument();
  });

  it("renders a parameter label without a type facet", () => {
    render(<ApiDocsView model={model} />);
    const param = screen.getByText("p");
    expect(param.closest("li")?.textContent).toMatch(/\(query\)/);
  });

  it("renders a non-required request body without the (required) marker", () => {
    render(<ApiDocsView model={model} />);
    const bodyLine = screen.getByText("BodyDto").closest("p");
    expect(bodyLine?.textContent).toMatch(/Request body:/);
    expect(bodyLine?.textContent).not.toMatch(/\(required\)/);
  });

  it("renders a response line without a schema code", () => {
    render(<ApiDocsView model={model} />);
    const line = screen.getByText("200").closest("li");
    expect(line?.textContent).toMatch(/OK/);
    expect(line?.querySelector("code")).toBeNull(); // null schema → no <code> badge
  });
});
