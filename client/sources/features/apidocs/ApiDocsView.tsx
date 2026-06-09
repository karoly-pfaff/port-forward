import { type ReactElement } from "react";

interface EndpointDoc {
  method: string;
  path: string;
  purpose: string;
  params?: string;
  response?: string;
  notes?: string;
  planned?: string;
}

const ENDPOINTS: EndpointDoc[] = [
  {
    method: "GET",
    path: "/api/forwards",
    purpose: "List all forwarding rules with port advisories.",
    response: "ForwardRuleResponse[]"
  },
  {
    method: "POST",
    path: "/api/forwards",
    purpose: "Create a new forwarding rule.",
    params: "Body: ForwardRuleInput",
    response: "201 ForwardRuleResponse · 400 validation · 409 duplicate binding"
  },
  {
    method: "PATCH",
    path: "/api/forwards/:id",
    purpose: "Update an existing rule. Forwarding-affecting changes restart a running rule; name/enabled changes do not.",
    params: "Body: Partial ForwardRuleInput",
    response: "ForwardRuleResponse · 400 · 404 · 409"
  },
  {
    method: "DELETE",
    path: "/api/forwards/:id",
    purpose: "Delete a rule (stops it first if running).",
    response: "204 No Content · 404"
  },
  {
    method: "POST",
    path: "/api/forwards/:id/start",
    purpose: "Start a forwarding rule.",
    response: "ForwardStatus · 404 · 500"
  },
  {
    method: "POST",
    path: "/api/forwards/:id/stop",
    purpose: "Stop a forwarding rule.",
    response: "ForwardStatus · 404"
  },
  {
    method: "POST",
    path: "/api/forwards/:id/diagnose",
    purpose:
      "Diagnose an existing forwarding rule without changing its state. Does not start, stop, or modify the rule.",
    params: "Path: id (rule ID). No request body required.",
    response:
      "RuleDiagnosticsResult { ruleId, ruleName, protocol, summary, checks[], diagnosedAt } · 404 when rule not found",
    notes:
      "Status values: pass · warn · fail · skip. Check IDs: listen-host · listen-bind · target-host · target-connect · udp-mode · lan-exposure · privileged-port · common-port. TCP: target-connect attempts a short connection to the target. UDP: target-connect is always skip (UDP reachability cannot be verified). Running rule: listen-bind returns pass if Portier owns the socket."
  },
  {
    method: "POST",
    path: "/api/forwards/reorder",
    purpose: "Reorder rules by providing an array of IDs in the desired order. Does not restart running rules.",
    params: "Body: { ids: string[] }",
    response: "ForwardRuleResponse[] (updated order)"
  },
  {
    method: "GET",
    path: "/api/status",
    purpose: "List runtime status for all rules.",
    response: "ForwardStatus[]"
  },
  {
    method: "GET",
    path: "/api/runtime",
    purpose: "Expose runtime environment details for the local management UI. Added in v1.2.",
    response:
      "RuntimeInfo { name, version, runtime, platform, arch, uptimeSeconds, startedAt, managementHost, managementPort, configPath, staticDir, serviceMode, pid }"
  },
  {
    method: "GET",
    path: "/api/activity",
    purpose: "List recent activity events (in-memory, resets on restart, max 500).",
    params: "Query: limit (default 100, max 500), ruleId, type, severity",
    response: "{ events: ActivityEvent[] } — newest first"
  },
  {
    method: "DELETE",
    path: "/api/activity",
    purpose: "Clear the in-memory activity log. Does not affect rules or forwarding state.",
    response: "204 No Content"
  },
  {
    method: "GET",
    path: "/api/config/export",
    purpose: "Export current rules as a portable JSON config.",
    response: "{ version, exportedAt, rules }"
  },
  {
    method: "POST",
    path: "/api/config/import",
    purpose: "Import rules from a config. Validates all rules before applying; no partial import on error.",
    params: "Body: { mode: 'replace' | 'merge', config: ExportedConfig }",
    response: "{ result: ImportResult, rules: ForwardRuleResponse[] } · 400 bad body · 422 validation errors"
  },
  {
    method: "GET",
    path: "/api/ports/advisory",
    purpose: "Get port advisory messages for a given port, host, and purpose.",
    params: "Query: port (1–65535), purpose (forward | management), listenHost (optional)",
    response: "PortAdvisory[]"
  },
  {
    method: "GET",
    path: "/api/connections",
    purpose:
      "Return a read-only snapshot of active TCP connections and UDP sessions for all running forwarding rules, along with per-rule live traffic summaries.",
    response:
      "LiveConnectionsResponse { generatedAt, tcpConnections[], udpSessions[], ruleSummaries[] }",
    notes:
      "tcpConnections, udpSessions, and ruleSummaries are always arrays; empty arrays when nothing is active. TCP status: active. UDP status: active | idle (idle after 30s, retained up to 5min). bytesIn = client-to-target; bytesOut = target-to-client. IDs are runtime-local and do not persist across restarts. Payload contents are never exposed."
  },
  {
    method: "POST",
    path: "/api/config/plan",
    purpose:
      "Compare a desired config against the currently running configuration and return a structured plan showing adds, updates, removes, and unchanged rules. Read-only — does not modify state.",
    params: "Body: ConfigPlanRequest { desired: { rules: ForwardRuleInput[] } }",
    response:
      "ConfigPlanResponse { generatedAt, mode, summary: ConfigPlanSummary, operations: ConfigPlanOperation[], errors: ConfigPlanError[], warnings: ConfigPlanWarning[] }",
    notes:
      "Rules are matched by stable rule id when present; otherwise by protocol+listenHost+listenPort identity. Ambiguous matches produce an error and refuse apply. hasDrift is true when any operation is add, update, or remove. destructive is true when a remove or forwarding-affecting update is present. Does not mutate running config.",
    planned: "v1.5"
  },
  {
    method: "POST",
    path: "/api/config/apply",
    purpose:
      "Apply a desired config to the running configuration after explicit confirmation. Supports dry-run and backup before apply.",
    params:
      "Body: ConfigApplyRequest { desired: { rules: ForwardRuleInput[] }, yes: true, backup?: boolean }",
    response:
      "ConfigApplyResponse { appliedAt, applied, errors: ConfigPlanError[], warnings: ConfigPlanWarning[] }",
    notes:
      "requires yes: true for destructive operations. Use POST /api/config/plan first to preview. backup: true returns the pre-apply config for safekeeping. Dry-run behavior is via the CLI --dry-run flag; the API endpoint always applies when called with yes: true.",
    planned: "v1.5"
  }
];

const METHOD_CLASS: Record<string, string> = {
  GET: "method-get",
  POST: "method-post",
  PATCH: "method-patch",
  DELETE: "method-delete"
};

export function ApiDocsView(): ReactElement {
  return (
    <div className="api-docs-view">
      <div className="rule-list-section" style={{ flex: 1, minHeight: 0 }}>
        <div className="rule-list-header">
          <div className="rule-list-title-group">
            <div className="rule-list-title">API Reference</div>
            <div className="rule-list-subtitle">
              Management API at 127.0.0.1:47831. Errors return{" "}
              <code>{"{ errors: string[] }"}</code>.
            </div>
          </div>
        </div>
        <div className="rule-list-body">
          <ol className="api-endpoint-list">
            {ENDPOINTS.map((ep) => (
              <li key={`${ep.method}-${ep.path}`} className="api-endpoint">
                <div className="api-endpoint-head">
                  <span className={`api-method ${METHOD_CLASS[ep.method] ?? ""}`}>
                    {ep.method}
                  </span>
                  <code className="api-path">{ep.path}</code>
                  {ep.planned && (
                    <span className="api-planned-badge">Planned — {ep.planned}</span>
                  )}
                </div>
                <p className="api-purpose">{ep.purpose}</p>
                {ep.params && (
                  <p className="api-meta"><strong>Params:</strong> {ep.params}</p>
                )}
                {ep.response && (
                  <p className="api-meta"><strong>Response:</strong> {ep.response}</p>
                )}
                {ep.notes && (
                  <p className="api-meta"><strong>Notes:</strong> {ep.notes}</p>
                )}
              </li>
            ))}
          </ol>
        </div>
      </div>
    </div>
  );
}
