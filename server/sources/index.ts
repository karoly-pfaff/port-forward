import type http from "node:http";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import type { Socket } from "node:net";
import { ConfigStore } from "./config-store.js";
import { ForwardManager } from "./forward-manager.js";
import { ActivityStore } from "./activity/activity-store.js";
import { createConsoleLogger, errorFields } from "./logger.js";
import { resolveServerOptions } from "./server-options.js";
import type { RuntimeInfoOptions } from "./runtime-info.js";
import { createNestApp } from "./app/app.factory.js";
import type { AppRuntime } from "./common/runtime-context.js";
import { createStaticFallback, hasStaticClient } from "./static/static-serving.js";

/**
 * Default Portier TypeScript server entry — the NestJS runtime.
 *
 * It owns the same lifecycle the server has always had (resolve options, load the
 * config + start enabled forwarders, bind the HTTP server, graceful shutdown), and
 * builds a NestJS app whose providers are wired to the live `ForwardManager`/
 * `ActivityStore`/runtime info/static client via `createNestApp(runtime)`.
 */

function readServerVersion(): string {
  try {
    // From build/index.js up to server/package.json.
    const pkgPath = join(dirname(fileURLToPath(import.meta.url)), "..", "package.json");
    const pkg = JSON.parse(readFileSync(pkgPath, "utf-8")) as { version?: string };
    return typeof pkg.version === "string" ? pkg.version : "unknown";
  } catch {
    return "unknown";
  }
}

const logger = createConsoleLogger();

process.on("uncaughtException", (error) => {
  logger.error("process.uncaught_exception", "Uncaught exception.", errorFields(error));
  process.exit(1);
});

process.on("unhandledRejection", (error) => {
  logger.error("process.unhandled_rejection", "Unhandled rejection.", errorFields(error));
  process.exit(1);
});

void main().catch((error: unknown) => {
  logger.error("server.start_failed", "Portier server failed to start.", errorFields(error));
  process.exit(1);
});

async function main(): Promise<void> {
  const startedAt = new Date();
  const options = resolveServerOptions();
  const store = new ConfigStore(options.configPath);
  const activity = new ActivityStore();
  const manager = new ForwardManager(store, activity);
  const startedForwardRuleCount = await manager.loadAndStartEnabled();

  const runtimeInfo: RuntimeInfoOptions = {
    version: readServerVersion(),
    managementHost: options.host,
    managementPort: options.port,
    configPath: options.configPath,
    staticDir: options.staticClientDir,
    serviceMode: options.service,
    startedAt,
  };

  // The single live manager satisfies every reader/writer interface; the activity
  // store, runtime-info reader, and static fallback back the remaining providers.
  const runtime: AppRuntime = {
    manager,
    activity,
    runtimeInfoReader: { options: () => runtimeInfo, startedAt: () => startedAt },
    staticFallback: createStaticFallback(options.staticClientDir),
    staticClientDir: options.staticClientDir,
  };

  const app = await createNestApp(runtime);
  const server = app.getHttpServer() as http.Server;
  const httpSockets = new Set<Socket>();

  server.on("connection", (socket: Socket) => {
    httpSockets.add(socket);
    socket.on("close", () => httpSockets.delete(socket));
  });

  await app.listen(options.port, options.host);
  server.on("error", (error) => {
    logger.error("server.error", "HTTP server error.", errorFields(error));
  });
  if (options.host === "0.0.0.0") {
    logger.warn(
      "server.management_lan_exposure",
      "Management API is bound to 0.0.0.0 — the web UI and REST API are reachable from the local network.",
      { bindAddress: `${options.host}:${options.port}` }
    );
  }

  logger.info("server.started", "Portier server started.", {
    runtime: "nest",
    service: options.service,
    configPath: options.configPath,
    bindAddress: `${options.host}:${options.port}`,
    staticClientDir: options.staticClientDir,
    staticClientDirSource: options.staticClientDirSource,
    startedForwardRuleCount,
  });
  if (!hasStaticClient(options.staticClientDir)) {
    logger.warn("static_client.unavailable", "Web UI static files were not found; API remains available.", {
      staticClientDir: options.staticClientDir,
      staticClientDirSource: options.staticClientDirSource,
    });
  }

  let shutdownStarted = false;
  async function shutdown(signal: NodeJS.Signals): Promise<void> {
    if (shutdownStarted) {
      logger.warn("shutdown.duplicate_signal", "Shutdown already in progress.", { signal });
      return;
    }
    shutdownStarted = true;

    logger.info("shutdown.received", "Shutdown signal received.", { signal });
    try {
      logger.info("shutdown.forwarders_stopping", "Stopping TCP and UDP forwarders.");
      await manager.stopAll();
      logger.info("shutdown.forwarders_stopped", "Forwarders stopped.");

      logger.info("shutdown.config_flushing", "Flushing config store.");
      await manager.flush();
      logger.info("shutdown.config_flushed", "Config store flushed.");

      logger.info("shutdown.http_closing", "Closing HTTP server.", { activeHttpSockets: httpSockets.size });
      for (const socket of httpSockets) {
        socket.destroy();
      }
      await app.close();
      logger.info("shutdown.http_closed", "HTTP server closed.");
      process.exit(0);
    } catch (error) {
      logger.error("shutdown.failed", "Shutdown failed.", errorFields(error));
      process.exit(1);
    }
  }

  process.on("SIGINT", (signal) => {
    void shutdown(signal);
  });
  process.on("SIGTERM", (signal) => {
    void shutdown(signal);
  });
}
