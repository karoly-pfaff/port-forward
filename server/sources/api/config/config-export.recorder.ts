import type { ActivityStore } from "../../activity/activity-store.js";
import { configExportedActivityEvent } from "../../config/config-export.js";

/**
 * Narrow write capability owning the `GET /api/config/export` side-effect: records
 * that a successful config export happened. Separating it from the pure
 * `ConfigExportReader` (which only reads rules) and the pure `buildExportedConfig`
 * builder keeps the read path a pure snapshot while the activity emission lives in
 * exactly one place. Mirrors the live/no-op provider pattern the other feature
 * tokens use.
 */
export interface ConfigExportRecorder {
  recordExport(ruleCount: number): void;
}

/** Injection token for the config-export recorder. */
export const CONFIG_EXPORT_RECORDER = "CONFIG_EXPORT_RECORDER";

/**
 * Default: no forwarding runtime / activity store is wired into the static
 * AppModule (OpenAPI generation and tests), so a config export records nothing.
 * The live runtime overrides this with `createConfigExportRecorder` so a real
 * export emits the `config.exported` activity event; tests that assert emission
 * inject a recording fake.
 */
export const noopConfigExportRecorder: ConfigExportRecorder = {
  recordExport: () => {},
};

/**
 * Live recorder: emits the canonical `config.exported` event (via the shared
 * `configExportedActivityEvent` payload, so it cannot drift from the
 * `ForwardManager`) into the activity store the live runtime is built with. Takes
 * the minimal `add` capability of the `ActivityStore`.
 */
export function createConfigExportRecorder(
  activity: Pick<ActivityStore, "add">
): ConfigExportRecorder {
  return {
    recordExport: (ruleCount: number) => {
      activity.add(configExportedActivityEvent(ruleCount));
    },
  };
}
