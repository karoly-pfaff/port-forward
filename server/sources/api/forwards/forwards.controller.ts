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
import { ApiErrorResponseDto } from "../common/api-error.schema.js";
import { CreateForwardRuleBodyDto, UpdateForwardRuleBodyDto } from "./forward-rule.body.schema.js";
import { ForwardRuleResponseDto } from "./forward-rule.schema.js";
import { ForwardStatusDto } from "./forward-status.schema.js";
import { GroupActionResponseDto } from "./group-action.schema.js";
import { RuleDiagnosticsResultDto } from "./rule-diagnostics.schema.js";
import { ApiValidationPipe } from "../common/api-validation.pipe.js";
import { CreateForwardRuleService } from "./create-forward-rule.service.js";
import { DeleteForwardRuleService } from "./delete-forward-rule.service.js";
import { DiagnoseForwardRuleService } from "./diagnose-forward-rule.service.js";
import { ReorderForwardRulesService } from "./reorder-forward-rules.service.js";
import { StartForwardGroupService } from "./start-forward-group.service.js";
import { StartForwardRuleService } from "./start-forward-rule.service.js";
import { StopForwardGroupService } from "./stop-forward-group.service.js";
import { StopForwardRuleService } from "./stop-forward-rule.service.js";
import { UpdateForwardRuleService } from "./update-forward-rule.service.js";
import { ReorderForwardRulesBodyDto } from "./reorder-forward-rules.body.dto.js";
import { toForwardRuleResponseDto } from "./forward-rule.response.dto.js";
import { toForwardStatusResponseDto } from "./forward-status.response.dto.js";
import { toGroupActionResponseDto } from "./group-action.response.dto.js";
import { toRuleDiagnosticsResponseDto } from "./rule-diagnostics.response.dto.js";
import { ForwardsService } from "./forwards.service.js";
import { toForwardsListResponseDto, type ForwardsListResponseDto } from "./forwards-list.response.dto.js";

/**
 * Transport adapter for `/api/forwards`. `GET` lists rules (read); `POST` creates
 * a rule (write — the first migrated rule mutation). Both map service results to
 * their response DTO at the HTTP boundary (matching the documented `/api` contract). The
 * `POST` body is NOT run through a validation pipe — validation is delegated to
 * the shared `validateForwardRule` inside the manager (a documented parity
 * exception), so `@ApiBody` documents the schema explicitly. The reorder body
 * (`ReorderForwardRulesBodyDto`) IS a real validated DTO (its `ids: string[]`
 * check is simple enough to re-express exactly in class-validator), run through
 * `ApiValidationPipe`. List/create/update/delete, the lifecycle start + stop
 * (`POST :id/start`, `POST :id/stop`), reorder (`POST reorder`), the group actions, and diagnose.
 */
@ApiTags("forwards")
@Controller("api/forwards")
export class ForwardsController {
  constructor(
    @Inject(ForwardsService) private readonly forwards: ForwardsService,
    @Inject(CreateForwardRuleService) private readonly createService: CreateForwardRuleService,
    @Inject(UpdateForwardRuleService) private readonly updateService: UpdateForwardRuleService,
    @Inject(DeleteForwardRuleService) private readonly deleteService: DeleteForwardRuleService,
    @Inject(StartForwardRuleService) private readonly startService: StartForwardRuleService,
    @Inject(StopForwardRuleService) private readonly stopService: StopForwardRuleService,
    @Inject(ReorderForwardRulesService) private readonly reorderService: ReorderForwardRulesService,
    @Inject(DiagnoseForwardRuleService) private readonly diagnoseService: DiagnoseForwardRuleService,
    @Inject(StopForwardGroupService) private readonly stopGroupService: StopForwardGroupService,
    @Inject(StartForwardGroupService) private readonly startGroupService: StartForwardGroupService
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
      "Creates a forward rule. A created rule with autostart enabled starts its forwarder.",
  })
  @ApiBody({ type: CreateForwardRuleBodyDto })
  @ApiCreatedResponse({ type: ForwardRuleResponseDto, description: "The created rule with its port advisories." })
  @ApiBadRequestResponse({ type: ApiErrorResponseDto, description: "Invalid rule definition." })
  @ApiConflictResponse({ type: ApiErrorResponseDto, description: "A rule already listens on that binding." })
  async create(@Body() body: CreateForwardRuleBodyDto): Promise<ForwardRuleResponseDto> {
    return toForwardRuleResponseDto(await this.createService.create(body));
  }

  @Post("reorder")
  @HttpCode(200)
  @ApiOperation({
    summary: "Reorder forward rules",
    description:
      "Reorders the rules to the given id order; any rule id not listed keeps its relative order at the end " +
      "(a partial set is allowed, a duplicate id is tolerated, an empty list is a no-op). An unknown id returns " +
      "404 and no reorder is persisted. Returns 200 with the full reordered rule list (NestJS " +
      "would otherwise default POST to 201). Reorder is metadata only — running forwarders are not affected.",
  })
  @ApiBody({ type: ReorderForwardRulesBodyDto })
  @ApiOkResponse({ type: ForwardRuleResponseDto, isArray: true, description: "The full reordered rule list." })
  @ApiBadRequestResponse({ type: ApiErrorResponseDto, description: "`ids` is not an array of strings." })
  @ApiNotFoundResponse({ type: ApiErrorResponseDto, description: "An id in the list does not match any rule." })
  async reorder(
    @Body(new ApiValidationPipe(ReorderForwardRulesBodyDto)) body: ReorderForwardRulesBodyDto
  ): Promise<ForwardsListResponseDto> {
    return toForwardsListResponseDto(await this.reorderService.reorder(body.ids));
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
      "with no body. The `:id` path param has no validation — an " +
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
      "Returns 200 (NestJS would otherwise default POST to 201).",
  })
  @ApiParam({ name: "id", type: String, description: "The rule id." })
  @ApiOkResponse({ type: ForwardStatusDto, description: "The rule's status after starting." })
  @ApiNotFoundResponse({ type: ApiErrorResponseDto, description: "No rule with that id." })
  async start(@Param("id") id: string): Promise<ForwardStatusDto> {
    return toForwardStatusResponseDto(await this.startService.start(id));
  }

  @Post(":id/stop")
  @HttpCode(200)
  @ApiOperation({
    summary: "Stop a forward rule",
    description:
      "Stops the rule's forwarder and returns its current status. Idempotent — a rule that is not running " +
      "returns its status without touching a socket. An unknown id returns 404. Returns 200 (" +
      "NestJS would otherwise default POST to 201).",
  })
  @ApiParam({ name: "id", type: String, description: "The rule id." })
  @ApiOkResponse({ type: ForwardStatusDto, description: "The rule's status after stopping." })
  @ApiNotFoundResponse({ type: ApiErrorResponseDto, description: "No rule with that id." })
  async stop(@Param("id") id: string): Promise<ForwardStatusDto> {
    return toForwardStatusResponseDto(await this.stopService.stop(id));
  }

  @Post(":id/diagnose")
  @HttpCode(200)
  @ApiOperation({
    summary: "Diagnose a forward rule",
    description:
      "Runs read-only diagnostics for the rule — listen-host/LAN-exposure/privileged-port/common-port advisories, " +
      "a listen-bind probe (skipped while the rule is running, since Portier already owns the port), target-host " +
      "DNS resolution, a TCP target-connect probe (skipped for UDP / unresolved targets), and the UDP mode (UDP " +
      "only) — and returns the ordered checks plus an overall summary. Does not mutate the rule. An unknown id " +
      "returns 404. Returns 200 (NestJS would otherwise default POST to 201).",
  })
  @ApiParam({ name: "id", type: String, description: "The rule id." })
  @ApiOkResponse({ type: RuleDiagnosticsResultDto, description: "The diagnostic checks and summary for the rule." })
  @ApiNotFoundResponse({ type: ApiErrorResponseDto, description: "No rule with that id." })
  async diagnose(@Param("id") id: string): Promise<RuleDiagnosticsResultDto> {
    return toRuleDiagnosticsResponseDto(await this.diagnoseService.diagnose(id));
  }

  @Post("groups/:group/stop")
  @HttpCode(200)
  @ApiOperation({
    summary: "Stop a group of forward rules",
    description:
      "Stops every rule sharing the given group label, in rule order. Behaviour over existing rule metadata — " +
      "never mutates rule definitions, order, autostart, or group. A rule that is not running is skipped " +
      "(not_running, no socket touched); the response summarizes per-rule results. An empty/invalid group name " +
      "returns 400; a group with no matching rules returns 404. Returns 200 (NestJS would " +
      "otherwise default POST to 201).",
  })
  @ApiParam({ name: "group", type: String, description: "The group label (URL-encoded)." })
  @ApiOkResponse({ type: GroupActionResponseDto, description: "Per-rule results + counts for the group stop." })
  @ApiBadRequestResponse({ type: ApiErrorResponseDto, description: "The group name is empty or invalid." })
  @ApiNotFoundResponse({ type: ApiErrorResponseDto, description: "No rules belong to that group." })
  async stopGroup(@Param("group") group: string): Promise<GroupActionResponseDto> {
    return toGroupActionResponseDto(await this.stopGroupService.stop(group));
  }

  @Post("groups/:group/start")
  @HttpCode(200)
  @ApiOperation({
    summary: "Start a group of forward rules",
    description:
      "Starts every rule sharing the given group label, in rule order. Behaviour over existing rule metadata — " +
      "never mutates rule definitions, order, autostart, or group; autostart/enabled is not a precondition. A rule " +
      "that is already running is skipped (already_running, no new socket); the response summarizes per-rule " +
      "results. An empty/invalid group name returns 400; a group with no matching rules returns 404. Returns 200 " +
      "(NestJS would otherwise default POST to 201).",
  })
  @ApiParam({ name: "group", type: String, description: "The group label (URL-encoded)." })
  @ApiOkResponse({ type: GroupActionResponseDto, description: "Per-rule results + counts for the group start." })
  @ApiBadRequestResponse({ type: ApiErrorResponseDto, description: "The group name is empty or invalid." })
  @ApiNotFoundResponse({ type: ApiErrorResponseDto, description: "No rules belong to that group." })
  async startGroup(@Param("group") group: string): Promise<GroupActionResponseDto> {
    return toGroupActionResponseDto(await this.startGroupService.start(group));
  }
}
