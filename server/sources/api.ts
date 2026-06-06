import { existsSync } from "node:fs";
import { join } from "node:path";
import express from "express";
import cors from "cors";
import type {
  ExportedConfig,
  ForwardRule,
  ForwardRuleResponse,
  ActivityEventType,
  ActivitySeverity
} from "@portier/shared";
import { getPortAdvisories } from "@portier/shared";
import type { ForwardManager } from "./forward-manager.js";
import { ConflictError, NotFoundError, ValidationError } from "./forward-manager.js";
import type { ActivityStore } from "./activity/activity-store.js";

export interface AppOptions {
  staticClientDir?: string;
  activity?: ActivityStore;
}

export function createApp(manager: ForwardManager, options: AppOptions = {}): express.Express {
  const app = express();
  app.use(cors());
  app.use(express.json());

  // ── Forward rules CRUD ──────────────────────────────────────────────────────

  app.get("/api/forwards", (_request, response) => {
    response.json(manager.listRules().map(toRuleResponse));
  });

  app.post("/api/forwards", async (request, response, next) => {
    try {
      response.status(201).json(toRuleResponse(await manager.addRule(request.body)));
    } catch (error) {
      next(error);
    }
  });

  app.patch("/api/forwards/:id", async (request, response, next) => {
    try {
      response.json(toRuleResponse(await manager.updateRule(request.params.id, request.body)));
    } catch (error) {
      next(error);
    }
  });

  app.delete("/api/forwards/:id", async (request, response, next) => {
    try {
      await manager.deleteRule(request.params.id);
      response.status(204).send();
    } catch (error) {
      next(error);
    }
  });

  app.post("/api/forwards/reorder", async (request, response, next) => {
    try {
      const body = request.body as { ids?: unknown };
      if (!Array.isArray(body.ids) || !body.ids.every((id) => typeof id === "string")) {
        response.status(400).json({ errors: ["ids must be an array of strings."] });
        return;
      }
      await manager.reorderRules(body.ids as string[]);
      response.json(manager.listRules().map(toRuleResponse));
    } catch (error) {
      next(error);
    }
  });

  app.post("/api/forwards/:id/start", async (request, response, next) => {
    try {
      response.json(await manager.startRule(request.params.id));
    } catch (error) {
      next(error);
    }
  });

  app.post("/api/forwards/:id/stop", async (request, response, next) => {
    try {
      response.json(await manager.stopRule(request.params.id));
    } catch (error) {
      next(error);
    }
  });

  // ── Status ──────────────────────────────────────────────────────────────────

  app.get("/api/status", (_request, response) => {
    response.json(manager.listStatus());
  });

  // ── Activity ─────────────────────────────────────────────────────────────────

  app.get("/api/activity", (request, response) => {
    const store = options.activity;
    if (!store) {
      response.json({ events: [] });
      return;
    }

    const rawLimit = Number(request.query.limit);
    const limit = Number.isInteger(rawLimit) && rawLimit > 0 ? Math.min(rawLimit, 500) : 100;
    const ruleId = typeof request.query.ruleId === "string" ? request.query.ruleId : undefined;
    const type = typeof request.query.type === "string" ? request.query.type as ActivityEventType : undefined;
    const severity = typeof request.query.severity === "string" ? request.query.severity as ActivitySeverity : undefined;

    const events = store.list({ limit, ruleId, type, severity });
    response.json({ events });
  });

  // ── Config import/export ─────────────────────────────────────────────────────

  app.get("/api/config/export", (_request, response) => {
    response.json(manager.exportConfig());
  });

  app.post("/api/config/import", async (request, response, next) => {
    try {
      const body = request.body as { mode?: unknown; config?: unknown };
      const mode = body.mode;
      if (mode !== "replace" && mode !== "merge") {
        response.status(400).json({ errors: ["mode must be replace or merge."] });
        return;
      }

      const config = body.config as Partial<ExportedConfig> | undefined;
      if (!config || config.version !== "1" || !Array.isArray(config.rules)) {
        response.status(400).json({ errors: ["config must be a valid Portier config object with version 1 and a rules array."] });
        return;
      }

      const result = await manager.importConfig(config as ExportedConfig, mode);
      if (result.errors.length > 0) {
        response.status(422).json({ errors: result.errors, result });
        return;
      }

      response.json({ result, rules: manager.listRules().map(toRuleResponse) });
    } catch (error) {
      next(error);
    }
  });

  // ── Port advisory ─────────────────────────────────────────────────────────────

  app.get("/api/ports/advisory", (request, response) => {
    const port = Number(request.query.port);
    const purpose = request.query.purpose;
    const listenHost = typeof request.query.listenHost === "string" ? request.query.listenHost : undefined;

    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      response.status(400).json({ errors: ["port must be an integer from 1 to 65535."] });
      return;
    }

    if (purpose !== "management" && purpose !== "forward") {
      response.status(400).json({ errors: ["purpose must be management or forward."] });
      return;
    }

    response.json(getPortAdvisories({ port, listenHost, purpose }));
  });

  app.use("/api", (_request, response) => {
    response.status(404).json({ errors: ["API route was not found."] });
  });

  if (options.staticClientDir && hasStaticClient(options.staticClientDir)) {
    app.use(express.static(options.staticClientDir));
    app.get(/^(?!\/api(?:\/|$)).*/, (_request, response) => {
      response.sendFile(join(options.staticClientDir as string, "index.html"));
    });
  }

  app.use((error: unknown, _request: express.Request, response: express.Response, _next: express.NextFunction) => {
    if (error instanceof ValidationError) {
      response.status(400).json({ errors: error.errors });
      return;
    }
    if (error instanceof ConflictError) {
      response.status(409).json({ errors: [error.message] });
      return;
    }
    if (error instanceof NotFoundError) {
      response.status(404).json({ errors: [error.message] });
      return;
    }

    const message = error instanceof Error ? error.message : "Unexpected server error.";
    response.status(500).json({ errors: [message] });
  });

  return app;
}

export function hasStaticClient(staticClientDir: string): boolean {
  return existsSync(join(staticClientDir, "index.html"));
}

function toRuleResponse(rule: ForwardRule): ForwardRuleResponse {
  return {
    ...rule,
    advisories: getPortAdvisories({
      port: rule.listenPort,
      listenHost: rule.listenHost,
      purpose: "forward"
    })
  };
}
