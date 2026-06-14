import { Controller, Get, Inject } from "@nestjs/common";
import { ApiOkResponse, ApiOperation, ApiTags } from "@nestjs/swagger";
import { ConfigExportService } from "./config-export.service.js";
import {
  ConfigExportResponseDto,
  toConfigExportResponseDto,
} from "./config-export.response.dto.js";

/**
 * Transport adapter for `GET /api/config/export`. No request input (no request
 * DTO needed). Delegates to the service and maps the domain exported config to
 * `ConfigExportResponseDto` at the HTTP boundary (matching the existing Express
 * route). Read-only and always `200`; no export logic lives here.
 */
@ApiTags("config")
@Controller("api/config/export")
export class ConfigExportController {
  constructor(@Inject(ConfigExportService) private readonly configExport: ConfigExportService) {}

  @Get()
  @ApiOperation({ summary: "Export config", description: "Returns the current forward rules as an exportable config." })
  @ApiOkResponse({ type: ConfigExportResponseDto, description: "Exported config snapshot." })
  export(): ConfigExportResponseDto {
    return toConfigExportResponseDto(this.configExport.export());
  }
}
