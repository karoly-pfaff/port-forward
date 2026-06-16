import { Inject, Injectable } from "@nestjs/common";
import type { ConfigApplyResponse, ExportedConfig } from "@portier/shared";
import { buildApplyImportFromPlan, buildConfigPlan } from "../../config/config-plan.js";
import { ApiBadRequestException } from "../common/api-errors.js";
import { CLOCK_READER, type ClockReader } from "../common/clock.reader.js";
import { CONFIG_APPLIER, type ConfigApplier } from "./config-apply.writer.js";

/**
 * Behaviour for `POST /api/config/apply` — the MUTATING desired-state apply.
 * Mirrors the Express route exactly, in the SAME order:
 *
 *   1. A missing `desired` key → `400 ["desired is required."]` (a key-presence
 *      check exactly like Express's `"desired" in body`; `desired: null` passes and
 *      surfaces as a plan error → `ok:false`, NOT a `400`).
 *   2. `plan.summary.hasErrors` → `200 {ok:false, dryRun, appliedAt, plan, applied:0s}`
 *      (errors-first; the `unchanged` count is still surfaced).
 *   3. `dryRun` → `200 {ok:true, dryRun:true, appliedAt, plan, applied}` — checked
 *      BEFORE the destructive gate, so a destructive dry-run is `ok:true` and does
 *      NOT require `yes`.
 *   4. `destructive > 0 && !yes` → `400 ["Apply requires yes: true ..."]`.
 *   5. `hasDrift` → `replace` import; if that import reports errors (a belt-and-
 *      suspenders guard — unreachable from this path because duplicate bindings and
 *      invalid rules are pre-blocked in the plan, and persist failures throw), surface
 *      them through `plan.errors` as `200 {ok:false, ...}` with zero applied counts.
 *   6. Otherwise → `200 {ok:true, dryRun:false, appliedAt, plan, applied}`.
 *
 * Both volatile timestamps — the top-level `appliedAt` and the embedded
 * `plan.generatedAt` — are stamped from the SAME injected-clock instant (pinned in
 * parity tests so the whole response is byte-for-byte comparable). The two `400`s are
 * thrown as `ApiBadRequestException` (the pure `{errors}` envelope via the shared
 * filter); every other outcome is RETURNED with status `200`, so the controller
 * needs only `@HttpCode(200)` and no `@Res` (unlike import, which had a `422`).
 * Mutation happens ONLY on the non-dry-run drift path, via the SAME `importConfig`
 * Express uses (so rollback/start/activity semantics are identical).
 */
@Injectable()
export class ConfigApplyService {
  constructor(
    @Inject(CONFIG_APPLIER) private readonly applier: ConfigApplier,
    @Inject(CLOCK_READER) private readonly clock: ClockReader
  ) {}

  async apply(body: unknown): Promise<ConfigApplyResponse> {
    if (!body || typeof body !== "object" || !("desired" in body)) {
      throw new ApiBadRequestException(["desired is required."]);
    }

    const { desired, yes: rawYes, dryRun: rawDryRun } = body as {
      desired: unknown;
      yes?: unknown;
      dryRun?: unknown;
    };
    const yes = rawYes === true;
    const dryRun = rawDryRun === true;
    // One clock instant stamps both volatile fields: the top-level `appliedAt` and
    // the embedded `plan.generatedAt` (so the whole response is byte-for-byte
    // comparable when the clock is pinned).
    const now = this.clock.now();
    const appliedAt = now.toISOString();
    const plan = buildConfigPlan({ currentRules: this.applier.listRules(), desiredRaw: desired, now });

    if (plan.summary.hasErrors) {
      return {
        ok: false,
        dryRun,
        appliedAt,
        plan,
        applied: { add: 0, update: 0, remove: 0, unchanged: plan.summary.unchanged },
      };
    }

    const { rules: rulesForImport, applied } = buildApplyImportFromPlan(plan);

    if (dryRun) {
      return { ok: true, dryRun: true, appliedAt, plan, applied };
    }

    if (plan.summary.destructive > 0 && !yes) {
      throw new ApiBadRequestException(["Apply requires yes: true when destructive operations are present."]);
    }

    if (plan.summary.hasDrift) {
      const importCfg: ExportedConfig = { version: "1", exportedAt: appliedAt, rules: rulesForImport };
      const result = await this.applier.importConfig(importCfg, "replace");
      if (result.errors.length > 0) {
        const planWithErrors = {
          ...plan,
          errors: [...plan.errors, ...result.errors.map((message) => ({ code: "IMPORT_ERROR", message }))],
          summary: { ...plan.summary, hasErrors: true },
        };
        return {
          ok: false,
          dryRun: false,
          appliedAt,
          plan: planWithErrors,
          applied: { add: 0, update: 0, remove: 0, unchanged: plan.summary.unchanged },
        };
      }
    }

    return { ok: true, dryRun: false, appliedAt, plan, applied };
  }
}
