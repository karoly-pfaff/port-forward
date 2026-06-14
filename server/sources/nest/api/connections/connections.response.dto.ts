import type { LiveConnectionsResponse } from "@portier/shared";

/**
 * Response DTO for `GET /api/connections` — the `LiveConnectionsResponse` object
 * Express returns (the shared REST-contract shape). The mapper is a structural
 * copy at the HTTP boundary: a fresh object preserving the
 * `generatedAt`/`tcpConnections`/`udpSessions`/`ruleSummaries` field order, with
 * each nested record freshly copied (it would become an explicit field pick only
 * to hide a future internal-only field).
 */
export type ConnectionsResponseDto = LiveConnectionsResponse;

/** Maps the live-connections snapshot to the response DTO (fresh object + fresh arrays). */
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
