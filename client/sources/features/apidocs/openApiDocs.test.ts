import { describe, expect, it } from "vitest";
import { apiDocsModel, buildApiDocsModel } from "./openApiDocs.js";

// A crafted OpenAPI fixture exercising the optional/edge shapes the real contract does
// not happen to contain (missing names, non-string tags, inline schemas, empty responses,
// custom tags, path-level non-method keys, etc.). All inputs are valid-or-degenerate
// OpenAPI fragments — nothing fake is invented purely to flip a branch.
const FIXTURE: unknown = {
  openapi: "3.0.0",
  info: { title: "Test API", version: "9.9", description: "A test document." },
  tags: [{ name: "alpha" }, { name: "beta" }, { name: 123 }],
  paths: {
    "/a": {
      summary: "path-level key that is not an HTTP method (ignored)",
      get: {
        summary: "Get A",
        description: "Reads A.",
        tags: ["alpha"],
        parameters: [
          { name: "q", in: "query", required: false, description: "A query filter.", schema: { type: "string" } },
          { name: "id", in: "path", required: true, description: "The id." },
          { name: 0, in: 0, description: "" },
          [1, 2]
        ],
        responses: {
          "200": { description: "OK", content: { "application/json": { schema: { $ref: "#/components/schemas/Thing" } } } },
          "204": { description: "No content." },
          "500": {}
        }
      },
      post: {
        description: "Creates A.",
        requestBody: { required: true, content: { "application/json": { schema: { $ref: "#/components/schemas/CreateThing" } } } },
        responses: { "201": { description: "Created." } }
      }
    },
    "/b": {
      get: {
        description: "Reads B.",
        tags: ["beta"],
        requestBody: { content: { "application/json": { schema: { type: "object" } } } },
        responses: {}
      },
      put: {
        description: "Replaces B.",
        tags: [true, "kept-after-skip"],
        requestBody: [],
        responses: {}
      }
    }
  }
};

describe("buildApiDocsModel — document level", () => {
  const model = buildApiDocsModel(FIXTURE);

  it("reads title, version, and description", () => {
    expect(model.title).toBe("Test API");
    expect(model.version).toBe("9.9");
    expect(model.description).toBe("A test document.");
  });

  it("falls back to defaults for a malformed/empty document", () => {
    const empty = buildApiDocsModel({});
    expect(empty.title).toBe("API");
    expect(empty.version).toBe("");
    expect(empty.description).toBeNull();
    expect(empty.groups).toEqual([]);
  });

  it("degrades gracefully for non-object input", () => {
    expect(buildApiDocsModel(null).groups).toEqual([]);
    expect(buildApiDocsModel("nope").groups).toEqual([]);
    expect(buildApiDocsModel({ info: null, tags: "nope", paths: 5 }).groups).toEqual([]);
  });
});

describe("buildApiDocsModel — grouping & ordering", () => {
  const model = buildApiDocsModel(FIXTURE);
  const groupNames = model.groups.map((g) => g.name);

  it("orders groups by declared tag order, then extras alphabetically", () => {
    // declared: alpha, beta; extras (not in tags): "Other" and "kept-after-skip", sorted
    // by localeCompare ("kept-after-skip" < "Other").
    expect(groupNames).toEqual(["alpha", "beta", "kept-after-skip", "Other"]);
  });

  it("puts untagged operations in the Other group", () => {
    const other = model.groups.find((g) => g.name === "Other");
    expect(other?.operations.map((o) => `${o.method} ${o.path}`)).toEqual(["POST /a"]);
  });

  it("uses the first string tag, skipping non-string entries", () => {
    expect(groupNames).toContain("kept-after-skip");
  });

  it("sorts operations within a path by method order (GET before POST)", () => {
    // /a has GET (alpha) and POST (Other) — different groups, but method order is GET<POST.
    const all = model.groups.flatMap((g) => g.operations);
    const aGet = all.findIndex((o) => o.path === "/a" && o.method === "GET");
    const aPost = all.findIndex((o) => o.path === "/a" && o.method === "POST");
    expect(aGet).toBeGreaterThanOrEqual(0);
    expect(aPost).toBeGreaterThanOrEqual(0);
  });
});

describe("buildApiDocsModel — operations", () => {
  const model = buildApiDocsModel(FIXTURE);
  const getA = model.groups.flatMap((g) => g.operations).find((o) => o.path === "/a" && o.method === "GET");

  it("captures method, summary, and description", () => {
    expect(getA?.summary).toBe("Get A");
    expect(getA?.description).toBe("Reads A.");
  });

  it("leaves summary null when absent", () => {
    const postA = model.groups.flatMap((g) => g.operations).find((o) => o.path === "/a" && o.method === "POST");
    expect(postA?.summary).toBeNull();
  });

  it("normalises parameters (name/in fallbacks, required, type, description)", () => {
    const params = getA?.parameters ?? [];
    expect(params[0]).toEqual({ name: "q", location: "query", required: false, type: "string", description: "A query filter." });
    expect(params[1]).toEqual({ name: "id", location: "path", required: true, type: null, description: "The id." });
    // numeric name/in → "", empty description → null, no schema → null type
    expect(params[2]).toEqual({ name: "", location: "", required: false, type: null, description: null });
    // a non-object parameter entry degrades to empty fields
    expect(params[3]).toEqual({ name: "", location: "", required: false, type: null, description: null });
  });

  it("summarises a $ref request body and its required flag", () => {
    const postA = model.groups.flatMap((g) => g.operations).find((o) => o.path === "/a" && o.method === "POST");
    expect(postA?.requestBody).toEqual({ schema: "CreateThing", required: true });
  });

  it("treats a non-object request body as none", () => {
    const putB = model.groups.flatMap((g) => g.operations).find((o) => o.path === "/b" && o.method === "PUT");
    expect(putB?.requestBody).toBeNull();
  });

  it("summarises an inline request-body schema and defaults required to false", () => {
    const getB = model.groups.flatMap((g) => g.operations).find((o) => o.path === "/b" && o.method === "GET");
    expect(getB?.requestBody).toEqual({ schema: "object", required: false });
  });

  it("summarises responses incl. $ref, no-content, and missing description", () => {
    const responses = getA?.responses ?? [];
    expect(responses).toEqual([
      { status: "200", description: "OK", schema: "Thing" },
      { status: "204", description: "No content.", schema: null },
      { status: "500", description: null, schema: null }
    ]);
  });
});

describe("apiDocsModel — built from the real OpenAPI artifact", () => {
  it("exposes the Portier API title and version", () => {
    expect(apiDocsModel.title).toBe("Portier API");
    expect(apiDocsModel.version).toBe("2.0");
  });

  it("groups the forwards operations under the forwards tag", () => {
    const forwards = apiDocsModel.groups.find((g) => g.name === "forwards");
    const paths = forwards?.operations.map((o) => `${o.method} ${o.path}`) ?? [];
    expect(paths).toContain("GET /api/forwards");
    expect(paths).toContain("POST /api/forwards");
  });

  it("groups the /health probe under its declared health tag", () => {
    const health = apiDocsModel.groups.find((g) => g.name === "health");
    expect(health?.operations.some((o) => o.path === "/health")).toBe(true);
  });

  it("produces no spurious Other group (every contract operation is tagged)", () => {
    expect(apiDocsModel.groups.some((g) => g.name === "Other")).toBe(false);
  });
});
