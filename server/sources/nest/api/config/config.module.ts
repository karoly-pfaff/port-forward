import { Module } from "@nestjs/common";
import { CLOCK_READER, defaultClockReader } from "../../common/clock.reader.js";
import { ConfigExportController } from "./config-export.controller.js";
import { ConfigExportService } from "./config-export.service.js";
import { CONFIG_EXPORT_READER, emptyConfigExportReader } from "./config-export.reader.js";

/**
 * Config API feature. Currently exposes only the read-only `GET /api/config/export`
 * (Slice 10) — config import/write endpoints stay with Express, deferred. The
 * `CONFIG_EXPORT_READER` token defaults to an empty reader (the scaffold has no
 * runtime wired); the `CLOCK_READER` defaults to the real wall clock. Both are
 * overridden in tests for byte-for-byte parity (`exportedAt` pinned by a fixed
 * clock, rules supplied by a seeded manager).
 */
@Module({
  controllers: [ConfigExportController],
  providers: [
    ConfigExportService,
    { provide: CONFIG_EXPORT_READER, useValue: emptyConfigExportReader },
    { provide: CLOCK_READER, useValue: defaultClockReader },
  ],
})
export class ConfigModule {}
