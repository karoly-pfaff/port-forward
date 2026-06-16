import { Inject, Injectable } from "@nestjs/common";
import type { ConfigPlanResponse } from "@portier/shared";
import { buildConfigPlan } from "../../config/config-plan.js";
import { ApiBadRequestException } from "../common/api-errors.js";
import { CLOCK_READER, type ClockReader } from "../common/clock.reader.js";
import { CONFIG_PLAN_READER, type ConfigPlanReader } from "./config-plan.reader.js";

/**
 * Behaviour for `POST /api/config/plan`: a NON-MUTATING dry-run that diffs the
 * desired config against the current rules and returns the plan. Mirrors the
 * documented `/api` contract — it inline-validates that the body is an object carrying
 * a `desired` key (`400 ["desired is required."]` otherwise; note `desired: null`
 * passes this key-presence check, exactly like the documented `"desired" in body`, and
 * surfaces as a plan error, NOT a `400`), then runs the SHARED `buildConfigPlan`
 * over the injected reader's current rules with `generatedAt` stamped from the
 * injected clock (so the volatile timestamp can be pinned for byte-for-byte
 * parity). It never mutates rules, opens sockets, or emits activity.
 */
@Injectable()
export class ConfigPlanService {
  constructor(
    @Inject(CONFIG_PLAN_READER) private readonly reader: ConfigPlanReader,
    @Inject(CLOCK_READER) private readonly clock: ClockReader
  ) {}

  plan(body: unknown): ConfigPlanResponse {
    if (!body || typeof body !== "object" || !("desired" in body)) {
      throw new ApiBadRequestException(["desired is required."]);
    }
    return buildConfigPlan({
      currentRules: this.reader.listRules(),
      desiredRaw: (body as { desired: unknown }).desired,
      now: this.clock.now(),
    });
  }
}
