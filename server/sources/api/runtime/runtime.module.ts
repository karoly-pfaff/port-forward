import { Module } from "@nestjs/common";
import { APP_RUNTIME, type AppRuntime } from "../../app/runtime-context.js";
import { RuntimeController } from "./runtime.controller.js";
import { RuntimeService } from "./runtime.service.js";
import {
  CLOCK_READER,
  createDefaultRuntimeInfoReader,
  defaultClockReader,
  defaultProcessReader,
  PROCESS_READER,
  RUNTIME_INFO_READER,
  type RuntimeInfoReader,
} from "./runtime.reader.js";

/**
 * `GET /api/runtime` — the first migrated endpoint with volatile fields
 * (`uptimeSeconds`, process `pid`/`platform`/`arch`). The volatile/process/info
 * values come from narrow injected readers (clock, process, runtime-info) that
 * default to the real clock/process and a fixed-at-construction start time;
 * tests override them with fixed values for exact parity. The endpoint shares
 * the `buildRuntimeInfo` builder, the single source of the runtime-info shape.
 */
@Module({
  controllers: [RuntimeController],
  providers: [
    RuntimeService,
    { provide: CLOCK_READER, useValue: defaultClockReader },
    { provide: PROCESS_READER, useValue: defaultProcessReader },
    {
      provide: RUNTIME_INFO_READER,
      useFactory: (rt: AppRuntime | null): RuntimeInfoReader =>
        rt?.runtimeInfoReader ?? createDefaultRuntimeInfoReader(),
      inject: [APP_RUNTIME],
    },
  ],
})
export class RuntimeModule {}
