import http from "node:http";
import type { Socket } from "node:net";
import { createApp, hasStaticClient } from "./api.js";
import { ConfigStore } from "./config-store.js";
import { ForwardManager } from "./forward-manager.js";
import { ActivityStore } from "./activity/activity-store.js";
import { createConsoleLogger, errorFields } from "./logger.js";
import { resolveServerOptions } from "./server-options.js";

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
  const options = resolveServerOptions();
  const store = new ConfigStore(options.configPath);
  const activity = new ActivityStore();
  const manager = new ForwardManager(store, activity);
  const startedForwardRuleCount = await manager.loadAndStartEnabled();

  const app = createApp(manager, { staticClientDir: options.staticClientDir, activity });
  const server = http.createServer(app);
  const httpSockets = new Set<Socket>();

  server.on("connection", (socket) => {
    httpSockets.add(socket);
    socket.on("close", () => httpSockets.delete(socket));
  });

  await listenHttpServer(server, options.port, options.host);
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
    service: options.service,
    configPath: options.configPath,
    bindAddress: `${options.host}:${options.port}`,
    staticClientDir: options.staticClientDir,
    staticClientDirSource: options.staticClientDirSource,
    startedForwardRuleCount
  });
  if (!hasStaticClient(options.staticClientDir)) {
    logger.warn("static_client.unavailable", "Web UI static files were not found; API remains available.", {
      staticClientDir: options.staticClientDir,
      staticClientDirSource: options.staticClientDirSource
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
      await closeHttpServer(server);
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

function listenHttpServer(httpServer: http.Server, port: number, host: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const onError = (error: Error) => {
      httpServer.off("listening", onListening);
      reject(error);
    };
    const onListening = () => {
      httpServer.off("error", onError);
      resolve();
    };

    httpServer.once("error", onError);
    httpServer.once("listening", onListening);
    httpServer.listen(port, host);
  });
}

function closeHttpServer(httpServer: http.Server): Promise<void> {
  return new Promise((resolve, reject) => {
    httpServer.close((error) => {
      if (error) {
        reject(error);
        return;
      }
      resolve();
    });
  });
}
