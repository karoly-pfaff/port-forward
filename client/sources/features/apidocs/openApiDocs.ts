// OpenAPI → API-docs view model.
//
// Consumes the canonical generated OpenAPI artifact (docs/openapi.json — the single
// source of truth, validated by `npm run validate:openapi:go`) and converts it into a
// small, stable, display-friendly view model. There is no hand-maintained endpoint list:
// the API docs view is derived entirely from the contract, so it cannot drift.
//
// The input is treated as `unknown` and narrowed with type guards (no `any`, no unsafe
// casts) so a malformed document degrades gracefully instead of throwing.

import openApiDocument from "../../../../docs/openapi.json";

export interface ApiParameter {
  name: string;
  location: string; // "query" | "path" | "header" | ...
  required: boolean;
  type: string | null;
  description: string | null;
}

export interface ApiRequestBody {
  schema: string | null;
  required: boolean;
}

export interface ApiResponse {
  status: string;
  description: string | null;
  schema: string | null;
}

export interface ApiOperation {
  method: string; // upper-case, e.g. "GET"
  path: string;
  summary: string | null;
  description: string | null;
  parameters: ApiParameter[];
  requestBody: ApiRequestBody | null;
  responses: ApiResponse[];
}

export interface ApiGroup {
  name: string;
  operations: ApiOperation[];
}

export interface ApiDocsModel {
  title: string;
  version: string;
  description: string | null;
  groups: ApiGroup[];
}

// Methods we surface, in canonical display/sort order. Any other path-item key
// (parameters, $ref, summary, …) is ignored.
const HTTP_METHODS = ["get", "post", "put", "patch", "delete", "options", "head"];
const OTHER_GROUP = "Other";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function asRecord(value: unknown): Record<string, unknown> {
  return isRecord(value) ? value : {};
}

function asString(value: unknown): string | null {
  return typeof value === "string" && value.length > 0 ? value : null;
}

function asArray(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

// "#/components/schemas/ForwardRuleResponseDto" → "ForwardRuleResponseDto"; an inline
// schema falls back to its `type` (e.g. "object"); anything else → null.
function schemaName(schema: unknown): string | null {
  const record = asRecord(schema);
  const ref = asString(record.$ref);
  if (ref !== null) return ref.slice(ref.lastIndexOf("/") + 1);
  return asString(record.type);
}

function buildParameter(raw: unknown): ApiParameter {
  const param = asRecord(raw);
  return {
    name: asString(param.name) ?? "",
    location: asString(param.in) ?? "",
    required: param.required === true,
    type: asString(asRecord(param.schema).type),
    description: asString(param.description),
  };
}

function buildRequestBody(raw: unknown): ApiRequestBody | null {
  if (!isRecord(raw)) return null;
  const json = asRecord(asRecord(raw.content)["application/json"]);
  return { schema: schemaName(json.schema), required: raw.required === true };
}

function buildResponses(raw: unknown): ApiResponse[] {
  const responses = asRecord(raw);
  return Object.keys(responses).map((status) => {
    const response = asRecord(responses[status]);
    const json = asRecord(asRecord(response.content)["application/json"]);
    return { status, description: asString(response.description), schema: schemaName(json.schema) };
  });
}

function firstTag(raw: unknown): string | null {
  for (const tag of asArray(raw)) {
    const name = asString(tag);
    if (name !== null) return name;
  }
  return null;
}

function buildOperation(method: string, path: string, raw: unknown): ApiOperation {
  const op = asRecord(raw);
  return {
    method: method.toUpperCase(),
    path,
    summary: asString(op.summary),
    description: asString(op.description),
    parameters: asArray(op.parameters).map(buildParameter),
    requestBody: buildRequestBody(op.requestBody),
    responses: buildResponses(op.responses),
  };
}

interface TaggedOperation {
  tag: string;
  op: ApiOperation;
}

function collectOperations(pathsRaw: unknown): TaggedOperation[] {
  const paths = asRecord(pathsRaw);
  const result: TaggedOperation[] = [];
  for (const path of Object.keys(paths)) {
    const item = asRecord(paths[path]);
    for (const method of Object.keys(item)) {
      if (!HTTP_METHODS.includes(method)) continue;
      const raw = item[method];
      result.push({ tag: firstTag(asRecord(raw).tags) ?? OTHER_GROUP, op: buildOperation(method, path, raw) });
    }
  }
  return result;
}

function groupRank(name: string, tagOrder: string[]): number {
  const index = tagOrder.indexOf(name);
  return index === -1 ? tagOrder.length : index;
}

function compareOperations(a: ApiOperation, b: ApiOperation): number {
  if (a.path !== b.path) return a.path.localeCompare(b.path);
  return HTTP_METHODS.indexOf(a.method.toLowerCase()) - HTTP_METHODS.indexOf(b.method.toLowerCase());
}

function buildGroups(entries: TaggedOperation[], tagOrder: string[]): ApiGroup[] {
  const groups: ApiGroup[] = [];
  for (const { tag, op } of entries) {
    let group = groups.find((candidate) => candidate.name === tag);
    if (group === undefined) {
      group = { name: tag, operations: [] };
      groups.push(group);
    }
    group.operations.push(op);
  }
  groups.sort((a, b) => {
    const rankA = groupRank(a.name, tagOrder);
    const rankB = groupRank(b.name, tagOrder);
    if (rankA !== rankB) return rankA - rankB;
    return a.name.localeCompare(b.name);
  });
  for (const group of groups) group.operations.sort(compareOperations);
  return groups;
}

export function buildApiDocsModel(raw: unknown): ApiDocsModel {
  const doc = asRecord(raw);
  const info = asRecord(doc.info);
  const tagOrder = asArray(doc.tags)
    .map((tag) => asString(asRecord(tag).name))
    .filter((name): name is string => name !== null);
  return {
    title: asString(info.title) ?? "API",
    version: asString(info.version) ?? "",
    description: asString(info.description),
    groups: buildGroups(collectOperations(doc.paths), tagOrder),
  };
}

// Built once from the canonical artifact at module load.
export const apiDocsModel: ApiDocsModel = buildApiDocsModel(openApiDocument);
