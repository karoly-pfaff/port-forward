import type { ActivityEvent, ForwardRuleResponse, ForwardStatus, RuleDiagnosticsResult, RuntimeInfo } from "@portier/shared";
import { PORTIER_APP_VERSION, PORTIER_DEFAULT_HOST, PORTIER_DEFAULT_PORT } from "@portier/shared";
import { fetchActivity, fetchForwardRules, fetchForwardStatus, fetchRuntimeInfo } from "../../api/portierApi.js";
import type { DiagnosisEntry } from "../forwards/ForwardRuleList.js";

export interface DiagnosticsBundle {
  schemaVersion: "1";
  exportedAt: string;
  app: {
    name: string;
    version: string;
  };
  runtime: RuntimeInfo | null;
  rules: ForwardRuleResponse[];
  statuses: ForwardStatus[];
  diagnostics: Record<string, RuleDiagnosticsResult>;
  diagnosticsNote?: string;
  activity: {
    included: boolean;
    events: ActivityEvent[];
    note: string;
  };
  metadata: {
    managementUrl: string;
    source: "client";
    generatedBy: "settings";
  };
  errors?: Array<{ source: string; message: string }>;
}

export function buildDiagnosticsFilename(): string {
  const d = new Date();
  const pad = (n: number) => String(n).padStart(2, "0");
  const date = `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}`;
  const time = `${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`;
  return `portier-diagnostics-${date}-${time}.json`;
}

export async function buildDiagnosticsBundle(
  diagnosisMap: ReadonlyMap<string, DiagnosisEntry>
): Promise<DiagnosticsBundle> {
  const errors: Array<{ source: string; message: string }> = [];

  const [runtimeResult, rulesResult, statusesResult, activityResult] = await Promise.allSettled([
    fetchRuntimeInfo(),
    fetchForwardRules(),
    fetchForwardStatus(),
    fetchActivity({ limit: 100 })
  ]);

  const runtime = runtimeResult.status === "fulfilled" ? runtimeResult.value : null;
  if (runtimeResult.status === "rejected") {
    errors.push({ source: "runtime", message: toErrorMessage(runtimeResult.reason) });
  }

  const rules = rulesResult.status === "fulfilled" ? rulesResult.value : [];
  if (rulesResult.status === "rejected") {
    errors.push({ source: "rules", message: toErrorMessage(rulesResult.reason) });
  }

  const statuses = statusesResult.status === "fulfilled" ? statusesResult.value : [];
  if (statusesResult.status === "rejected") {
    errors.push({ source: "statuses", message: toErrorMessage(statusesResult.reason) });
  }

  const activityEvents = activityResult.status === "fulfilled" ? activityResult.value : [];
  if (activityResult.status === "rejected") {
    errors.push({ source: "activity", message: toErrorMessage(activityResult.reason) });
  }

  const diagnostics: Record<string, RuleDiagnosticsResult> = {};
  for (const [ruleId, entry] of diagnosisMap) {
    if (entry.state === "done") {
      diagnostics[ruleId] = entry.result;
    }
  }

  const managementUrl = runtime
    ? `http://${runtime.managementHost}:${runtime.managementPort}`
    : `http://${PORTIER_DEFAULT_HOST}:${PORTIER_DEFAULT_PORT}`;

  const bundle: DiagnosticsBundle = {
    schemaVersion: "1",
    exportedAt: new Date().toISOString(),
    app: {
      name: "Portier",
      version: runtime?.version ?? PORTIER_APP_VERSION
    },
    runtime,
    rules,
    statuses,
    diagnostics,
    activity: {
      included: true,
      events: activityEvents,
      note: "Packet events may be throttled in the activity log; connection and status counters remain exact. Up to 100 recent events included."
    },
    metadata: {
      managementUrl,
      source: "client",
      generatedBy: "settings"
    }
  };

  if (Object.keys(diagnostics).length === 0) {
    bundle.diagnosticsNote = "No rule diagnostics had been run in this UI session.";
  }

  if (errors.length > 0) {
    bundle.errors = errors;
  }

  return bundle;
}

export function downloadJson(filename: string, data: unknown): void {
  const json = JSON.stringify(data, null, 2);
  const blob = new Blob([json], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

function toErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Unknown error.";
}
