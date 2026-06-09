export type ConfigPlanOperationType = "add" | "update" | "remove" | "unchanged";

export interface ConfigPlanChange {
  field: string;
  before: unknown;
  after: unknown;
}

export interface ConfigPlanRuleSnapshot {
  id?: string;
  name: string;
  protocol: "tcp" | "udp";
  listenHost: string;
  listenPort: number;
  targetHost: string;
  targetPort: number;
  enabled: boolean;
  udpMode?: "one-way" | "bidirectional-last-client" | "bidirectional-multi-client";
}

export interface ConfigPlanOperation {
  type: ConfigPlanOperationType;
  ruleId?: string;
  ruleName: string;
  protocol: "tcp" | "udp";
  current?: ConfigPlanRuleSnapshot;
  desired?: ConfigPlanRuleSnapshot;
  changes?: ConfigPlanChange[];
  destructive: boolean;
}

export interface ConfigPlanSummary {
  add: number;
  update: number;
  remove: number;
  unchanged: number;
  destructive: number;
  hasDrift: boolean;
  hasErrors: boolean;
}

export interface ConfigPlanError {
  code: string;
  message: string;
  field?: string;
}

export interface ConfigPlanWarning {
  code: string;
  message: string;
}

export interface ConfigPlanResponse {
  generatedAt: string;
  mode: "plan";
  summary: ConfigPlanSummary;
  operations: ConfigPlanOperation[];
  errors: ConfigPlanError[];
  warnings: ConfigPlanWarning[];
}

export interface DesiredConfig {
  rules: ConfigPlanRuleSnapshot[];
}

export interface ConfigPlanRequest {
  desired: DesiredConfig;
}

export interface ConfigAppliedCounts {
  add: number;
  update: number;
  remove: number;
  unchanged: number;
}

export interface ConfigApplyRequest {
  desired: DesiredConfig;
  yes: boolean;
  dryRun?: boolean;
}

export interface ConfigApplyResponse {
  ok: boolean;
  dryRun: boolean;
  appliedAt: string;
  plan: ConfigPlanResponse;
  applied: ConfigAppliedCounts;
}
