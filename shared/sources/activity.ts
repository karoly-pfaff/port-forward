export type ActivityEventType =
  | "rule.created"
  | "rule.updated"
  | "rule.deleted"
  | "rule.started"
  | "rule.stopped"
  | "rule.error"
  | "tcp.connection.opened"
  | "tcp.connection.closed"
  | "tcp.connection.error"
  | "udp.packet.forwarded"
  | "udp.packet.returned"
  | "udp.packet.error"
  | "udp.session.opened"
  | "udp.session.closed"
  | "config.exported"
  | "config.imported"
  | "config.import.failed";

export type ActivitySeverity = "info" | "success" | "warning" | "error";

export type ActivityEvent = {
  id: string;
  timestamp: string;
  type: ActivityEventType;
  severity: ActivitySeverity;
  ruleId?: string;
  ruleName?: string;
  protocol?: "tcp" | "udp";
  message: string;
  details?: Record<string, string | number | boolean | null>;
};

export type ActivityEventInput = Omit<ActivityEvent, "id" | "timestamp">;
