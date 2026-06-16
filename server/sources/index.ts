import type http from "node:http";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import type { Socket } from "node:net";
import { Logger } from "@nestjs/common";
import { ConfigStore } from "./persistence/config-store.js";
import { ForwardManager } from "./forwarders/forward-manager.js";
import { ActivityStore } from "./activity/activity-store.js";
import { resolveServerOptions } from "./app/server-options.js";
import type { RuntimeInfoOptions } from "./runtime/runtime-info.js";
import { createNestApp } from "./app/app.factory.js";
import type { AppRuntime } from "./app/runtime-context.js";
import { createStaticFallback, hasStaticClient } from "./static/static-serving.js";

/**
 * Portier TypeScript server entry — the NestJS runtime.
 *
 * It owns the server lifecycle (resolve options, load the config + start enabled
 * forwarders, bind the HTTP server, graceful shutdown) and builds a NestJS app whose
 * providers are wired to the live `ForwardManager`/`ActivityStore`/runtime info/static
 * client via `createNestApp(runtime)`. Runtime logging goes through the NestJS `Logger`.
 */

const logger = new Logger("Server");

/** The stack (or string form) of a thrown value, for `Logger.error`'s stack argument. */
function stackOf(error: unknown): string | undefined {
  return error instanceof Error ? error.stack : String(error);
}

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

process.on("uncaughtException", (error) => {
  logger.error("Uncaught exception.", stackOf(error));
  process.exit(1);
});

process.on("unhandledRejection", (error) => {
  logger.error("Unhandled rejection.", stackOf(error));
  process.exit(1);
});

void main().catch((error: unknown) => {
  logger.error("Portier server failed to start.", stackOf(error));
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
    logger.error("HTTP server error.", stackOf(error));
  });
  if (options.host === "0.0.0.0") {
    logger.warn(
      `Management API is bound to 0.0.0.0 (${options.host}:${options.port}) — the web UI and REST API are reachable from the local network.`
    );
  }

  logger.log(
    `Portier server started on ${options.host}:${options.port} ` +
      `(service=${options.service}, config=${options.configPath}, ` +
      `static=${options.staticClientDir} [${options.staticClientDirSource}], ` +
      `startedForwarders=${startedForwardRuleCount}).`
  );
  if (!hasStaticClient(options.staticClientDir)) {
    logger.warn(
      `Web UI static files were not found at ${options.staticClientDir} ` +
        `[${options.staticClientDirSource}]; API remains available.`
    );
  }

  let shutdownStarted = false;
  async function shutdown(signal: NodeJS.Signals): Promise<void> {
    if (shutdownStarted) {
      logger.warn(`Shutdown already in progress (signal ${signal}).`);
      return;
    }
    shutdownStarted = true;

    logger.log(`Shutdown signal received (${signal}).`);
    try {
      logger.log("Stopping TCP and UDP forwarders.");
      await manager.stopAll();
      logger.log("Forwarders stopped.");

      logger.log("Flushing config store.");
      await manager.flush();
      logger.log("Config store flushed.");

      logger.log(`Closing HTTP server (${httpSockets.size} active sockets).`);
      for (const socket of httpSockets) {
        socket.destroy();
      }
      await app.close();
      logger.log("HTTP server closed.");
      process.exit(0);
    } catch (error) {
      logger.error("Shutdown failed.", stackOf(error));
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
