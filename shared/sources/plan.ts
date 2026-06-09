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

export interface ConfigApplyRequest {
  desired: DesiredConfig;
  yes: boolean;
  backup?: boolean;
}

export interface ConfigApplyResponse {
  appliedAt: string;
  applied: number;
  errors: ConfigPlanError[];
  warnings: ConfigPlanWarning[];
}
