import { Controller, Get, Inject } from "@nestjs/common";
import { ApiOkResponse, ApiOperation, ApiTags } from "@nestjs/swagger";
import { ConnectionsService } from "./connections.service.js";
import {
  ConnectionsResponseDto,
  toConnectionsResponseDto,
} from "./connections.response.dto.js";

/**
 * Transport adapter for `GET /api/connections`. No request input (no request DTO
 * needed). Delegates to the service and maps the domain snapshot to
 * `ConnectionsResponseDto` at the HTTP boundary (matching the documented `/api` contract). Read-only and always `200`; no connection logic lives here.
 */
@ApiTags("connections")
@Controller("api/connections")
export class ConnectionsController {
  constructor(@Inject(ConnectionsService) private readonly connections: ConnectionsService) {}

  @Get()
  @ApiOperation({ summary: "Live connections", description: "Returns live TCP connections, UDP sessions, and per-rule summaries." })
  @ApiOkResponse({ type: ConnectionsResponseDto, description: "Live connection snapshot." })
  get(): ConnectionsResponseDto {
    return toConnectionsResponseDto(this.connections.get());
  }
}
