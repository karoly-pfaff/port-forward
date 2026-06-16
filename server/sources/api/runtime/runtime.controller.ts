import { Controller, Get, Inject } from "@nestjs/common";
import { ApiOkResponse, ApiOperation, ApiTags } from "@nestjs/swagger";
import { RuntimeService } from "./runtime.service.js";
import {
  RuntimeInfoResponseDto,
  toRuntimeInfoResponseDto,
} from "./runtime.response.dto.js";

/**
 * Transport adapter for `GET /api/runtime`. No request input (no request DTO
 * needed). Delegates to the service and maps the domain runtime info to
 * `RuntimeInfoResponseDto` at the HTTP boundary (matching the documented `/api` contract). Read-only and always `200`; no runtime logic lives here.
 */
@ApiTags("runtime")
@Controller("api/runtime")
export class RuntimeController {
  constructor(@Inject(RuntimeService) private readonly runtime: RuntimeService) {}

  @Get()
  @ApiOperation({ summary: "Runtime info", description: "Returns runtime/version/uptime/process metadata." })
  @ApiOkResponse({ type: RuntimeInfoResponseDto, description: "Runtime info." })
  get(): RuntimeInfoResponseDto {
    return toRuntimeInfoResponseDto(this.runtime.get());
  }
}
