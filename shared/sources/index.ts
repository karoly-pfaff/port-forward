export type {
  ActivityEvent,
  ActivityEventInput,
  ActivityEventType,
  ActivitySeverity
} from "./activity.js";

export type ForwardProtocol = "tcp" | "udp";

export type UdpMode = "one-way" | "bidirectional-last-client" | "bidirectional-multi-client";

export const PORTIER_DEFAULT_HOST = "127.0.0.1";
export const PORTIER_DEFAULT_PORT = 47831;

export const PORTIER_RECOMMENDED_FORWARD_PORT_MIN = 48000;
export const PORTIER_RECOMMENDED_FORWARD_PORT_MAX = 48999;

export type CommonPortInfo = {
  port: number;
  label: string;
  category: "system" | "database" | "dev" | "network" | "debug" | "other";
  severity: "info" | "warning" | "danger";
};

export const COMMON_PORTS: CommonPortInfo[] = [
  { port: 20, label: "FTP data", category: "network", severity: "danger" },
  { port: 21, label: "FTP control", category: "network", severity: "danger" },
  { port: 22, label: "SSH", category: "system", severity: "danger" },
  { port: 25, label: "SMTP", category: "network", severity: "danger" },
  { port: 53, label: "DNS", category: "network", severity: "danger" },
  { port: 80, label: "HTTP", category: "network", severity: "danger" },
  { port: 110, label: "POP3", category: "network", severity: "danger" },
  { port: 123, label: "NTP", category: "network", severity: "danger" },
  { port: 143, label: "IMAP", category: "network", severity: "danger" },
  { port: 443, label: "HTTPS", category: "network", severity: "danger" },
  { port: 445, label: "SMB / Windows file sharing", category: "system", severity: "danger" },
  { port: 3000, label: "React / Next.js / common dev server", category: "dev", severity: "warning" },
  { port: 3001, label: "Common secondary dev server", category: "dev", severity: "warning" },
  { port: 3306, label: "MySQL / MariaDB", category: "database", severity: "warning" },
  { port: 3389, label: "Remote Desktop", category: "system", severity: "danger" },
  { port: 4200, label: "Angular dev server", category: "dev", severity: "warning" },
  { port: 5000, label: "Flask / common dev API", category: "dev", severity: "warning" },
  { port: 5173, label: "Vite dev server", category: "dev", severity: "warning" },
  { port: 5432, label: "PostgreSQL", category: "database", severity: "warning" },
  { port: 6379, label: "Redis", category: "database", severity: "warning" },
  { port: 8000, label: "Common dev HTTP server", category: "dev", severity: "warning" },
  { port: 8080, label: "Alternate HTTP / proxy / dev server", category: "dev", severity: "warning" },
  { port: 8443, label: "Alternate HTTPS", category: "network", severity: "warning" },
  { port: 9229, label: "Node.js debugger", category: "debug", severity: "danger" },
  { port: 27017, label: "MongoDB", category: "database", severity: "warning" }
];

export type PortAdvisory = {
  code:
    | "COMMON_PORT"
    | "PRIVILEGED_PORT"
    | "OUTSIDE_RECOMMENDED_RANGE"
    | "LAN_EXPOSURE"
    | "MANAGEMENT_LAN_EXPOSURE";
  severity: "info" | "warning" | "danger";
  message: string;
};

export interface ForwardRule {
  id: string;
  name: string;
  protocol: ForwardProtocol;
  listenHost: string;
  listenPort: number;
  targetHost: string;
  targetPort: number;
  enabled: boolean;
  udpMode?: UdpMode;
}

export interface ForwardStatus {
  ruleId: string;
  running: boolean;
  activeConnections?: number;
  bytesIn: number;
  bytesOut: number;
  packetsIn?: number;
  packetsOut?: number;
  activeUdpSessions?: number;
  lastError?: string;
  startedAt?: string;
}

export type ForwardRuleInput = Omit<ForwardRule, "id"> & { id?: string };

export type ForwardRuleResponse = ForwardRule & { advisories: PortAdvisory[] };

export type ImportMode = "replace" | "merge";

export interface RuntimeInfo {
  name: string;
  version: string;
  runtime: "node" | "go";
  platform: "windows" | "macos" | "linux" | "unknown";
  arch: "x64" | "arm64" | "unknown";
  uptimeSeconds: number;
  startedAt: string;
  managementHost: string;
  managementPort: number;
  configPath: string;
  staticDir: string;
  serviceMode: boolean;
  pid: number;
}

export interface ExportedConfig {
  version: "1";
  exportedAt: string;
  rules: ForwardRule[];
}

export interface ImportResult {
  imported: number;
  skipped: number;
  errors: string[];
}

export type DiagnosticStatus = "pass" | "warn" | "fail" | "skip";

export interface DiagnosticCheck {
  id: string;
  label: string;
  status: DiagnosticStatus;
  message: string;
  details?: Record<string, string | number | boolean | null>;
}

export interface DiagnosticSummary {
  status: "pass" | "warn" | "fail";
  message: string;
}

export interface RuleDiagnosticsResult {
  ruleId: string;
  ruleName: string;
  protocol: ForwardProtocol;
  summary: DiagnosticSummary;
  checks: DiagnosticCheck[];
  diagnosedAt: string;
}

export interface ValidationResult<T> {
  valid: boolean;
  value?: T;
  errors: string[];
}

export function validateForwardRule(input: unknown): ValidationResult<ForwardRuleInput> {
  const errors: string[] = [];
  const value = input as Partial<ForwardRuleInput>;

  if (!value || typeof value !== "object") {
    return { valid: false, errors: ["Rule must be an object."] };
  }

  if (value.id !== undefined && !isNonEmptyString(value.id)) {
    errors.push("id must be a non-empty string when provided.");
  }
  if (!isNonEmptyString(value.name)) {
    errors.push("name is required.");
  }
  if (value.protocol !== "tcp" && value.protocol !== "udp") {
    errors.push("protocol must be tcp or udp.");
  }
  if (!isNonEmptyString(value.listenHost)) {
    errors.push("listenHost is required.");
  }
  if (!isValidPort(value.listenPort)) {
    errors.push("listenPort must be an integer from 1 to 65535.");
  }
  if (!isNonEmptyString(value.targetHost)) {
    errors.push("targetHost is required.");
  }
  if (!isValidPort(value.targetPort)) {
    errors.push("targetPort must be an integer from 1 to 65535.");
  }
  if (typeof value.enabled !== "boolean") {
    errors.push("enabled must be a boolean.");
  }
  if (
    value.protocol === "udp" &&
    value.udpMode !== undefined &&
    value.udpMode !== "one-way" &&
    value.udpMode !== "bidirectional-last-client" &&
    value.udpMode !== "bidirectional-multi-client"
  ) {
    errors.push("udpMode must be one-way, bidirectional-last-client, or bidirectional-multi-client.");
  }
  if (value.protocol === "tcp" && value.udpMode !== undefined) {
    errors.push("udpMode is only valid for UDP rules.");
  }

  if (errors.length > 0) {
    return { valid: false, errors };
  }

  return {
    valid: true,
    errors: [],
    value: {
      id: value.id,
      name: value.name!.trim(),
      protocol: value.protocol!,
      listenHost: value.listenHost!.trim(),
      listenPort: value.listenPort!,
      targetHost: value.targetHost!.trim(),
      targetPort: value.targetPort!,
      enabled: value.enabled!,
      udpMode: value.protocol === "udp" ? value.udpMode ?? "one-way" : undefined
    }
  };
}

export function validateForwardRulePatch(input: unknown): ValidationResult<Partial<ForwardRuleInput>> {
  const errors: string[] = [];
  const value = input as Partial<ForwardRuleInput>;

  if (!value || typeof value !== "object") {
    return { valid: false, errors: ["Patch must be an object."] };
  }

  if (value.id !== undefined && !isNonEmptyString(value.id)) {
    errors.push("id must be a non-empty string when provided.");
  }
  if (value.name !== undefined && !isNonEmptyString(value.name)) {
    errors.push("name must be a non-empty string.");
  }
  if (value.protocol !== undefined && value.protocol !== "tcp" && value.protocol !== "udp") {
    errors.push("protocol must be tcp or udp.");
  }
  if (value.listenHost !== undefined && !isNonEmptyString(value.listenHost)) {
    errors.push("listenHost must be a non-empty string.");
  }
  if (value.listenPort !== undefined && !isValidPort(value.listenPort)) {
    errors.push("listenPort must be an integer from 1 to 65535.");
  }
  if (value.targetHost !== undefined && !isNonEmptyString(value.targetHost)) {
    errors.push("targetHost must be a non-empty string.");
  }
  if (value.targetPort !== undefined && !isValidPort(value.targetPort)) {
    errors.push("targetPort must be an integer from 1 to 65535.");
  }
  if (value.enabled !== undefined && typeof value.enabled !== "boolean") {
    errors.push("enabled must be a boolean.");
  }
  if (
    value.udpMode !== undefined &&
    value.udpMode !== "one-way" &&
    value.udpMode !== "bidirectional-last-client" &&
    value.udpMode !== "bidirectional-multi-client"
  ) {
    errors.push("udpMode must be one-way, bidirectional-last-client, or bidirectional-multi-client.");
  }

  if (errors.length > 0) {
    return { valid: false, errors };
  }

  return {
    valid: true,
    errors: [],
    value: {
      ...value,
      // Only include trimmed strings when the field was present in the patch.
      // Absent fields must not appear as `undefined` — that would overwrite the
      // existing rule's value when the caller spreads this result.
      ...(value.name !== undefined ? { name: value.name.trim() } : undefined),
      ...(value.listenHost !== undefined ? { listenHost: value.listenHost.trim() } : undefined),
      ...(value.targetHost !== undefined ? { targetHost: value.targetHost.trim() } : undefined),
    }
  };
}

export function listenKey(rule: Pick<ForwardRule, "protocol" | "listenHost" | "listenPort">): string {
  return `${rule.protocol}:${rule.listenHost}:${rule.listenPort}`;
}

export function getCommonPortInfo(port: number): CommonPortInfo | undefined {
  return COMMON_PORTS.find((info) => info.port === port);
}

export function isRecommendedForwardPort(port: number): boolean {
  return port >= PORTIER_RECOMMENDED_FORWARD_PORT_MIN && port <= PORTIER_RECOMMENDED_FORWARD_PORT_MAX;
}

export function getPortAdvisories(input: {
  port: number;
  listenHost?: string;
  purpose: "management" | "forward";
}): PortAdvisory[] {
  const advisories: PortAdvisory[] = [];
  const commonPort = getCommonPortInfo(input.port);

  if (commonPort) {
    advisories.push({
      code: "COMMON_PORT",
      severity: commonPort.severity,
      message: `Port ${input.port} is commonly used by ${commonPort.label}.`
    });
  }

  if (input.port < 1024) {
    advisories.push({
      code: "PRIVILEGED_PORT",
      severity: "danger",
      message: `Port ${input.port} is privileged and may require elevated permissions.`
    });
  }

  if (input.purpose === "forward" && !isRecommendedForwardPort(input.port)) {
    advisories.push({
      code: "OUTSIDE_RECOMMENDED_RANGE",
      severity: "info",
      message: `Port ${input.port} is outside Portier's recommended forwarding range ${PORTIER_RECOMMENDED_FORWARD_PORT_MIN}-${PORTIER_RECOMMENDED_FORWARD_PORT_MAX}.`
    });
  }

  if (input.listenHost === "0.0.0.0" && input.purpose === "forward") {
    advisories.push({
      code: "LAN_EXPOSURE",
      severity: "warning",
      message: "Listening on 0.0.0.0 exposes this forwarded port on the LAN."
    });
  }

  if (input.listenHost === "0.0.0.0" && input.purpose === "management") {
    advisories.push({
      code: "MANAGEMENT_LAN_EXPOSURE",
      severity: "danger",
      message: "Listening on 0.0.0.0 exposes the Portier management UI/API on the LAN."
    });
  }

  return advisories;
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
}

function isValidPort(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value >= 1 && value <= 65535;
}
