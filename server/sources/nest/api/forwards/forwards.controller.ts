import { Body, Controller, Get, Inject, Post } from "@nestjs/common";
import {
  ApiBadRequestResponse,
  ApiBody,
  ApiConflictResponse,
  ApiCreatedResponse,
  ApiOkResponse,
  ApiOperation,
  ApiTags,
} from "@nestjs/swagger";
import { ApiErrorResponseDto, CreateForwardRuleBodyDto, ForwardRuleResponseDto } from "../../common/api-schemas.js";
import { CreateForwardRuleService } from "./create-forward-rule.service.js";
import { toForwardRuleResponseDto } from "./forward-rule.response.dto.js";
import { ForwardsService } from "./forwards.service.js";
import { toForwardsListResponseDto, type ForwardsListResponseDto } from "./forwards-list.response.dto.js";

/**
 * Transport adapter for `/api/forwards`. `GET` lists rules (read); `POST` creates
 * a rule (write — the first migrated rule mutation). Both map service results to
 * their response DTO at the HTTP boundary (matching the Express routes). The
 * `POST` body is NOT run through a validation pipe — validation is delegated to
 * the shared `validateForwardRule` inside the manager (a documented parity
 * exception), so `@ApiBody` documents the schema explicitly. Only list + create
 * are migrated; the other write/lifecycle routes under `/api/forwards/...` stay
 * with Express.
 */
@ApiTags("forwards")
@Controller("api/forwards")
export class ForwardsController {
  constructor(
    @Inject(ForwardsService) private readonly forwards: ForwardsService,
    @Inject(CreateForwardRuleService) private readonly createService: CreateForwardRuleService
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
}
