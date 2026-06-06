import dgram from "node:dgram";
import type { RemoteInfo } from "node:dgram";
import type { ForwardRule, ForwardStatus, ActivityEventInput } from "@portier/shared";
import type { Forwarder } from "./types.js";

const UDP_LOG_INTERVAL_MS = 1000;
const DEFAULT_SESSION_TIMEOUT_MS = 60_000;

interface UdpSession {
  targetSocket: dgram.Socket;
  timer: ReturnType<typeof setTimeout>;
}

export class UdpForwarder implements Forwarder {
  private listenSocket?: dgram.Socket;
  private targetSocket?: dgram.Socket; // shared socket for one-way / last-client modes
  private lastClient?: RemoteInfo;
  private sessions = new Map<string, UdpSession>();
  private status: ForwardStatus;
  private lastForwardLogAt = 0;
  private lastReturnLogAt = 0;

  constructor(
    private readonly rule: ForwardRule,
    private readonly onEvent?: (event: ActivityEventInput) => void,
    private readonly sessionTimeoutMs: number = DEFAULT_SESSION_TIMEOUT_MS
  ) {
    this.status = {
      ruleId: rule.id,
      running: false,
      bytesIn: 0,
      bytesOut: 0,
      packetsIn: 0,
      packetsOut: 0,
      activeUdpSessions: rule.udpMode === "bidirectional-multi-client" ? 0 : undefined
    };
  }

  async start(): Promise<void> {
    if (this.listenSocket) {
      return;
    }

    const listenSocket = dgram.createSocket("udp4");
    this.listenSocket = listenSocket;

    const mode = this.rule.udpMode ?? "one-way";

    if (mode === "bidirectional-multi-client") {
      listenSocket.on("message", (message, remote) => {
        this.handleMultiClientMessage(message, remote, listenSocket);
      });
    } else {
      const targetSocket = dgram.createSocket("udp4");
      this.targetSocket = targetSocket;

      listenSocket.on("message", (message, remote) => {
        this.lastClient = remote;
        this.status.bytesIn += message.length;
        this.status.packetsIn = (this.status.packetsIn ?? 0) + 1;

        targetSocket.send(message, this.rule.targetPort, this.rule.targetHost, (error) => {
          if (error) {
            this.status.lastError = error.message;
            this.onEvent?.({
              type: "udp.packet.error",
              severity: "error",
              ruleId: this.rule.id,
              ruleName: this.rule.name,
              protocol: "udp",
              message: `UDP send error: ${error.message}`
            });
            return;
          }

          const now = Date.now();
          if (now - this.lastForwardLogAt >= UDP_LOG_INTERVAL_MS) {
            this.lastForwardLogAt = now;
            this.onEvent?.({
              type: "udp.packet.forwarded",
              severity: "info",
              ruleId: this.rule.id,
              ruleName: this.rule.name,
              protocol: "udp",
              message: `UDP packet forwarded from ${remote.address}:${remote.port} to ${this.rule.targetHost}:${this.rule.targetPort}.`,
              details: {
                fromAddress: remote.address,
                fromPort: remote.port,
                targetHost: this.rule.targetHost,
                targetPort: this.rule.targetPort,
                bytes: message.length
              }
            });
          }
        });
      });

      // This mode only remembers the most recent UDP client.
      if (mode === "bidirectional-last-client") {
        targetSocket.on("message", (message) => {
          if (!this.lastClient) {
            return;
          }
          this.status.bytesOut += message.length;
          this.status.packetsOut = (this.status.packetsOut ?? 0) + 1;

          const client = this.lastClient;
          listenSocket.send(message, client.port, client.address, (error) => {
            if (error) {
              this.status.lastError = error.message;
              this.onEvent?.({
                type: "udp.packet.error",
                severity: "error",
                ruleId: this.rule.id,
                ruleName: this.rule.name,
                protocol: "udp",
                message: `UDP return error: ${error.message}`
              });
              return;
            }

            const now = Date.now();
            if (now - this.lastReturnLogAt >= UDP_LOG_INTERVAL_MS) {
              this.lastReturnLogAt = now;
              this.onEvent?.({
                type: "udp.packet.returned",
                severity: "info",
                ruleId: this.rule.id,
                ruleName: this.rule.name,
                protocol: "udp",
                message: `UDP reply returned to ${client.address}:${client.port}.`,
                details: {
                  toAddress: client.address,
                  toPort: client.port,
                  bytes: message.length
                }
              });
            }
          });
        });
      }

      targetSocket.on("error", (error) => {
        this.status.lastError = error.message;
      });
    }

    listenSocket.on("error", (error) => {
      this.status.lastError = error.message;
    });

    await new Promise<void>((resolve, reject) => {
      const onError = (error: Error) => {
        listenSocket.off("listening", onListening);
        reject(error);
      };
      const onListening = () => {
        listenSocket.off("error", onError);
        this.status.running = true;
        this.status.startedAt = new Date().toISOString();
        resolve();
      };

      listenSocket.once("error", onError);
      listenSocket.once("listening", onListening);
      listenSocket.bind(this.rule.listenPort, this.rule.listenHost);
    });
  }

  async stop(): Promise<void> {
    // Close all multi-client sessions
    for (const [key, session] of this.sessions) {
      clearTimeout(session.timer);
      await closeSocket(session.targetSocket);
      this.sessions.delete(key);
    }

    const listenSocket = this.listenSocket;
    const targetSocket = this.targetSocket;
    this.listenSocket = undefined;
    this.targetSocket = undefined;
    this.lastClient = undefined;

    await Promise.all([closeSocket(listenSocket), closeSocket(targetSocket)]);

    this.status.running = false;
    this.status.startedAt = undefined;
    if (this.status.activeUdpSessions !== undefined) {
      this.status.activeUdpSessions = 0;
    }
  }

  getStatus(): ForwardStatus {
    return { ...this.status };
  }

  private handleMultiClientMessage(
    message: Buffer,
    remote: RemoteInfo,
    listenSocket: dgram.Socket
  ): void {
    const sessionKey = `${remote.address}:${remote.port}`;
    this.status.bytesIn += message.length;
    this.status.packetsIn = (this.status.packetsIn ?? 0) + 1;

    let session = this.sessions.get(sessionKey);

    if (!session) {
      const targetSocket = dgram.createSocket("udp4");

      targetSocket.on("message", (response) => {
        if (!this.listenSocket) return;
        this.status.bytesOut += response.length;
        this.status.packetsOut = (this.status.packetsOut ?? 0) + 1;

        listenSocket.send(response, remote.port, remote.address, (error) => {
          if (error) {
            this.status.lastError = error.message;
            this.onEvent?.({
              type: "udp.packet.error",
              severity: "error",
              ruleId: this.rule.id,
              ruleName: this.rule.name,
              protocol: "udp",
              message: `UDP multi-client return error for ${remote.address}:${remote.port}: ${error.message}`
            });
          }
        });
      });

      targetSocket.on("error", (error) => {
        this.status.lastError = error.message;
      });

      const timer = setTimeout(() => {
        this.closeSession(sessionKey, targetSocket, remote);
      }, this.sessionTimeoutMs);

      session = { targetSocket, timer };
      this.sessions.set(sessionKey, session);
      this.status.activeUdpSessions = this.sessions.size;

      this.onEvent?.({
        type: "udp.session.opened",
        severity: "info",
        ruleId: this.rule.id,
        ruleName: this.rule.name,
        protocol: "udp",
        message: `UDP session opened for ${remote.address}:${remote.port}.`,
        details: {
          clientAddress: remote.address,
          clientPort: remote.port,
          targetHost: this.rule.targetHost,
          targetPort: this.rule.targetPort
        }
      });
    } else {
      // Reset idle timer
      clearTimeout(session.timer);
      session.timer = setTimeout(() => {
        this.closeSession(sessionKey, session!.targetSocket, remote);
      }, this.sessionTimeoutMs);
    }

    const now = Date.now();
    if (now - this.lastForwardLogAt >= UDP_LOG_INTERVAL_MS) {
      this.lastForwardLogAt = now;
      this.onEvent?.({
        type: "udp.packet.forwarded",
        severity: "info",
        ruleId: this.rule.id,
        ruleName: this.rule.name,
        protocol: "udp",
        message: `UDP packet forwarded from ${remote.address}:${remote.port} (session).`,
        details: {
          fromAddress: remote.address,
          fromPort: remote.port,
          bytes: message.length
        }
      });
    }

    session.targetSocket.send(message, this.rule.targetPort, this.rule.targetHost, (error) => {
      if (error) {
        this.status.lastError = error.message;
        this.onEvent?.({
          type: "udp.packet.error",
          severity: "error",
          ruleId: this.rule.id,
          ruleName: this.rule.name,
          protocol: "udp",
          message: `UDP multi-client send error: ${error.message}`
        });
      }
    });
  }

  private closeSession(sessionKey: string, targetSocket: dgram.Socket, remote: RemoteInfo): void {
    this.sessions.delete(sessionKey);
    this.status.activeUdpSessions = this.sessions.size;
    clearSocket(targetSocket);

    this.onEvent?.({
      type: "udp.session.closed",
      severity: "info",
      ruleId: this.rule.id,
      ruleName: this.rule.name,
      protocol: "udp",
      message: `UDP session expired for ${remote.address}:${remote.port} (idle timeout).`,
      details: {
        clientAddress: remote.address,
        clientPort: remote.port
      }
    });
  }
}

async function closeSocket(socket: dgram.Socket | undefined): Promise<void> {
  if (!socket) {
    return;
  }
  await new Promise<void>((resolve) => {
    socket.close(() => resolve());
  });
}

function clearSocket(socket: dgram.Socket): void {
  socket.close(() => {});
}
