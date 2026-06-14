import { Module } from "@nestjs/common";
import { RuntimeController } from "./runtime.controller.js";
import { RuntimeService } from "./runtime.service.js";
import {
  CLOCK_READER,
  createDefaultRuntimeInfoReader,
  defaultClockReader,
  defaultProcessReader,
  PROCESS_READER,
  RUNTIME_INFO_READER,
} from "./runtime.reader.js";

/**
 * `GET /api/runtime` — the first migrated endpoint with volatile fields
 * (`uptimeSeconds`, process `pid`/`platform`/`arch`). The volatile/process/info
 * values come from narrow injected readers (clock, process, runtime-info) that
 * default to the real clock/process and a fixed-at-construction start time;
 * tests override them with fixed values for exact parity. The endpoint shares
 * the `buildRuntimeInfo` builder with the Express route, so the two cannot drift.
 */
@Module({
  controllers: [RuntimeController],
  providers: [
    RuntimeService,
    { provide: CLOCK_READER, useValue: defaultClockReader },
    { provide: PROCESS_READER, useValue: defaultProcessReader },
    { provide: RUNTIME_INFO_READER, useFactory: createDefaultRuntimeInfoReader },
  ],
})
export class RuntimeModule {}
