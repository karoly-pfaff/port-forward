import { Inject, Injectable } from "@nestjs/common";
import type { RuntimeInfo } from "@portier/shared";
import { buildRuntimeInfo } from "../../../runtime-info.js";
import {
  CLOCK_READER,
  PROCESS_READER,
  RUNTIME_INFO_READER,
  type ClockReader,
  type ProcessReader,
  type RuntimeInfoReader,
} from "./runtime.reader.js";

/**
 * Behaviour for `GET /api/runtime`: composes the injected volatile/process/info
 * readers and delegates to the shared `buildRuntimeInfo` builder — the exact
 * same builder the Express route uses, so the two runtimes cannot drift. Read-
 * only; never throws.
 */
@Injectable()
export class RuntimeService {
  constructor(
    @Inject(CLOCK_READER) private readonly clock: ClockReader,
    @Inject(PROCESS_READER) private readonly process: ProcessReader,
    @Inject(RUNTIME_INFO_READER) private readonly runtimeInfo: RuntimeInfoReader
  ) {}

  get(): RuntimeInfo {
    return buildRuntimeInfo({
      runtimeInfo: this.runtimeInfo.options(),
      startedAt: this.runtimeInfo.startedAt(),
      now: this.clock.now(),
      pid: this.process.pid(),
      platform: this.process.platform(),
      arch: this.process.arch(),
    });
  }
}
