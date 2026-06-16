import { Inject, Injectable } from "@nestjs/common";
import { summarizeGroupAction, validateGroupName, type GroupActionResponse } from "@portier/shared";
import { ApiBadRequestException, ApiNotFoundException } from "../common/api-errors.js";
import { FORWARD_GROUP_STARTER, type ForwardGroupStarter } from "./forwards.writer.js";

/**
 * Behaviour for `POST /api/forwards/groups/:group/start`: implements the documented
 * `handleGroupAction("start")` exactly (the start counterpart to the stop
 * service). It normalizes the path group the same way (`decodeURIComponent(...).trim()`),
 * validates it with the SHARED `validateGroupName` (a `400 { errors }` on failure —
 * delegated to the shared validator, not re-expressed in class-validator, so the
 * messages cannot drift), starts every rule sharing the group via the injected
 * starter (`ForwardManager.startGroup` — the shared domain path, so the per-rule
 * skip/start/fail outcomes are identical), maps an empty result set to a `404`, and
 * otherwise summarizes the results with the SHARED `summarizeGroupAction`. The
 * `Api*Exception`s carry the contract envelope (owned by the shared error filter),
 * the documented `400`/`404`.
 */
@Injectable()
export class StartForwardGroupService {
  constructor(@Inject(FORWARD_GROUP_STARTER) private readonly starter: ForwardGroupStarter) {}

  async start(group: string): Promise<GroupActionResponse> {
    const normalized = decodeURIComponent(String(group)).trim();
    const errors = validateGroupName(normalized);
    if (errors.length > 0) {
      throw new ApiBadRequestException(errors);
    }
    const results = await this.starter.startGroup(normalized);
    if (results.length === 0) {
      throw new ApiNotFoundException([`No rules found in group "${normalized}".`]);
    }
    return summarizeGroupAction(normalized, "start", results);
  }
}
