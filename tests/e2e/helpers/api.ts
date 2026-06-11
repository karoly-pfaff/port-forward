// Direct REST API helpers used in beforeEach hooks to isolate test state.

export async function clearAllRules(baseURL: string): Promise<void> {
  const resp = await fetch(`${baseURL}/api/config/import`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode: "replace", config: { version: "1", rules: [] } }),
  });
  if (!resp.ok) {
    throw new Error(`clearAllRules failed with status ${resp.status}`);
  }
}

export interface RuleInput {
  name: string;
  protocol: "tcp" | "udp";
  listenHost: string;
  listenPort: number;
  targetHost: string;
  targetPort: number;
  enabled?: boolean;
  group?: string;
}

export async function createRule(
  baseURL: string,
  rule: RuleInput
): Promise<{ id: string }> {
  const resp = await fetch(`${baseURL}/api/forwards`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ...rule, enabled: rule.enabled ?? false }),
  });
  if (!resp.ok) {
    const body = await resp.text().catch(() => "");
    throw new Error(`createRule failed with status ${resp.status}: ${body}`);
  }
  return (await resp.json()) as { id: string };
}

export async function startRule(baseURL: string, id: string): Promise<void> {
  const resp = await fetch(`${baseURL}/api/forwards/${id}/start`, {
    method: "POST",
  });
  if (!resp.ok) {
    throw new Error(`startRule failed with status ${resp.status}`);
  }
}

export async function stopRule(baseURL: string, id: string): Promise<void> {
  const resp = await fetch(`${baseURL}/api/forwards/${id}/stop`, {
    method: "POST",
  });
  if (!resp.ok) {
    throw new Error(`stopRule failed with status ${resp.status}`);
  }
}
