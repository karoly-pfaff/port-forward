import type { LiveConnectionsResponse } from "@portier/shared";
import { ConnectionsResponseDto } from "../../common/api-schemas.js";

export { ConnectionsResponseDto } from "../../common/api-schemas.js";

/**
 * Maps the domain live-connections snapshot to the `ConnectionsResponseDto` shape
 * at the HTTP boundary — a fresh object + fresh arrays of fresh records (the DTO
 * class is the OpenAPI schema, defined in `common/api-schemas.ts`; this mapper is
 * the covered logic).
 */
export function toConnectionsResponseDto(
  snapshot: LiveConnectionsResponse
): ConnectionsResponseDto {
  return {
    generatedAt: snapshot.generatedAt,
    tcpConnections: snapshot.tcpConnections.map((connection) => ({ ...connection })),
    udpSessions: snapshot.udpSessions.map((session) => ({ ...session })),
    ruleSummaries: snapshot.ruleSummaries.map((summary) => ({ ...summary })),
  };
}
