import { type ReactElement } from "react";

interface EndpointDoc {
  method: string;
  path: string;
  purpose: string;
  params?: string;
  response?: string;
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
    path: "/api/activity",
    purpose: "List recent activity events (in-memory, resets on restart, max 500).",
    params: "Query: limit (default 100, max 500), ruleId, type, severity",
    response: "{ events: ActivityEvent[] } — newest first"
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
                </div>
                <p className="api-purpose">{ep.purpose}</p>
                {ep.params && (
                  <p className="api-meta"><strong>Params:</strong> {ep.params}</p>
                )}
                {ep.response && (
                  <p className="api-meta"><strong>Response:</strong> {ep.response}</p>
                )}
              </li>
            ))}
          </ol>
        </div>
      </div>
    </div>
  );
}
