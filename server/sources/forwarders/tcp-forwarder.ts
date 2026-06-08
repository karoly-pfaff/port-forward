import net from "node:net";
import type { ForwardRule, ForwardStatus, ActivityEventInput } from "@portier/shared";
import type { Forwarder } from "./types.js";
import type { TcpConnectionRegistry } from "../connections/tcp-connection-registry.js";

export class TcpForwarder implements Forwarder {
  private server?: net.Server;
  private sockets = new Set<net.Socket>();
  private status: ForwardStatus;

  constructor(
    private readonly rule: ForwardRule,
    private readonly onEvent?: (event: ActivityEventInput) => void,
    private readonly registry?: TcpConnectionRegistry
  ) {
    this.status = {
      ruleId: rule.id,
      running: false,
      activeConnections: 0,
      bytesIn: 0,
      bytesOut: 0
    };
  }

  async start(): Promise<void> {
    if (this.server) {
      return;
    }

    const server = net.createServer((clientSocket) => this.handleClient(clientSocket));
    this.server = server;

    server.on("error", (error) => {
      this.status.lastError = error.message;
    });

    await new Promise<void>((resolve, reject) => {
      const onError = (error: Error) => {
        server.off("listening", onListening);
        reject(error);
      };
      const onListening = () => {
        server.off("error", onError);
        this.status.running = true;
        this.status.startedAt = new Date().toISOString();
        resolve();
      };

      server.once("error", onError);
      server.once("listening", onListening);
      server.listen(this.rule.listenPort, this.rule.listenHost);
    });
  }

  async stop(): Promise<void> {
    const server = this.server;
    this.server = undefined;

    for (const socket of this.sockets) {
      socket.destroy();
    }
    this.sockets.clear();
    this.registry?.closeConnectionsForRule(this.rule.id);

    if (server) {
      await new Promise<void>((resolve, reject) => {
        server.close((error) => {
          if (error) {
            reject(error);
            return;
          }
          resolve();
        });
      });
    }

    this.status.running = false;
    this.status.activeConnections = 0;
    this.status.startedAt = undefined;
  }

  getStatus(): ForwardStatus {
    return { ...this.status };
  }

  private handleClient(clientSocket: net.Socket): void {
    const targetSocket = net.createConnection({
      host: this.rule.targetHost,
      port: this.rule.targetPort
    });

    this.trackSocket(clientSocket);
    this.trackSocket(targetSocket);
    this.status.activeConnections = (this.status.activeConnections ?? 0) + 1;

    const remoteAddress = clientSocket.remoteAddress ?? "unknown";
    const remotePort = clientSocket.remotePort ?? 0;

    const connId = this.registry?.openConnection({
      ruleId: this.rule.id,
      ruleName: this.rule.name,
      clientAddress: remoteAddress,
      clientPort: remotePort,
      targetAddress: this.rule.targetHost,
      targetPort: this.rule.targetPort
    });

    this.onEvent?.({
      type: "tcp.connection.opened",
      severity: "info",
      ruleId: this.rule.id,
      ruleName: this.rule.name,
      protocol: "tcp",
      message: `TCP connection opened from ${remoteAddress}:${remotePort}.`,
      details: {
        remoteAddress,
        remotePort,
        targetHost: this.rule.targetHost,
        targetPort: this.rule.targetPort
      }
    });

    clientSocket.on("data", (chunk) => {
      this.status.bytesIn += chunk.length;
      if (connId) this.registry?.addBytesIn(connId, chunk.length);
    });
    targetSocket.on("data", (chunk) => {
      this.status.bytesOut += chunk.length;
      if (connId) this.registry?.addBytesOut(connId, chunk.length);
    });

    clientSocket.pipe(targetSocket);
    targetSocket.pipe(clientSocket);

    let loggedError = false;
    const closeBoth = (error?: Error) => {
      if (error && !loggedError) {
        loggedError = true;
        this.status.lastError = error.message;
        if (connId) this.registry?.closeConnection(connId);
        this.onEvent?.({
          type: "tcp.connection.error",
          severity: "error",
          ruleId: this.rule.id,
          ruleName: this.rule.name,
          protocol: "tcp",
          message: `TCP connection error: ${error.message}`,
          details: { remoteAddress, remotePort }
        });
      }
      clientSocket.destroy();
      targetSocket.destroy();
    };

    clientSocket.on("error", closeBoth);
    targetSocket.on("error", closeBoth);

    let countedClosed = false;
    const onClosed = () => {
      this.sockets.delete(clientSocket);
      this.sockets.delete(targetSocket);
      if (!countedClosed) {
        countedClosed = true;
        this.status.activeConnections = Math.max(0, (this.status.activeConnections ?? 0) - 1);
        if (connId) this.registry?.closeConnection(connId);
        if (!loggedError) {
          this.onEvent?.({
            type: "tcp.connection.closed",
            severity: "info",
            ruleId: this.rule.id,
            ruleName: this.rule.name,
            protocol: "tcp",
            message: `TCP connection closed from ${remoteAddress}:${remotePort}.`,
            details: { remoteAddress, remotePort }
          });
        }
      }
    };

    clientSocket.on("close", onClosed);
    targetSocket.on("close", onClosed);
  }

  private trackSocket(socket: net.Socket): void {
    this.sockets.add(socket);
  }
}
