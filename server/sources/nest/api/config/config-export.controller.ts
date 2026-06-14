import { Controller, Get, Inject } from "@nestjs/common";
import { ConfigExportService } from "./config-export.service.js";
import {
  toConfigExportResponseDto,
  type ConfigExportResponseDto,
} from "./config-export.response.dto.js";

/**
 * Transport adapter for `GET /api/config/export`. No request input (no request
 * DTO needed). Delegates to the service and maps the domain exported config to
 * `ConfigExportResponseDto` at the HTTP boundary (matching the existing Express
 * route). Read-only and always `200`; no export logic lives here.
 */
@Controller("api/config/export")
export class ConfigExportController {
  constructor(@Inject(ConfigExportService) private readonly configExport: ConfigExportService) {}

  @Get()
  export(): ConfigExportResponseDto {
    return toConfigExportResponseDto(this.configExport.export());
  }
}
