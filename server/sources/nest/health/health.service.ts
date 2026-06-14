import { Injectable } from "@nestjs/common";

/**
 * Shape of the NestJS scaffold liveness probe.
 *
 * `server: "node"` mirrors the documented Go `GET /api/health` shape
 * (`{ ok, server, name }`) so a future migration can align them deliberately.
 * The scaffold serves this at `GET /health` (outside the frozen `/api`
 * contract) — see the durable rules in CLAUDE.md.
 */
export interface HealthStatus {
  ok: boolean;
  server: "node";
  name: string;
}

/**
 * Application-layer health logic. Holds no transport concerns and needs no
 * runtime manager — a probe must answer even when no forwarders are loaded.
 */
@Injectable()
export class HealthService {
  getHealth(): HealthStatus {
    return { ok: true, server: "node", name: "Portier" };
  }
}
