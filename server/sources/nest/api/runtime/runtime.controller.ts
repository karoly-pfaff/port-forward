import { Controller, Get, Inject } from "@nestjs/common";
import { RuntimeService } from "./runtime.service.js";
import {
  toRuntimeInfoResponseDto,
  type RuntimeInfoResponseDto,
} from "./runtime.response.dto.js";

/**
 * Transport adapter for `GET /api/runtime`. No request input (no request DTO
 * needed). Delegates to the service and maps the domain runtime info to
 * `RuntimeInfoResponseDto` at the HTTP boundary (matching the existing Express
 * route). Read-only and always `200`; no runtime logic lives here.
 */
@Controller("api/runtime")
export class RuntimeController {
  constructor(@Inject(RuntimeService) private readonly runtime: RuntimeService) {}

  @Get()
  get(): RuntimeInfoResponseDto {
    return toRuntimeInfoResponseDto(this.runtime.get());
  }
}
