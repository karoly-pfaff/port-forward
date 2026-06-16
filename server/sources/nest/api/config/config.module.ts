import { Module } from "@nestjs/common";
import { CLOCK_READER, defaultClockReader } from "../../common/clock.reader.js";
import { APP_RUNTIME, type AppRuntime } from "../../common/runtime-context.js";
import { ConfigApplyController } from "./config-apply.controller.js";
import { ConfigApplyService } from "./config-apply.service.js";
import { CONFIG_APPLIER, createDefaultConfigApplier, type ConfigApplier } from "./config-apply.writer.js";
import { ConfigExportController } from "./config-export.controller.js";
import { ConfigExportService } from "./config-export.service.js";
import { CONFIG_EXPORT_READER, emptyConfigExportReader, type ConfigExportReader } from "./config-export.reader.js";
import { ConfigImportController } from "./config-import.controller.js";
import { ConfigImportService } from "./config-import.service.js";
import { CONFIG_IMPORTER, createDefaultConfigImporter, type ConfigImporter } from "./config-import.writer.js";
import { ConfigPlanController } from "./config-plan.controller.js";
import { ConfigPlanService } from "./config-plan.service.js";
import { CONFIG_PLAN_READER, emptyConfigPlanReader, type ConfigPlanReader } from "./config-plan.reader.js";

/**
 * Config API feature. Exposes the read-only `GET /api/config/export`, the
 * non-mutating `POST /api/config/plan` dry-run, and the mutating
 * `POST /api/config/import` and `POST /api/config/apply`. The
 * `CONFIG_EXPORT_READER`/`CONFIG_PLAN_READER` tokens default to empty readers and
 * `CONFIG_IMPORTER`/`CONFIG_APPLIER` to fresh isolated in-memory managers (no live
 * forwarding runtime is wired into the NestJS server); the `CLOCK_READER` defaults
 * to the real wall clock (it stamps the export `exportedAt`, the plan `generatedAt`,
 * and the apply `appliedAt`). All are overridden in tests for byte-for-byte parity
 * (the timestamp pinned by a fixed clock, rules supplied/imported/applied by a
 * seeded manager).
 */
@Module({
  controllers: [ConfigExportController, ConfigPlanController, ConfigImportController, ConfigApplyController],
  providers: [
    ConfigExportService,
    ConfigPlanService,
    ConfigImportService,
    ConfigApplyService,
    {
      provide: CONFIG_EXPORT_READER,
      useFactory: (rt: AppRuntime | null): ConfigExportReader => rt?.manager ?? emptyConfigExportReader,
      inject: [APP_RUNTIME],
    },
    {
      provide: CONFIG_PLAN_READER,
      useFactory: (rt: AppRuntime | null): ConfigPlanReader => rt?.manager ?? emptyConfigPlanReader,
      inject: [APP_RUNTIME],
    },
    {
      provide: CONFIG_IMPORTER,
      useFactory: (rt: AppRuntime | null): ConfigImporter => rt?.manager ?? createDefaultConfigImporter(),
      inject: [APP_RUNTIME],
    },
    {
      provide: CONFIG_APPLIER,
      useFactory: (rt: AppRuntime | null): ConfigApplier => rt?.manager ?? createDefaultConfigApplier(),
      inject: [APP_RUNTIME],
    },
    { provide: CLOCK_READER, useValue: defaultClockReader },
  ],
})
export class ConfigModule {}
