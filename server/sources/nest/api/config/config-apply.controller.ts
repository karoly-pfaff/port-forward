import { Body, Controller, HttpCode, Inject, Post } from "@nestjs/common";
import { ApiBadRequestResponse, ApiBody, ApiOkResponse, ApiOperation, ApiTags } from "@nestjs/swagger";
import { ApiErrorResponseDto } from "../../common/api-error.schema.js";
import { ConfigApplyBodyDto } from "./config-apply.schema.js";
import { ConfigApplyService } from "./config-apply.service.js";
import { ConfigApplyResponseDto, toConfigApplyResponseDto } from "./config-apply.response.dto.js";

/**
 * Transport adapter for `POST /api/config/apply` — the MUTATING desired-state apply.
 * Validation (the `desired`-key-presence check and the destructive-confirmation gate)
 * is delegated to the service to match Express's inline checks exactly, so the
 * `ConfigApplyBodyDto` is documentation/typing only (no validation pipe). All
 * success / `ok:false` outcomes are status `200` (so `@HttpCode(200)` — NestJS would
 * otherwise default `POST` to `201`); the two gating errors (missing `desired`,
 * destructive-without-`yes`) are thrown by the service as `ApiBadRequestException`
 * and flow through the shared `{errors}` error filter. Unlike import there is no
 * `422`, so no `@Res` is needed.
 */
@ApiTags("config")
@Controller("api/config")
export class ConfigApplyController {
  constructor(@Inject(ConfigApplyService) private readonly configApply: ConfigApplyService) {}

  @Post("apply")
  @HttpCode(200)
  @ApiOperation({
    summary: "Apply a config change",
    description:
      "Diffs the desired config against the current rules and applies it. A missing `desired` key returns 400; a " +
      "plan with errors returns 200 `ok:false` (no mutation). `dryRun:true` returns the plan + applied counts " +
      "without mutating (and is allowed even for destructive plans). A non-dry-run plan with destructive " +
      "operations requires `yes:true` (else 400). When the plan has drift, the rules are replaced (MUTATING; " +
      "enabled rules start). Unlike `plan` (always a dry-run) and `import` (a raw replace/merge), apply gates on " +
      "drift/destructive/confirmation and returns the plan with an `appliedAt` timestamp.",
  })
  @ApiBody({ type: ConfigApplyBodyDto })
  @ApiOkResponse({
    type: ConfigApplyResponseDto,
    description: "The apply result (ok:true applied / ok:true dry-run / ok:false on plan errors), with the plan.",
  })
  @ApiBadRequestResponse({
    type: ApiErrorResponseDto,
    description: "`desired` is required, or destructive operations require `yes:true`.",
  })
  async apply(@Body() body: unknown): Promise<ConfigApplyResponseDto> {
    return toConfigApplyResponseDto(await this.configApply.apply(body));
  }
}
