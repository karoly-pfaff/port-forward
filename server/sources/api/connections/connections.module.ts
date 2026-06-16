import { Module } from "@nestjs/common";
import { CLOCK_READER, defaultClockReader } from "../../common/clock.reader.js";
import { APP_RUNTIME, type AppRuntime } from "../../common/runtime-context.js";
import { ConnectionsController } from "./connections.controller.js";
import { ConnectionsService } from "./connections.service.js";
import { CONNECTIONS_READER, emptyConnectionsReader, type ConnectionsReader } from "./connections.reader.js";

/**
 * `GET /api/connections` — a volatile read endpoint (`generatedAt`).
 * `CONNECTIONS_READER` resolves to the live `ForwardManager` when one is wired, else
 * an empty reader; the `CLOCK_READER` is the real wall clock. Both are overridden in
 * tests for byte-for-byte parity (`generatedAt` pinned by a fixed clock,
 * rules/connections supplied by a seeded fake — no sockets). Read-only.
 */
@Module({
  controllers: [ConnectionsController],
  providers: [
    ConnectionsService,
    {
      provide: CONNECTIONS_READER,
      useFactory: (rt: AppRuntime | null): ConnectionsReader => rt?.manager ?? emptyConnectionsReader,
      inject: [APP_RUNTIME],
    },
    { provide: CLOCK_READER, useValue: defaultClockReader },
  ],
})
export class ConnectionsModule {}
