import { Body, Controller, Inject, Post, Res } from "@nestjs/common";
import {
  ApiBadRequestResponse,
  ApiBody,
  ApiOkResponse,
  ApiOperation,
  ApiResponse,
  ApiTags,
} from "@nestjs/swagger";
import type { Response } from "express";
import { ApiErrorResponseDto } from "../../common/api-error.schema.js";
import { ConfigImportBodyDto } from "./config-import.schema.js";
import { ConfigImportService } from "./config-import.service.js";
import {
  ConfigImportErrorResponseDto,
  ConfigImportResponseDto,
  toConfigImportErrorResponseDto,
  toConfigImportResponseDto,
} from "./config-import.response.dto.js";

/**
 * Transport adapter for `POST /api/config/import` — the MUTATING config import.
 * Validation (mode/config) is delegated to the service to match Express's
 * short-circuit `400` order exactly (so `ConfigImportBodyDto` is documentation/
 * typing only — no validation pipe). The success (`200 {result, rules}`) and the
 * import-error (`422 {errors, result}`) responses are RETURNED with their status
 * via `@Res({ passthrough: true })` (matching Express's `response.status(...).json`)
 * — the `422` body carries `result` alongside `errors`, so it is NOT the plain
 * `{errors}` envelope and must not flow through the error filter. The two `400`s
 * are thrown as `ApiBadRequestException` and DO flow through the shared filter.
 */
@ApiTags("config")
@Controller("api/config")
export class ConfigImportController {
  constructor(@Inject(ConfigImportService) private readonly configImport: ConfigImportService) {}

  @Post("import")
  @ApiOperation({
    summary: "Import a config",
    description:
      "Imports a Portier config in `replace` (replace all rules) or `merge` (add non-conflicting rules) mode. " +
      "Returns 200 with the import result + the full rule list on success, or 422 with the errors + result when " +
      "the import reports validation/duplicate-binding/merge-conflict errors (rejected before any mutation). An " +
      "invalid mode or config returns 400. Unlike `plan` (a dry-run), import MUTATES the rules; enabled imported " +
      "rules start their forwarders.",
  })
  @ApiBody({ type: ConfigImportBodyDto })
  @ApiOkResponse({ type: ConfigImportResponseDto, description: "Import succeeded; the full rule list is returned." })
  @ApiResponse({ status: 422, type: ConfigImportErrorResponseDto, description: "The import reported errors (no mutation)." })
  @ApiBadRequestResponse({ type: ApiErrorResponseDto, description: "Invalid mode or config." })
  async import(
    @Body() body: unknown,
    @Res({ passthrough: true }) res: Response
  ): Promise<ConfigImportResponseDto | ConfigImportErrorResponseDto> {
    const outcome = await this.configImport.import(body);
    res.status(outcome.status);
    return outcome.status === 200
      ? toConfigImportResponseDto(outcome.body)
      : toConfigImportErrorResponseDto(outcome.body);
  }
}
