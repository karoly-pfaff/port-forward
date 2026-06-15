import { Module } from "@nestjs/common";
import { CLOCK_READER, defaultClockReader } from "../../common/clock.reader.js";
import { ConfigExportController } from "./config-export.controller.js";
import { ConfigExportService } from "./config-export.service.js";
import { CONFIG_EXPORT_READER, emptyConfigExportReader } from "./config-export.reader.js";
import { ConfigPlanController } from "./config-plan.controller.js";
import { ConfigPlanService } from "./config-plan.service.js";
import { CONFIG_PLAN_READER, emptyConfigPlanReader } from "./config-plan.reader.js";

/**
 * Config API feature. Exposes the read-only `GET /api/config/export` (Slice 10) and
 * the NON-MUTATING `POST /api/config/plan` dry-run (Slice 23) — the mutating config
 * import/apply endpoints stay with Express, deferred. The `CONFIG_EXPORT_READER`/
 * `CONFIG_PLAN_READER` tokens default to empty readers (the scaffold has no runtime
 * wired); the `CLOCK_READER` defaults to the real wall clock (it stamps both the
 * export `exportedAt` and the plan `generatedAt`). All are overridden in tests for
 * byte-for-byte parity (the timestamp pinned by a fixed clock, rules supplied by a
 * seeded manager).
 */
@Module({
  controllers: [ConfigExportController, ConfigPlanController],
  providers: [
    ConfigExportService,
    ConfigPlanService,
    { provide: CONFIG_EXPORT_READER, useValue: emptyConfigExportReader },
    { provide: CONFIG_PLAN_READER, useValue: emptyConfigPlanReader },
    { provide: CLOCK_READER, useValue: defaultClockReader },
  ],
})
export class ConfigModule {}
