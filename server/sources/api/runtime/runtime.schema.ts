import { ApiProperty } from "@nestjs/swagger";
import type { RuntimeInfo } from "@portier/shared";

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
}
