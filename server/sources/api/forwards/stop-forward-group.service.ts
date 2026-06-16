import { Inject, Injectable } from "@nestjs/common";
import { summarizeGroupAction, validateGroupName, type GroupActionResponse } from "@portier/shared";
import { ApiBadRequestException, ApiNotFoundException } from "../common/api-errors.js";
import { FORWARD_GROUP_STOPPER, type ForwardGroupStopper } from "./forwards.writer.js";

/**
 * Behaviour for `POST /api/forwards/groups/:group/stop`: mirrors Express's
 * `handleGroupAction("stop")` exactly. It normalizes the path group the same way
 * (`decodeURIComponent(...).trim()`), validates it with the SHARED
 * `validateGroupName` (a `400 { errors }` on failure — delegated to the shared
 * validator, not re-expressed in class-validator, so the messages cannot drift),
 * stops every rule sharing the group via the injected stopper
 * (`ForwardManager.stopGroup` — the SAME path Express uses, so the per-rule
 * skip/stop/fail outcomes are identical), maps an empty result set to a `404`, and
 * otherwise summarizes the results with the SHARED `summarizeGroupAction`. The
 * `Api*Exception`s carry the contract envelope (owned by the shared error filter),
 * matching Express's inline `400`/`404`.
 */
@Injectable()
export class StopForwardGroupService {
  constructor(@Inject(FORWARD_GROUP_STOPPER) private readonly stopper: ForwardGroupStopper) {}

  async stop(group: string): Promise<GroupActionResponse> {
    const normalized = decodeURIComponent(String(group)).trim();
    const errors = validateGroupName(normalized);
    if (errors.length > 0) {
      throw new ApiBadRequestException(errors);
    }
    const results = await this.stopper.stopGroup(normalized);
    if (results.length === 0) {
      throw new ApiNotFoundException([`No rules found in group "${normalized}".`]);
    }
    return summarizeGroupAction(normalized, "stop", results);
  }
}
