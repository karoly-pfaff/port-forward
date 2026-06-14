import { Injectable } from "@nestjs/common";
import { getPortAdvisories, type PortAdvisory } from "@portier/shared";

/**
 * Result of resolving a port-advisory request: either the computed advisories,
 * or the contract error envelope's messages for a bad request. Keeping the
 * validation here (not in the controller) makes every branch unit-testable
 * without booting HTTP.
 */
export type AdvisoryResult =
  | { ok: true; advisories: PortAdvisory[] }
  | { ok: false; errors: string[] };

/**
 * Behaviour for `GET /api/ports/advisory`. Validates the raw query inputs with
 * the SAME rules as the existing Express route (port is an integer 1–65535;
 * purpose is `management` or `forward`; `listenHost` is an optional string) and
 * delegates the advisory computation to the shared `getPortAdvisories` so the
 * two runtimes cannot drift.
 */
@Injectable()
export class PortsService {
  resolveAdvisories(rawPort: unknown, rawPurpose: unknown, rawListenHost: unknown): AdvisoryResult {
    const port = Number(rawPort);
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      return { ok: false, errors: ["port must be an integer from 1 to 65535."] };
    }
    if (rawPurpose !== "management" && rawPurpose !== "forward") {
      return { ok: false, errors: ["purpose must be management or forward."] };
    }
    const listenHost = typeof rawListenHost === "string" ? rawListenHost : undefined;
    return { ok: true, advisories: getPortAdvisories({ port, listenHost, purpose: rawPurpose }) };
  }
}
