import { Body, Controller, HttpCode, Inject, Post } from "@nestjs/common";
import { ApiBadRequestResponse, ApiBody, ApiOkResponse, ApiOperation, ApiTags } from "@nestjs/swagger";
import { ApiErrorResponseDto, ConfigPlanBodyDto } from "../../common/api-schemas.js";
import { ConfigPlanService } from "./config-plan.service.js";
import { ConfigPlanResponseDto, toConfigPlanResponseDto } from "./config-plan.response.dto.js";

/**
 * Transport adapter for `POST /api/config/plan` — a NON-MUTATING dry-run that diffs
 * the desired config against the current rules. The body validation (`desired` must
 * be present) is delegated to the service to match Express's `"desired" in body`
 * key-presence check exactly (`desired: null` is allowed and surfaces as a plan
 * error, NOT a `400` — so the `ConfigPlanBodyDto` is documentation/typing only, no
 * validation pipe). `@HttpCode(200)` because NestJS defaults `POST` to `201`.
 */
@ApiTags("config")
@Controller("api/config")
export class ConfigPlanController {
  constructor(@Inject(ConfigPlanService) private readonly configPlan: ConfigPlanService) {}

  @Post("plan")
  @HttpCode(200)
  @ApiOperation({
    summary: "Plan a config change",
    description:
      "Diffs the desired config against the current rules and returns the plan (operations, summary, errors, " +
      "warnings) without mutating anything. A missing `desired` key returns 400; invalid desired rules surface " +
      "as plan errors (still 200). Returns 200 (matching Express; NestJS would otherwise default POST to 201).",
  })
  @ApiBody({ type: ConfigPlanBodyDto })
  @ApiOkResponse({ type: ConfigPlanResponseDto, description: "The config plan." })
  @ApiBadRequestResponse({ type: ApiErrorResponseDto, description: "`desired` is required." })
  plan(@Body() body: unknown): ConfigPlanResponseDto {
    return toConfigPlanResponseDto(this.configPlan.plan(body));
  }
}
