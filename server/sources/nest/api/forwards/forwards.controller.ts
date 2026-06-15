import { Body, Controller, Delete, Get, HttpCode, Inject, Param, Patch, Post } from "@nestjs/common";
import {
  ApiBadRequestResponse,
  ApiBody,
  ApiConflictResponse,
  ApiCreatedResponse,
  ApiNoContentResponse,
  ApiNotFoundResponse,
  ApiOkResponse,
  ApiOperation,
  ApiParam,
  ApiTags,
} from "@nestjs/swagger";
import {
  ApiErrorResponseDto,
  CreateForwardRuleBodyDto,
  ForwardRuleResponseDto,
  ForwardStatusDto,
  UpdateForwardRuleBodyDto,
} from "../../common/api-schemas.js";
import { CreateForwardRuleService } from "./create-forward-rule.service.js";
import { DeleteForwardRuleService } from "./delete-forward-rule.service.js";
import { StartForwardRuleService } from "./start-forward-rule.service.js";
import { UpdateForwardRuleService } from "./update-forward-rule.service.js";
import { toForwardRuleResponseDto } from "./forward-rule.response.dto.js";
import { toForwardStatusResponseDto } from "./forward-status.response.dto.js";
import { ForwardsService } from "./forwards.service.js";
import { toForwardsListResponseDto, type ForwardsListResponseDto } from "./forwards-list.response.dto.js";

/**
 * Transport adapter for `/api/forwards`. `GET` lists rules (read); `POST` creates
 * a rule (write — the first migrated rule mutation). Both map service results to
 * their response DTO at the HTTP boundary (matching the Express routes). The
 * `POST` body is NOT run through a validation pipe — validation is delegated to
 * the shared `validateForwardRule` inside the manager (a documented parity
 * exception), so `@ApiBody` documents the schema explicitly. List/create/update/
 * delete and the lifecycle **start** (`POST :id/start`) are migrated; the remaining
 * lifecycle routes under `/api/forwards/...` (stop/reorder/group/diagnose) stay
 * with Express.
 */
@ApiTags("forwards")
@Controller("api/forwards")
export class ForwardsController {
  constructor(
    @Inject(ForwardsService) private readonly forwards: ForwardsService,
    @Inject(CreateForwardRuleService) private readonly createService: CreateForwardRuleService,
    @Inject(UpdateForwardRuleService) private readonly updateService: UpdateForwardRuleService,
    @Inject(DeleteForwardRuleService) private readonly deleteService: DeleteForwardRuleService,
    @Inject(StartForwardRuleService) private readonly startService: StartForwardRuleService
  ) {}

  @Get()
  @ApiOperation({ summary: "List forward rules", description: "Returns every forward rule with its port advisories." })
  @ApiOkResponse({ type: ForwardRuleResponseDto, isArray: true, description: "All forward rules." })
  list(): ForwardsListResponseDto {
    return toForwardsListResponseDto(this.forwards.list());
  }

  @Post()
  @ApiOperation({
    summary: "Create a forward rule",
    description:
      "Creates a forward rule. A created rule with autostart enabled starts its forwarder (matching Express).",
  })
  @ApiBody({ type: CreateForwardRuleBodyDto })
  @ApiCreatedResponse({ type: ForwardRuleResponseDto, description: "The created rule with its port advisories." })
  @ApiBadRequestResponse({ type: ApiErrorResponseDto, description: "Invalid rule definition." })
  @ApiConflictResponse({ type: ApiErrorResponseDto, description: "A rule already listens on that binding." })
  async create(@Body() body: CreateForwardRuleBodyDto): Promise<ForwardRuleResponseDto> {
    return toForwardRuleResponseDto(await this.createService.create(body));
  }

  @Patch(":id")
  @ApiOperation({
    summary: "Update a forward rule",
    description:
      "Partially updates a rule. Unspecified fields are left unchanged. A change to a forwarding field " +
      "(protocol/listenHost/listenPort/targetHost/targetPort/udpMode) restarts the forwarder only if the rule " +
      "is running; metadata-only changes (name/group/autostart) do not restart, and a stopped rule is not started.",
  })
  @ApiParam({ name: "id", type: String, description: "The rule id." })
  @ApiBody({ type: UpdateForwardRuleBodyDto })
  @ApiOkResponse({ type: ForwardRuleResponseDto, description: "The updated rule with its port advisories." })
  @ApiBadRequestResponse({ type: ApiErrorResponseDto, description: "Invalid patch." })
  @ApiNotFoundResponse({ type: ApiErrorResponseDto, description: "No rule with that id." })
  @ApiConflictResponse({ type: ApiErrorResponseDto, description: "The update would duplicate another rule's binding." })
  async update(
    @Param("id") id: string,
    @Body() body: UpdateForwardRuleBodyDto
  ): Promise<ForwardRuleResponseDto> {
    return toForwardRuleResponseDto(await this.updateService.update(id, body));
  }

  @Delete(":id")
  @HttpCode(204)
  @ApiOperation({
    summary: "Delete a forward rule",
    description:
      "Deletes a rule. A running rule's forwarder is stopped first; an unknown id returns 404. Returns 204 " +
      "with no body (matching Express). The `:id` path param has no validation pipe (Express does none) — an " +
      "unknown id surfaces as the manager's NotFoundError → 404.",
  })
  @ApiParam({ name: "id", type: String, description: "The rule id." })
  @ApiNoContentResponse({ description: "Rule deleted. No response body." })
  @ApiNotFoundResponse({ type: ApiErrorResponseDto, description: "No rule with that id." })
  async remove(@Param("id") id: string): Promise<void> {
    await this.deleteService.delete(id);
  }

  @Post(":id/start")
  @HttpCode(200)
  @ApiOperation({
    summary: "Start a forward rule",
    description:
      "Starts the rule's forwarder and returns its current status. Idempotent — an already-running rule " +
      "returns its status without restarting; autostart/enabled is not a precondition. An unknown id returns 404. " +
      "Returns 200 (matching Express; NestJS would otherwise default POST to 201).",
  })
  @ApiParam({ name: "id", type: String, description: "The rule id." })
  @ApiOkResponse({ type: ForwardStatusDto, description: "The rule's status after starting." })
  @ApiNotFoundResponse({ type: ApiErrorResponseDto, description: "No rule with that id." })
  async start(@Param("id") id: string): Promise<ForwardStatusDto> {
    return toForwardStatusResponseDto(await this.startService.start(id));
  }
}
