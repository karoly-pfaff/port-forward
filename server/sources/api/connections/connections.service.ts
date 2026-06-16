import { Inject, Injectable } from "@nestjs/common";
import type { LiveConnectionsResponse } from "@portier/shared";
import { buildLiveConnections } from "../../connections/connections-snapshot.js";
import { CLOCK_READER, type ClockReader } from "../common/clock.reader.js";
import {
  CONNECTIONS_READER,
  type ConnectionsReader,
} from "./connections.reader.js";

/**
 * Behaviour for `GET /api/connections`: snapshots the current rules + live TCP/UDP
 * records from the injected reader and stamps `generatedAt` from the injected
 * clock, via the shared `buildLiveConnections` builder (the same builder the
 * Express route uses, so the two cannot drift). Read-only and pure; never throws.
 */
@Injectable()
export class ConnectionsService {
  constructor(
    @Inject(CONNECTIONS_READER) private readonly reader: ConnectionsReader,
    @Inject(CLOCK_READER) private readonly clock: ClockReader
  ) {}

  get(): LiveConnectionsResponse {
    return buildLiveConnections({
      rules: this.reader.listRules(),
      tcpConnections: this.reader.getLiveTcpConnections(),
      udpSessions: this.reader.getLiveUdpSessions(),
      now: this.clock.now(),
    });
  }
}
