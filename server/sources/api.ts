import { existsSync } from "node:fs";
import { join } from "node:path";
import express from "express";
import cors from "cors";
import type {
  ExportedConfig,
  ForwardRule,
  ForwardRuleResponse,
  LiveConnectionsResponse,
  RuleLiveSummary,
  ActivityEventType,
  ActivitySeverity
} from "@portier/shared";
import { getPortAdvisories, PORTIER_DEFAULT_HOST, PORTIER_DEFAULT_PORT } from "@portier/shared";
import type { ForwardManager } from "./forward-manager.js";
import { ConflictError, NotFoundError, ValidationError } from "./forward-manager.js";
import { diagnoseRule } from "./diagnose.js";
import type { ActivityStore } from "./activity/activity-store.js";
import { buildApplyImportFromPlan, buildConfigPlan } from "./config-plan.js";

export interface RuntimeInfoOptions {
  version: string;
  managementHost: string;
  managementPort: number;
  configPath: string;
  staticDir: string;
  serviceMode: boolean;
  startedAt: Date;
}

export interface AppOptions {
  staticClientDir?: string;
  activity?: ActivityStore;
  runtimeInfo?: RuntimeInfoOptions;
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

  app.post("/api/forwards/:id/diagnose", async (request, response, next) => {
    try {
      const rule = manager.getRule(request.params.id);
      if (!rule) {
        response.status(404).json({ errors: [`Forward rule ${request.params.id} was not found.`] });
        return;
      }
      const isRunning = manager.getStatus(rule.id).running;
      response.json(await diagnoseRule(rule, isRunning));
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
    const info = options.runtimeInfo;
    response.json({
      name: "Portier",
      version: info?.version ?? "unknown",
      runtime: "node",
      platform: normalizePlatform(process.platform),
      arch: normalizeArch(process.arch),
      uptimeSeconds: Math.floor((Date.now() - runtimeStartedAt.getTime()) / 1000),
      startedAt: runtimeStartedAt.toISOString(),
      managementHost: info?.managementHost ?? PORTIER_DEFAULT_HOST,
      managementPort: info?.managementPort ?? PORTIER_DEFAULT_PORT,
      configPath: info?.configPath ?? "",
      staticDir: info?.staticDir ?? "",
      serviceMode: info?.serviceMode ?? false,
      pid: process.pid
    });
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

  // ── Config plan ───────────────────────────────────────────────────────────────

  app.post("/api/config/plan", (request, response) => {
    const body = request.body as Record<string, unknown> | null;
    if (!body || typeof body !== "object" || !("desired" in body)) {
      response.status(400).json({ errors: ["desired is required."] });
      return;
    }
    const plan = buildConfigPlan({ currentRules: manager.listRules(), desiredRaw: body.desired });
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
        await manager.importConfig(importCfg, "replace");
      }

      response.json({ ok: true, dryRun: false, appliedAt, plan, applied });
    } catch (error) {
      next(error);
    }
  });

  // ── Port advisory ─────────────────────────────────────────────────────────────

  app.get("/api/connections", (_request, response) => {
    const tcpConnections = manager.getLiveTcpConnections();
    const udpSessions = manager.getLiveUdpSessions();

    const ruleSummaries: RuleLiveSummary[] = manager.listRules().map((rule) => {
      const tcpForRule = tcpConnections.filter((c) => c.ruleId === rule.id);
      const udpForRule = udpSessions.filter((s) => s.ruleId === rule.id);

      const activeTcpConnections = tcpForRule.length;
      const activeUdpSessions = udpForRule.length;
      const bytesIn = [...tcpForRule, ...udpForRule].reduce((sum, item) => sum + item.bytesIn, 0);
      const bytesOut = [...tcpForRule, ...udpForRule].reduce((sum, item) => sum + item.bytesOut, 0);
      const packetsIn = udpForRule.reduce((sum, s) => sum + s.packetsIn, 0);
      const packetsOut = udpForRule.reduce((sum, s) => sum + s.packetsOut, 0);

      let lastTrafficAt: string | null = null;
      if (tcpForRule.length > 0) {
        lastTrafficAt = [...tcpForRule].sort((a, b) => a.startedAt < b.startedAt ? 1 : -1)[0].startedAt;
      }
      if (udpForRule.length > 0) {
        const udpLast = [...udpForRule].sort((a, b) => a.lastSeenAt < b.lastSeenAt ? 1 : -1)[0].lastSeenAt;
        if (lastTrafficAt === null || udpLast > lastTrafficAt) {
          lastTrafficAt = udpLast;
        }
      }

      return {
        ruleId: rule.id,
        ruleName: rule.name,
        protocol: rule.protocol,
        activeTcpConnections,
        activeUdpSessions,
        bytesIn,
        bytesOut,
        packetsIn,
        packetsOut,
        lastTrafficAt
      };
    });

    const result: LiveConnectionsResponse = {
      generatedAt: new Date().toISOString(),
      tcpConnections,
      udpSessions,
      ruleSummaries
    };

    response.json(result);
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

export function normalizePlatform(platform: string): "windows" | "macos" | "linux" | "unknown" {
  if (platform === "win32") return "windows";
  if (platform === "darwin") return "macos";
  if (platform === "linux") return "linux";
  return "unknown";
}

export function normalizeArch(arch: string): "x64" | "arm64" | "unknown" {
  if (arch === "x64") return "x64";
  if (arch === "arm64") return "arm64";
  return "unknown";
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
