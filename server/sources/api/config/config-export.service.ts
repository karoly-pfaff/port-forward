import { Inject, Injectable } from "@nestjs/common";
import type { ExportedConfig } from "@portier/shared";
import { buildExportedConfig } from "../../config/config-export.js";
import { CLOCK_READER, type ClockReader } from "../common/clock.reader.js";
import {
  CONFIG_EXPORT_READER,
  type ConfigExportReader,
} from "./config-export.reader.js";

/**
 * Behaviour for `GET /api/config/export`: snapshots the current rules from the
 * injected reader and stamps `exportedAt` from the injected clock, via the
 * shared `buildExportedConfig` builder (the same builder the `ForwardManager`
 * uses, so the two cannot drift). Read-only and pure — it does NOT emit the
 * `config.exported` activity event (that write side-effect stays with the
 * `ForwardManager` — the read is a pure snapshot); never throws.
 */
@Injectable()
export class ConfigExportService {
  constructor(
    @Inject(CONFIG_EXPORT_READER) private readonly reader: ConfigExportReader,
    @Inject(CLOCK_READER) private readonly clock: ClockReader
  ) {}

  export(): ExportedConfig {
    return buildExportedConfig({ rules: this.reader.listRules(), now: this.clock.now() });
  }
}
