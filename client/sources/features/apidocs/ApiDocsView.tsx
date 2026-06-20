import { type ReactElement } from "react";
import { apiDocsModel, type ApiDocsModel, type ApiParameter } from "./openApiDocs.js";

const METHOD_CLASS: Record<string, string> = {
  GET: "method-get",
  POST: "method-post",
  PUT: "method-put",
  PATCH: "method-patch",
  DELETE: "method-delete",
  OPTIONS: "method-options",
  HEAD: "method-head",
};

function parameterLabel(param: ApiParameter): string {
  const facets = [param.location];
  if (param.type) facets.push(param.type);
  if (param.required) facets.push("required");
  return facets.join(", ");
}

export function ApiDocsView({ model = apiDocsModel }: { model?: ApiDocsModel } = {}): ReactElement {
  return (
    <div className="api-docs-view">
      <div className="rule-list-section" style={{ flex: 1, minHeight: 0 }}>
        <div className="rule-list-header">
          <div className="rule-list-title-group">
            <div className="rule-list-title">API Reference</div>
            <div className="rule-list-subtitle">
              {model.title} · v{model.version} — generated from the OpenAPI contract.
            </div>
          </div>
        </div>
        <div className="rule-list-body">
          {model.groups.map((group) => (
            <section key={group.name} className="api-group" aria-label={`${group.name} endpoints`}>
              <h3 className="api-group-title">{group.name}</h3>
              <ol className="api-endpoint-list">
                {group.operations.map((op) => (
                  <li key={`${op.method}-${op.path}`} className="api-endpoint">
                    <div className="api-endpoint-head">
                      <span className={`api-method ${METHOD_CLASS[op.method]}`}>{op.method}</span>
                      <code className="api-path">{op.path}</code>
                    </div>
                    {op.summary && <p className="api-purpose">{op.summary}</p>}
                    <p className="api-meta">{op.description}</p>

                    {op.parameters.length > 0 && (
                      <div className="api-meta">
                        <strong>Parameters</strong>
                        <ul className="api-param-list">
                          {op.parameters.map((param) => (
                            <li key={`${param.location}-${param.name}`}>
                              <code>{param.name}</code> ({parameterLabel(param)}) — {param.description}
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}

                    {op.requestBody && (
                      <p className="api-meta">
                        <strong>Request body{op.requestBody.required ? " (required)" : ""}:</strong>{" "}
                        <code>{op.requestBody.schema}</code>
                      </p>
                    )}

                    <div className="api-meta">
                      <strong>Responses</strong>
                      <ul className="api-response-list">
                        {op.responses.map((response) => (
                          <li key={response.status}>
                            <span className="api-status">{response.status}</span> — {response.description}
                            {response.schema && (
                              <>
                                {" "}
                                <code>{response.schema}</code>
                              </>
                            )}
                          </li>
                        ))}
                      </ul>
                    </div>
                  </li>
                ))}
              </ol>
            </section>
          ))}
        </div>
      </div>
    </div>
  );
}
