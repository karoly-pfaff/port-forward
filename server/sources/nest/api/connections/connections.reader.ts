import type { ForwardRule, TcpConnectionInfo, UdpSessionInfo } from "@portier/shared";

/**
 * Narrow read-only view of the forward manager that `GET /api/connections` needs
 * — the current rules plus the live TCP/UDP connection snapshots to aggregate.
 * The real domain `ForwardManager` satisfies it; tests inject a seeded fake (NO
 * real sockets/listeners). Mirrors the `StatusReader`/`ForwardsReader`/
 * `ConfigExportReader` seams. It reads only; no lifecycle/mutation.
 */
export interface ConnectionsReader {
  listRules(): ForwardRule[];
  getLiveTcpConnections(): TcpConnectionInfo[];
  getLiveUdpSessions(): UdpSessionInfo[];
}

/** Injection token for the connections reader. */
export const CONNECTIONS_READER = "CONNECTIONS_READER";

/**
 * Default: no forwarding runtime is wired into the NestJS app yet, so
 * there are no rules and no live connections. When the NestJS server becomes the
 * active runtime this token is bound to the shared `ForwardManager`; tests
 * override it with a seeded fake.
 */
export const emptyConnectionsReader: ConnectionsReader = {
  listRules: () => [],
  getLiveTcpConnections: () => [],
  getLiveUdpSessions: () => [],
};
