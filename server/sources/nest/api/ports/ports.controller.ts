import { Controller, Get, Inject, Query } from "@nestjs/common";
import { ApiValidationPipe } from "../../common/api-validation.pipe.js";
import { PortsAdvisoryQueryDto } from "./ports-advisory.query.dto.js";
import { toPortsAdvisoryResponseDto, type PortsAdvisoryResponseDto } from "./ports-advisory.response.dto.js";
import { PortsService } from "./ports.service.js";

/**
 * Transport adapter for `GET /api/ports/advisory`. Request: `PortsAdvisoryQueryDto`
 * validated by `ApiValidationPipe` (`400 { errors }` on failure). Response:
 * mapped to `PortsAdvisoryResponseDto` at the HTTP boundary. No validation or
 * envelope logic lives here.
 */
@Controller("api/ports")
export class PortsController {
  constructor(@Inject(PortsService) private readonly ports: PortsService) {}

  @Get("advisory")
  getAdvisory(
    @Query(new ApiValidationPipe(PortsAdvisoryQueryDto)) query: PortsAdvisoryQueryDto
  ): PortsAdvisoryResponseDto {
    return toPortsAdvisoryResponseDto(this.ports.getAdvisories(query));
  }
}
