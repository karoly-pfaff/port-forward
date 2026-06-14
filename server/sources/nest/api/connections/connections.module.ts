import { Module } from "@nestjs/common";
import { CLOCK_READER, defaultClockReader } from "../../common/clock.reader.js";
import { ConnectionsController } from "./connections.controller.js";
import { ConnectionsService } from "./connections.service.js";
import { CONNECTIONS_READER, emptyConnectionsReader } from "./connections.reader.js";

/**
 * `GET /api/connections` (v1.14 Slice 11) — the last volatile read endpoint
 * (`generatedAt`). The `CONNECTIONS_READER` token defaults to an empty reader
 * (the scaffold has no runtime wired); the `CLOCK_READER` defaults to the real
 * wall clock. Both are overridden in tests for byte-for-byte parity (`generatedAt`
 * pinned by a fixed clock, rules/connections supplied by a seeded fake — no
 * sockets). Read-only; no connection lifecycle/mutation.
 */
@Module({
  controllers: [ConnectionsController],
  providers: [
    ConnectionsService,
    { provide: CONNECTIONS_READER, useValue: emptyConnectionsReader },
    { provide: CLOCK_READER, useValue: defaultClockReader },
  ],
})
export class ConnectionsModule {}
