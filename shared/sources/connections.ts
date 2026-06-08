export type LiveConnectionStatus = "active";

export type UdpSessionStatus = "active" | "idle";

export interface TcpConnectionInfo {
  id: string;
  ruleId: string;
  ruleName: string;
  protocol: "tcp";
  clientAddress: string;
  clientPort: number;
  targetAddress: string;
  targetPort: number;
  startedAt: string;
  durationMs: number;
  bytesIn: number;
  bytesOut: number;
  status: LiveConnectionStatus;
}

export interface UdpSessionInfo {
  id: string;
  ruleId: string;
  ruleName: string;
  protocol: "udp";
  mode: "one-way" | "bidirectional-last-client" | "bidirectional-multi-client";
  clientAddress: string;
  clientPort: number;
  targetAddress: string;
  targetPort: number;
  startedAt: string;
  lastSeenAt: string;
  idleMs: number;
  packetsIn: number;
  packetsOut: number;
  bytesIn: number;
  bytesOut: number;
  status: UdpSessionStatus;
}

export interface RuleLiveSummary {
  ruleId: string;
  ruleName: string;
  protocol: "tcp" | "udp";
  activeTcpConnections: number;
  activeUdpSessions: number;
  bytesIn: number;
  bytesOut: number;
  packetsIn: number;
  packetsOut: number;
  lastTrafficAt: string | null;
}

export interface LiveConnectionsResponse {
  generatedAt: string;
  tcpConnections: TcpConnectionInfo[];
  udpSessions: UdpSessionInfo[];
  ruleSummaries: RuleLiveSummary[];
}
