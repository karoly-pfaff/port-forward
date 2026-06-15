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
import { getPortAdvisories, summarizeGroupAction, validateGroupName } from "@portier/shared";
import type { GroupActionType } from "@portier/shared";
import type { ForwardManager } from "./forward-manager.js";
import { ConflictError, NotFoundError, ValidationError } from "./forward-manager.js";
import { diagnoseRule } from "./diagnose.js";
import type { ActivityStore } from "./activity/activity-store.js";
import { buildApplyImportFromPlan, buildConfigPlan } from "./config-plan.js";
import { buildLiveConnections } from "./connections-snapshot.js";
import { buildRuntimeInfo, type RuntimeInfoOptions } from "./runtime-info.js";

// Re-exported for backward compatibility; the canonical definitions live in
// ./runtime-info.ts (shared with the NestJS runtime service).
export { normalizeArch, normalizePlatform } from "./runtime-info.js";
export type { RuntimeInfoOptions } from "./runtime-info.js";

export interface AppOptions {
  staticClientDir?: string;
  activity?: ActivityStore;
  runtimeInfo?: RuntimeInfoOptions;
  /**
   * Optional clock for the volatile-timestamp endpoints — `GET /api/runtime`'s
   * `uptimeSeconds`, `GET /api/config/export`'s `exportedAt`, and
   * `GET /api/connections`'s `generatedAt`. Defaults to the real wall clock;
   * production never overrides it. A minimal, test-only seam (like
   * `runtimeInfo.startedAt`) so these endpoints can be parity-tested
   * deterministically against the NestJS implementation.
   */
  now?: () => Date;
}

export function createApp(manager: ForwardManager, options: AppOptions = {}): express.Express {
  const app = express();
  app.use(cors());
  app.use(express.json());

  const runtimeStartedAt = options.runtimeInfo?.startedAt ?? new Date();

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

  // ── Group operations (v1.8) ───────────────────────────────────────────────
  // Start/stop every rule sharing a `group` label. Behaviour over existing rule
  // metadata — never mutates rule definitions, order, enabled, or group. The
  // 4-segment path cannot collide with /api/forwards/:id/start (3 segments).

  const handleGroupAction = (action: GroupActionType) =>
    async (request: express.Request, response: express.Response, next: express.NextFunction): Promise<void> => {
      try {
        const group = decodeURIComponent(String(request.params.group)).trim();
        const errors = validateGroupName(group);
        if (errors.length > 0) {
          response.status(400).json({ errors });
          return;
        }
        const results = action === "start"
          ? await manager.startGroup(group)
          : await manager.stopGroup(group);
        if (results.length === 0) {
          response.status(404).json({ errors: [`No rules found in group "${group}".`] });
          return;
        }
        response.json(summarizeGroupAction(group, action, results));
      } catch (error) {
        next(error);
      }
    };

  app.post("/api/forwards/groups/:group/start", handleGroupAction("start"));
  app.post("/api/forwards/groups/:group/stop", handleGroupAction("stop"));

  app.post("/api/forwards/:id/diagnose", async (request, response, next) => {
    try {
      const rule = manager.getRule(request.params.id);
      if (!rule) {
        response.status(404).json({ errors: [`Forward rule ${request.params.id} was not found.`] });
        return;
      }
      const isRunning = manager.getStatus(rule.id).running;
      response.json(await diagnoseRule(rule, isRunning, (options.now ?? (() => new Date()))()));
    } catch (error) {
      next(error);
    }
  });

  // ── Status ──────────────────────────────────────────────────────────────────

  app.get("/api/status", (_request, response) => {
    response.json(manager.listStatus());
  });

  // ── Runtime info ──────────────────────────────────────────────────────────────

  app.get("/api/runtime", (_request, response) => {
    response.json(
      buildRuntimeInfo({
        runtimeInfo: options.runtimeInfo,
        startedAt: runtimeStartedAt,
        now: (options.now ?? (() => new Date()))(),
        pid: process.pid,
        platform: process.platform,
        arch: process.arch
      })
    );
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

  app.delete("/api/activity", (_request, response) => {
    const store = options.activity;
    if (store) {
      store.clear();
    }
    response.status(204).end();
  });

  // ── Config import/export ─────────────────────────────────────────────────────

  app.get("/api/config/export", (_request, response) => {
    response.json(manager.exportConfig((options.now ?? (() => new Date()))()));
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

  // ── Config plan ───────────────────────────────────────────────────────────────

  app.post("/api/config/plan", (request, response) => {
    const body = request.body as Record<string, unknown> | null;
    if (!body || typeof body !== "object" || !("desired" in body)) {
      response.status(400).json({ errors: ["desired is required."] });
      return;
    }
    const plan = buildConfigPlan({
      currentRules: manager.listRules(),
      desiredRaw: body.desired,
      now: (options.now ?? (() => new Date()))(),
    });
    response.json(plan);
  });

  // ── Config apply ──────────────────────────────────────────────────────────────

  app.post("/api/config/apply", async (request, response, next) => {
    try {
      const body = request.body as Record<string, unknown> | null;
      if (!body || typeof body !== "object" || !("desired" in body)) {
        response.status(400).json({ errors: ["desired is required."] });
        return;
      }

      const yes = body.yes === true;
      const dryRun = body.dryRun === true;
      const appliedAt = new Date().toISOString();
      const plan = buildConfigPlan({ currentRules: manager.listRules(), desiredRaw: body.desired });

      if (plan.summary.hasErrors) {
        response.json({
          ok: false, dryRun, appliedAt, plan,
          applied: { add: 0, update: 0, remove: 0, unchanged: plan.summary.unchanged },
        });
        return;
      }

      // Apply transformation (desired-state rule list + counts) lives in the
      // plan engine; the handler keeps only request/response and gating concerns.
      const { rules: rulesForImport, applied } = buildApplyImportFromPlan(plan);

      if (dryRun) {
        response.json({ ok: true, dryRun: true, appliedAt, plan, applied });
        return;
      }

      if (plan.summary.destructive > 0 && !yes) {
        response.status(400).json({ errors: ["Apply requires yes: true when destructive operations are present."] });
        return;
      }

      if (plan.summary.hasDrift) {
        const importCfg: ExportedConfig = {
          version: "1",
          exportedAt: appliedAt,
          rules: rulesForImport,
        };
        const result = await manager.importConfig(importCfg, "replace");
        // Invariant (Resilience-C): apply must never report ok:true when the
        // underlying import reports errors. Every currently-reachable import
        // error path is pre-blocked before this point — duplicate listen bindings
        // via the plan engine's detectDuplicateKeys (→ summary.hasErrors), invalid
        // desired rules via plan validation, and persist failures throw (→ 500) —
        // so this is a belt-and-suspenders guard against future drift. Surface the
        // import errors through the existing plan.errors field; no applied counts.
        if (result.errors.length > 0) {
          const planWithErrors = {
            ...plan,
            errors: [...plan.errors, ...result.errors.map((message) => ({ code: "IMPORT_ERROR", message }))],
            summary: { ...plan.summary, hasErrors: true },
          };
          response.json({
            ok: false,
            dryRun: false,
            appliedAt,
            plan: planWithErrors,
            applied: { add: 0, update: 0, remove: 0, unchanged: plan.summary.unchanged },
          });
          return;
        }
      }

      response.json({ ok: true, dryRun: false, appliedAt, plan, applied });
    } catch (error) {
      next(error);
    }
  });

  // ── Port advisory ─────────────────────────────────────────────────────────────

  app.get("/api/connections", (_request, response) => {
    response.json(
      buildLiveConnections({
        rules: manager.listRules(),
        tcpConnections: manager.getLiveTcpConnections(),
        udpSessions: manager.getLiveUdpSessions(),
        now: (options.now ?? (() => new Date()))()
      })
    );
  });

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
