import dgram from "node:dgram";
import type { RemoteInfo } from "node:dgram";
import type {
  ForwardRule,
  ForwardStatus,
  ActivityEventInput,
  ActivityEventType,
  ActivitySeverity
} from "@portier/shared";
import type { Forwarder } from "./types.js";
import type { UdpSessionRegistry } from "../connections/udp-session-registry.js";

const UDP_LOG_INTERVAL_MS = 1000;
const DEFAULT_SESSION_TIMEOUT_MS = 60_000;

interface UdpSession {
  targetSocket: dgram.Socket;
  timer: ReturnType<typeof setTimeout>;
  registryId?: string;
}

export class UdpForwarder implements Forwarder {
  private listenSocket?: dgram.Socket;
  private targetSocket?: dgram.Socket; // shared socket for one-way / last-client modes
  private lastClient?: RemoteInfo;
  private lastClientSessionId?: string;
  private sessions = new Map<string, UdpSession>();
  private status: ForwardStatus;
  private lastForwardLogAt = 0;
  private lastReturnLogAt = 0;
  private readonly sessionTimeoutMs: number;
  private readonly events: UdpEventEmitter;

  constructor(
    private readonly rule: ForwardRule,
    onEvent?: (event: ActivityEventInput) => void,
    sessionTimeoutMs?: number,
    private readonly registry?: UdpSessionRegistry
  ) {
    this.sessionTimeoutMs = sessionTimeoutMs ?? DEFAULT_SESSION_TIMEOUT_MS;
    this.events = new UdpEventEmitter(rule, onEvent);
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
        // Detect last-client change: close previous session for new client
        if (mode === "bidirectional-last-client" && this.registry) {
          const prev = this.lastClient;
          if (prev && (prev.address !== remote.address || prev.port !== remote.port)) {
            if (this.lastClientSessionId) {
              this.registry.closeSession(this.lastClientSessionId);
              this.lastClientSessionId = undefined;
            }
          }
        }

        this.lastClient = remote;
        this.status.bytesIn += message.length;
        this.status.packetsIn = (this.status.packetsIn ?? 0) + 1;

        const sessionId = this.registry?.openOrTouchSession({
          ruleId: this.rule.id,
          ruleName: this.rule.name,
          mode,
          clientAddress: remote.address,
          clientPort: remote.port,
          targetAddress: this.rule.targetHost,
          targetPort: this.rule.targetPort
        });
        if (mode === "bidirectional-last-client") this.lastClientSessionId = sessionId;
        if (sessionId) this.registry?.recordInbound(sessionId, message.length);

        targetSocket.send(message, this.rule.targetPort, this.rule.targetHost, (error) => {
          if (error) {
            this.status.lastError = error.message;
            this.events.packetError(`UDP send error: ${error.message}`);
            return;
          }

          const now = Date.now();
          if (now - this.lastForwardLogAt >= UDP_LOG_INTERVAL_MS) {
            this.lastForwardLogAt = now;
            this.events.packetForwarded(
              `UDP packet forwarded from ${remote.address}:${remote.port} to ${this.rule.targetHost}:${this.rule.targetPort}.`,
              {
                fromAddress: remote.address,
                fromPort: remote.port,
                targetHost: this.rule.targetHost,
                targetPort: this.rule.targetPort,
                bytes: message.length
              }
            );
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
          if (this.lastClientSessionId) this.registry?.recordOutbound(this.lastClientSessionId, message.length);

          const client = this.lastClient;
          listenSocket.send(message, client.port, client.address, (error) => {
            if (error) {
              this.status.lastError = error.message;
              this.events.packetError(`UDP return error: ${error.message}`);
              return;
            }

            const now = Date.now();
            if (now - this.lastReturnLogAt >= UDP_LOG_INTERVAL_MS) {
              this.lastReturnLogAt = now;
              this.events.packetReturned(
                `UDP reply returned to ${client.address}:${client.port}.`,
                {
                  toAddress: client.address,
                  toPort: client.port,
                  bytes: message.length
                }
              );
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
    this.lastClientSessionId = undefined;

    await Promise.all([closeSocket(listenSocket), closeSocket(targetSocket)]);

    this.registry?.closeSessionsForRule(this.rule.id);

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

    const registryId = this.registry?.openOrTouchSession({
      ruleId: this.rule.id,
      ruleName: this.rule.name,
      mode: "bidirectional-multi-client",
      clientAddress: remote.address,
      clientPort: remote.port,
      targetAddress: this.rule.targetHost,
      targetPort: this.rule.targetPort
    });

    let session = this.sessions.get(sessionKey);

    if (!session) {
      const targetSocket = dgram.createSocket("udp4");

      targetSocket.on("message", (response) => {
        if (!this.listenSocket) return;
        this.status.bytesOut += response.length;
        this.status.packetsOut = (this.status.packetsOut ?? 0) + 1;
        if (registryId) this.registry?.recordOutbound(registryId, response.length);

        listenSocket.send(response, remote.port, remote.address, (error) => {
          if (error) {
            this.status.lastError = error.message;
            this.events.packetError(
              `UDP multi-client return error for ${remote.address}:${remote.port}: ${error.message}`
            );
          }
        });
      });

      targetSocket.on("error", (error) => {
        this.status.lastError = error.message;
      });

      const timer = setTimeout(() => {
        this.closeSession(sessionKey, targetSocket, remote);
      }, this.sessionTimeoutMs);

      session = { targetSocket, timer, registryId };
      this.sessions.set(sessionKey, session);
      this.status.activeUdpSessions = this.sessions.size;

      this.events.sessionOpened(
        `UDP session opened for ${remote.address}:${remote.port}.`,
        {
          clientAddress: remote.address,
          clientPort: remote.port,
          targetHost: this.rule.targetHost,
          targetPort: this.rule.targetPort
        }
      );
    } else {
      // Reset idle timer
      clearTimeout(session.timer);
      session.timer = setTimeout(() => {
        this.closeSession(sessionKey, session!.targetSocket, remote);
      }, this.sessionTimeoutMs);
    }

    if (registryId) this.registry?.recordInbound(registryId, message.length);

    const now = Date.now();
    if (now - this.lastForwardLogAt >= UDP_LOG_INTERVAL_MS) {
      this.lastForwardLogAt = now;
      this.events.packetForwarded(
        `UDP packet forwarded from ${remote.address}:${remote.port} (session).`,
        {
          fromAddress: remote.address,
          fromPort: remote.port,
          bytes: message.length
        }
      );
    }

    session.targetSocket.send(message, this.rule.targetPort, this.rule.targetHost, (error) => {
      if (error) {
        this.status.lastError = error.message;
        this.events.packetError(`UDP multi-client send error: ${error.message}`);
      }
    });
  }

  private closeSession(sessionKey: string, targetSocket: dgram.Socket, remote: RemoteInfo): void {
    const session = this.sessions.get(sessionKey);
    this.sessions.delete(sessionKey);
    this.status.activeUdpSessions = this.sessions.size;
    if (session?.registryId) this.registry?.closeSession(session.registryId);
    clearSocket(targetSocket);

    this.events.sessionClosed(
      `UDP session expired for ${remote.address}:${remote.port} (idle timeout).`,
      {
        clientAddress: remote.address,
        clientPort: remote.port
      }
    );
  }
}

/**
 * UdpEventEmitter centralizes UDP activity-event construction for one rule so the
 * forwarder's many emission sites no longer each re-spell the ruleId/ruleName/
 * protocol envelope (and the identical udp.packet.error shape). It only builds and
 * dispatches ActivityEventInput payloads — throttling, status mutation, and session
 * bookkeeping stay in the forwarder. Emitted payloads are byte-identical to the
 * previous inline object literals (same fields, same order, details omitted when
 * not provided).
 */
class UdpEventEmitter {
  constructor(
    private readonly rule: ForwardRule,
    private readonly onEvent?: (event: ActivityEventInput) => void
  ) {}

  packetError(message: string): void {
    this.emit("udp.packet.error", "error", message);
  }

  packetForwarded(message: string, details: UdpEventDetails): void {
    this.emit("udp.packet.forwarded", "info", message, details);
  }

  packetReturned(message: string, details: UdpEventDetails): void {
    this.emit("udp.packet.returned", "info", message, details);
  }

  sessionOpened(message: string, details: UdpEventDetails): void {
    this.emit("udp.session.opened", "info", message, details);
  }

  sessionClosed(message: string, details: UdpEventDetails): void {
    this.emit("udp.session.closed", "info", message, details);
  }

  private emit(
    type: ActivityEventType,
    severity: ActivitySeverity,
    message: string,
    details?: UdpEventDetails
  ): void {
    this.onEvent?.({
      type,
      severity,
      ruleId: this.rule.id,
      ruleName: this.rule.name,
      protocol: "udp",
      message,
      ...(details ? { details } : {})
    });
  }
}

type UdpEventDetails = NonNullable<ActivityEventInput["details"]>;

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
