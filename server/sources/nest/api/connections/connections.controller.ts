import { Controller, Get, Inject } from "@nestjs/common";
import { ConnectionsService } from "./connections.service.js";
import {
  toConnectionsResponseDto,
  type ConnectionsResponseDto,
} from "./connections.response.dto.js";

/**
 * Transport adapter for `GET /api/connections`. No request input (no request DTO
 * needed). Delegates to the service and maps the domain snapshot to
 * `ConnectionsResponseDto` at the HTTP boundary (matching the existing Express
 * route). Read-only and always `200`; no connection logic lives here.
 */
@Controller("api/connections")
export class ConnectionsController {
  constructor(@Inject(ConnectionsService) private readonly connections: ConnectionsService) {}

  @Get()
  get(): ConnectionsResponseDto {
    return toConnectionsResponseDto(this.connections.get());
  }
}
