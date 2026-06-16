import { Inject, Injectable } from "@nestjs/common";
import type { ExportedConfig } from "@portier/shared";
import { buildExportedConfig } from "../../config/config-export.js";
import { CLOCK_READER, type ClockReader } from "../common/clock.reader.js";
import {
  CONFIG_EXPORT_READER,
  type ConfigExportReader,
} from "./config-export.reader.js";
import {
  CONFIG_EXPORT_RECORDER,
  type ConfigExportRecorder,
} from "./config-export.recorder.js";

/**
 * Behaviour for `GET /api/config/export`: snapshots the current rules from the
 * injected reader and stamps `exportedAt` from the injected clock, via the
 * shared `buildExportedConfig` builder (the same builder the `ForwardManager`
 * uses, so the two cannot drift). The export shape stays pure; the one
 * side-effect — recording a `config.exported` activity event — is delegated to
 * the injected recorder (live runtime emits; the static/OpenAPI/test default is a
 * no-op), so exactly one event is emitted per successful export and the response
 * body is unchanged. Never throws.
 */
@Injectable()
export class ConfigExportService {
  constructor(
    @Inject(CONFIG_EXPORT_READER) private readonly reader: ConfigExportReader,
    @Inject(CLOCK_READER) private readonly clock: ClockReader,
    @Inject(CONFIG_EXPORT_RECORDER) private readonly recorder: ConfigExportRecorder
  ) {}

  export(): ExportedConfig {
    const config = buildExportedConfig({ rules: this.reader.listRules(), now: this.clock.now() });
    this.recorder.recordExport(config.rules.length);
    return config;
  }
}
