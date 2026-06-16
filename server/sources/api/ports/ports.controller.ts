import { Controller, Get, Inject, Query } from "@nestjs/common";
import { ApiBadRequestResponse, ApiOkResponse, ApiOperation, ApiQuery, ApiTags } from "@nestjs/swagger";
import { ApiErrorResponseDto } from "../../common/api-error.schema.js";
import { PortAdvisoryDto } from "./port-advisory.schema.js";
import { ApiValidationPipe } from "../../common/api-validation.pipe.js";
import { PortsAdvisoryQueryDto } from "./ports-advisory.query.dto.js";
import { toPortsAdvisoryResponseDto, type PortsAdvisoryResponseDto } from "./ports-advisory.response.dto.js";
import { PortsService } from "./ports.service.js";

/**
 * Transport adapter for `GET /api/ports/advisory`. Request: `PortsAdvisoryQueryDto`
 * validated by `ApiValidationPipe` (`400 { errors }` on failure). Response:
 * mapped to `PortsAdvisoryResponseDto` at the HTTP boundary. No validation or
 * envelope logic lives here. The `@ApiQuery` annotations document the params
 * explicitly because the esbuild/tsx generation transform does not emit the
 * `@Query` DTO's reflected type.
 */
@ApiTags("ports")
@Controller("api/ports")
export class PortsController {
  constructor(@Inject(PortsService) private readonly ports: PortsService) {}

  @Get("advisory")
  @ApiOperation({
    summary: "Port advisories for a listen binding",
    description: "Returns the port advisories (LAN exposure, privileged/common port, …) for a port.",
  })
  @ApiQuery({ name: "port", required: true, type: Number, example: 8080, description: "Port (1–65535)." })
  @ApiQuery({ name: "purpose", required: true, enum: ["management", "forward"], description: "Binding purpose." })
  @ApiQuery({ name: "listenHost", required: false, type: String, description: "Listen host (e.g. 0.0.0.0)." })
  @ApiOkResponse({ type: PortAdvisoryDto, isArray: true, description: "Advisories for the port." })
  @ApiBadRequestResponse({ type: ApiErrorResponseDto, description: "Invalid query parameters." })
  getAdvisory(
    @Query(new ApiValidationPipe(PortsAdvisoryQueryDto)) query: PortsAdvisoryQueryDto
  ): PortsAdvisoryResponseDto {
    return toPortsAdvisoryResponseDto(this.ports.getAdvisories(query));
  }
}
