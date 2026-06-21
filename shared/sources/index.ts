export type {
  ActivityEvent,
  ActivityEventInput,
  ActivityEventType,
  ActivitySeverity
} from "./activity.js";

export type {
  LiveConnectionStatus,
  UdpSessionStatus,
  TcpConnectionInfo,
  UdpSessionInfo,
  RuleLiveSummary,
  LiveConnectionsResponse
} from "./connections.js";

export type {
  ConfigPlanOperationType,
  ConfigPlanChange,
  ConfigPlanRuleSnapshot,
  ConfigPlanOperation,
  ConfigPlanSummary,
  ConfigPlanError,
  ConfigPlanWarning,
  ConfigPlanResponse,
  DesiredConfig,
  ConfigPlanRequest,
  ConfigAppliedCounts,
  ConfigApplyRequest,
  ConfigApplyResponse
} from "./plan.js";

export type {
  GroupActionType,
  GroupActionResultStatus,
  GroupActionResult,
  GroupActionResponse
} from "./groups.js";
export { summarizeGroupAction } from "./groups.js";

export type ForwardProtocol = "tcp" | "udp";

export type UdpMode = "one-way" | "bidirectional-last-client" | "bidirectional-multi-client";

export const PORTIER_DEFAULT_HOST = "127.0.0.1";
export const PORTIER_DEFAULT_PORT = 47831;

export const PORTIER_RECOMMENDED_FORWARD_PORT_MIN = 48000;
export const PORTIER_RECOMMENDED_FORWARD_PORT_MAX = 48999;

export const PORTIER_APP_VERSION = "1.19.0";

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
  /**
   * Optional, behavior-neutral grouping label for the rule. Used only as
   * operator-facing metadata (v1.8 Operator Power Tools) — it does NOT affect
   * forwarding, duplicate-binding, lifecycle, or status behavior. Absent when
   * the rule has no group. See {@link PORTIER_GROUP_MAX_LENGTH} and
   * {@link validateGroup} for the normalization/validation rules.
   */
  group?: string;
}

/** Maximum length (characters) of a normalized rule group label. */
export const PORTIER_GROUP_MAX_LENGTH = 64;

/**
 * Operator-facing health classification for a rule (v1.8 Slice 7). Derived
 * deterministically from existing runtime state — it does NOT probe targets or
 * run any background check. Distinct from the lifecycle `running` state:
 * - `error`:   the rule has a current `lastError` (failed start / socket error).
 * - `warning`: the rule is enabled (autostart) but is not currently running.
 * - `healthy`: running cleanly, or intentionally stopped (not enabled, no error).
 */
export type RuleHealth = "healthy" | "warning" | "error";

export interface ForwardStatus {
  ruleId: string;
  running: boolean;
  health: RuleHealth;
  activeConnections?: number;
  bytesIn: number;
  bytesOut: number;
  packetsIn?: number;
  packetsOut?: number;
  activeUdpSessions?: number;
  lastError?: string;
  startedAt?: string;
}

/**
 * Derive a rule's {@link RuleHealth} from existing runtime state. Pure and
 * deterministic; performs no I/O. The TypeScript server and Go service share
 * this exact logic (parity-tested via validate:contract). Priority: a present
 * `lastError` is always `error`; otherwise an enabled-but-stopped rule is
 * `warning`; everything else is `healthy`.
 */
export function deriveRuleHealth(input: {
  enabled: boolean;
  running: boolean;
  lastError?: string;
}): RuleHealth {
  if (input.lastError !== undefined && input.lastError.trim().length > 0) {
    return "error";
  }
  if (input.enabled && !input.running) {
    return "warning";
  }
  return "healthy";
}

export type ForwardRuleInput = Omit<ForwardRule, "id"> & { id?: string };

export type ForwardRuleResponse = ForwardRule & { advisories: PortAdvisory[] };

export type ImportMode = "replace" | "merge";

/**
 * Stable, machine-readable reason a runtime entered config recovery mode
 * (v1.17). Kebab-case to match the internal recovery state and the existing
 * kebab API convention (e.g. `udpMode`). Only file-level config-load failures
 * set global recovery; per-rule autostart/duplicate failures stay rule-level
 * (`ForwardStatus.lastError` + `health: "error"`), never setting this.
 */
export type RuntimeRecoveryReason =
  | "unreadable"
  | "malformed"
  | "schema-invalid"
  | "unsupported-version";

/**
 * Runtime config-recovery state, exposed on `GET /api/runtime` (v1.17). Always
 * present and backward-compatible: `{ active: false }` in normal operation; when
 * active, the remaining fields describe the condition. Messages are operator-safe
 * (no file contents); paths match the existing local-diagnostics posture
 * (`configPath` is already reported). `quarantinePath` is "" when nothing was
 * quarantined (e.g. an unreadable file is preserved in place).
 */
export interface RuntimeRecovery {
  active: boolean;
  reason?: RuntimeRecoveryReason;
  message?: string;
  configPath?: string;
  quarantinePath?: string;
  writesBlocked?: boolean;
  detectedAt?: string;
}

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
  recovery: RuntimeRecovery;
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
  collectGroupErrors(value.group, errors);

  if (errors.length > 0) {
    return { valid: false, errors };
  }

  const group = normalizeGroup(value.group);

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
      udpMode: value.protocol === "udp" ? value.udpMode ?? "one-way" : undefined,
      // Omit the key entirely when there is no group, so exported configs and
      // API responses stay byte-identical to legacy (no-group) rules.
      ...(group !== undefined ? { group } : {})
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
  collectGroupErrors(value.group, errors);

  if (errors.length > 0) {
    return { valid: false, errors };
  }

  const patch: Partial<ForwardRuleInput> = {
    ...value,
    // Only include trimmed strings when the field was present in the patch.
    // Absent fields must not appear as `undefined` — that would overwrite the
    // existing rule's value when the caller spreads this result.
    ...(value.name !== undefined ? { name: value.name.trim() } : undefined),
    ...(value.listenHost !== undefined ? { listenHost: value.listenHost.trim() } : undefined),
    ...(value.targetHost !== undefined ? { targetHost: value.targetHost.trim() } : undefined),
  };

  // Group patch semantics (mirrors Go ApplyPatch):
  //   - key absent or null  → unchanged (drop the key so the spread won't touch it)
  //   - non-empty string    → set to the trimmed value
  //   - empty/whitespace    → clear (set undefined so the merge removes the group)
  if (value.group === undefined || value.group === null) {
    delete patch.group;
  } else {
    patch.group = normalizeGroup(value.group);
  }

  return { valid: true, errors: [], value: patch };
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
      message: "Listening on 0.0.0.0 exposes this forwarded port on all interfaces. Other LAN devices may be able to connect if firewall settings allow it."
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

/**
 * Validate a candidate rule group label, returning error messages (empty when
 * valid). Optional metadata: `undefined`/`null`/empty/whitespace are all
 * accepted (they normalize to "no group"); a present non-empty value must be a
 * string of at most {@link PORTIER_GROUP_MAX_LENGTH} characters with no control
 * characters. Exported for reuse and documentation; the TypeScript and Go
 * runtimes apply the same rule (parity-tested via validate:contract).
 */
export function validateGroup(group: unknown): string[] {
  const errors: string[] = [];
  collectGroupErrors(group, errors);
  return errors;
}

/**
 * Validate a group label used as a **group-operation target** (e.g. the path of
 * a group start/stop request). Unlike {@link validateGroup}, an empty or
 * whitespace-only value is rejected (`group is required.`) — you cannot act on
 * "no group". A present value must still satisfy the normal length/character
 * rules. Returns error messages (empty when valid). Both runtimes apply this.
 */
export function validateGroupName(group: unknown): string[] {
  if (typeof group !== "string" || group.trim().length === 0) {
    return ["group is required."];
  }
  return validateGroup(group.trim());
}

function collectGroupErrors(group: unknown, errors: string[]): void {
  if (group === undefined || group === null) {
    return;
  }
  if (typeof group !== "string") {
    errors.push("group must be a string.");
    return;
  }
  const trimmed = group.trim();
  if (trimmed.length === 0) {
    return; // normalized to absent
  }
  if (trimmed.length > PORTIER_GROUP_MAX_LENGTH) {
    errors.push(`group must be ${PORTIER_GROUP_MAX_LENGTH} characters or fewer.`);
  }
  if (hasControlChar(trimmed)) {
    errors.push("group must not contain control characters.");
  }
}

// Control characters: C0 range (U+0000-U+001F) plus DEL (U+007F). Rejected in
// group labels so the metadata stays a simple human-readable name. Checked with
// charCodeAt rather than a regex literal to keep the source free of embedded
// control bytes.
function hasControlChar(value: string): boolean {
  for (let i = 0; i < value.length; i++) {
    const code = value.charCodeAt(i);
    if (code < 0x20 || code === 0x7f) {
      return true;
    }
  }
  return false;
}

/**
 * Normalize a group label to its stored form: trims whitespace and returns
 * `undefined` when the result is empty (so a "no group" rule omits the field).
 * Assumes the value already passed {@link validateGroup}.
 */
function normalizeGroup(group: unknown): string | undefined {
  if (typeof group !== "string") {
    return undefined;
  }
  const trimmed = group.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
}

function isValidPort(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value >= 1 && value <= 65535;
}
