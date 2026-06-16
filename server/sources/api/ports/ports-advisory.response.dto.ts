import type { PortAdvisory } from "@portier/shared";

/**
 * Response DTO for `GET /api/ports/advisory` — the documented JSON array.
 *
 * The `@portier/shared` types are Portier's REST contract (guarded by
 * `validate:contract`), so the response DTO mirrors that shape exactly and the
 * mapper is a structural copy at the HTTP boundary (it would become an explicit
 * field pick only if a domain object ever carried fields that must not leak).
 */
export type PortsAdvisoryResponseDto = PortAdvisory[];

/** Maps the advisory list to the response DTO (a fresh array of fresh objects). */
export function toPortsAdvisoryResponseDto(advisories: PortAdvisory[]): PortsAdvisoryResponseDto {
  return advisories.map((advisory) => ({ ...advisory }));
}
