import { ApiProperty, ApiPropertyOptional } from "@nestjs/swagger";
import type { RuntimeInfo, RuntimeRecovery } from "@portier/shared";

/**
 * The additive `recovery` block on `GET /api/runtime` (v1.17). Always present:
 * `{ active: false }` in normal operation. When active, the optional fields
 * describe the config-load recovery condition (operator-safe; no file contents).
 */
export class RuntimeRecoveryDto implements RuntimeRecovery {
  @ApiProperty({ type: Boolean, description: "Whether the runtime is in config recovery mode." })
  active!: boolean;
  @ApiPropertyOptional({
    enum: ["unreadable", "malformed", "schema-invalid", "unsupported-version"],
    description: "Why recovery is active (omitted when inactive).",
  })
  reason?: RuntimeRecovery["reason"];
  @ApiPropertyOptional({ type: String, description: "Operator-safe summary." })
  message?: string;
  @ApiPropertyOptional({ type: String, description: "Config path Portier tried to load." })
  configPath?: string;
  @ApiPropertyOptional({ type: String, description: "Where the bad config was quarantined (empty when none)." })
  quarantinePath?: string;
  @ApiPropertyOptional({ type: Boolean, description: "Whether rule writes are blocked while recovery is active." })
  writesBlocked?: boolean;
  @ApiPropertyOptional({ type: String, format: "date-time", description: "When the recovery condition was detected." })
  detectedAt?: string;
}

/** Response body for `GET /api/runtime` — metadata-only, `implements RuntimeInfo`. */
export class RuntimeInfoResponseDto implements RuntimeInfo {
  @ApiProperty({ type: String, example: "Portier" }) name!: string;
  @ApiProperty({ type: String, example: "1.16.0" }) version!: string;
  @ApiProperty({ enum: ["node", "go"], description: "Runtime implementation serving the API." })
  runtime!: RuntimeInfo["runtime"];
  @ApiProperty({ enum: ["windows", "macos", "linux", "unknown"] }) platform!: RuntimeInfo["platform"];
  @ApiProperty({ enum: ["x64", "arm64", "unknown"] }) arch!: RuntimeInfo["arch"];
  @ApiProperty({ type: Number, description: "Seconds since the runtime started." }) uptimeSeconds!: number;
  @ApiProperty({ type: String, format: "date-time" }) startedAt!: string;
  @ApiProperty({ type: String }) managementHost!: string;
  @ApiProperty({ type: Number }) managementPort!: number;
  @ApiProperty({ type: String }) configPath!: string;
  @ApiProperty({ type: String }) staticDir!: string;
  @ApiProperty({ type: Boolean }) serviceMode!: boolean;
  @ApiProperty({ type: Number, description: "Process id." }) pid!: number;
  @ApiProperty({ type: RuntimeRecoveryDto, description: "Config-recovery state (v1.17); { active: false } in normal operation." })
  recovery!: RuntimeRecovery;
}
