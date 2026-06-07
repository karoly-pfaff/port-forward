import type {
  ActivityEvent,
  ActivityEventType,
  ActivitySeverity,
  ExportedConfig,
  ForwardRule,
  ForwardRuleInput,
  ForwardRuleResponse,
  ForwardStatus,
  ImportMode,
  ImportResult,
  PortAdvisory,
  RuleDiagnosticsResult,
  RuntimeInfo
} from "@portier/shared";

export async function fetchForwardRules(): Promise<ForwardRuleResponse[]> {
  const response = await fetch("/api/forwards");
  await ensureOk(response);
  return (await response.json()) as ForwardRuleResponse[];
}

export async function fetchForwardStatus(): Promise<ForwardStatus[]> {
  const response = await fetch("/api/status");
  await ensureOk(response);
  return (await response.json()) as ForwardStatus[];
}

export async function saveForwardRule(id: string | undefined, payload: ForwardRuleInput): Promise<ForwardRuleResponse> {
  const response = await fetch(id ? `/api/forwards/${id}` : "/api/forwards", {
    method: id ? "PATCH" : "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
  await ensureOk(response);
  return (await response.json()) as ForwardRuleResponse;
}

export async function deleteForwardRule(rule: ForwardRule): Promise<void> {
  const response = await fetch(`/api/forwards/${rule.id}`, { method: "DELETE" });
  await ensureOk(response);
}

export async function setForwardRuleRunning(rule: ForwardRule, running: boolean): Promise<ForwardStatus> {
  const response = await fetch(`/api/forwards/${rule.id}/${running ? "start" : "stop"}`, { method: "POST" });
  await ensureOk(response);
  return (await response.json()) as ForwardStatus;
}

export async function reorderForwardRules(ids: string[]): Promise<ForwardRuleResponse[]> {
  const response = await fetch("/api/forwards/reorder", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ids })
  });
  await ensureOk(response);
  return (await response.json()) as ForwardRuleResponse[];
}

export async function fetchPortAdvisories(input: {
  port: number;
  listenHost?: string;
  purpose: "forward" | "management";
}): Promise<PortAdvisory[]> {
  const params = new URLSearchParams({
    port: String(input.port),
    purpose: input.purpose
  });

  if (input.listenHost) {
    params.set("listenHost", input.listenHost);
  }

  const response = await fetch(`/api/ports/advisory?${params.toString()}`);
  await ensureOk(response);
  return (await response.json()) as PortAdvisory[];
}

export interface FetchActivityParams {
  limit?: number;
  ruleId?: string;
  type?: ActivityEventType;
  severity?: ActivitySeverity;
}

export async function fetchActivity(params: FetchActivityParams = {}): Promise<ActivityEvent[]> {
  const query = new URLSearchParams();
  if (params.limit !== undefined) query.set("limit", String(params.limit));
  if (params.ruleId !== undefined) query.set("ruleId", params.ruleId);
  if (params.type !== undefined) query.set("type", params.type);
  if (params.severity !== undefined) query.set("severity", params.severity);

  const qs = query.toString();
  const response = await fetch(`/api/activity${qs ? `?${qs}` : ""}`);
  await ensureOk(response);
  const body = (await response.json()) as { events: ActivityEvent[] };
  return body.events;
}

export async function diagnoseForwardRule(ruleId: string): Promise<RuleDiagnosticsResult> {
  const response = await fetch(`/api/forwards/${ruleId}/diagnose`, { method: "POST" });
  await ensureOk(response);
  return (await response.json()) as RuleDiagnosticsResult;
}

export async function fetchRuntimeInfo(): Promise<RuntimeInfo> {
  const response = await fetch("/api/runtime");
  await ensureOk(response);
  return (await response.json()) as RuntimeInfo;
}

export async function exportConfig(): Promise<ExportedConfig> {
  const response = await fetch("/api/config/export");
  await ensureOk(response);
  return (await response.json()) as ExportedConfig;
}

export interface ImportConfigResult {
  result: ImportResult;
  rules: ForwardRuleResponse[];
}

export async function importConfig(
  config: ExportedConfig,
  mode: ImportMode
): Promise<ImportConfigResult> {
  const response = await fetch("/api/config/import", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode, config })
  });
  await ensureOk(response);
  return (await response.json()) as ImportConfigResult;
}

async function ensureOk(response: Response): Promise<void> {
  if (response.ok) {
    return;
  }

  const body = (await response.json().catch(() => undefined)) as { errors?: string[] } | undefined;
  throw new Error(body?.errors?.join(" ") ?? `Request failed with ${response.status}.`);
}
