import { Module } from "@nestjs/common";
import { CLOCK_READER, defaultClockReader } from "../../common/clock.reader.js";
import { CreateForwardRuleService } from "./create-forward-rule.service.js";
import { DeleteForwardRuleService } from "./delete-forward-rule.service.js";
import { DiagnoseForwardRuleService } from "./diagnose-forward-rule.service.js";
import { ReorderForwardRulesService } from "./reorder-forward-rules.service.js";
import { StartForwardRuleService } from "./start-forward-rule.service.js";
import { StopForwardGroupService } from "./stop-forward-group.service.js";
import { StopForwardRuleService } from "./stop-forward-rule.service.js";
import { UpdateForwardRuleService } from "./update-forward-rule.service.js";
import { ForwardsController } from "./forwards.controller.js";
import { createDefaultDiagnosticReader, DIAGNOSTIC_READER } from "./forwards-diagnostics.reader.js";
import { emptyForwardsReader, FORWARDS_READER } from "./forwards.reader.js";
import { ForwardsService } from "./forwards.service.js";
import {
  createDefaultForwardGroupStopper,
  createDefaultForwardRuleCreator,
  createDefaultForwardRuleDeleter,
  createDefaultForwardRulesReorderer,
  createDefaultForwardRuleStarter,
  createDefaultForwardRuleStopper,
  createDefaultForwardRuleUpdater,
  FORWARD_GROUP_STOPPER,
  FORWARD_RULE_CREATOR,
  FORWARD_RULE_DELETER,
  FORWARD_RULES_REORDERER,
  FORWARD_RULE_STARTER,
  FORWARD_RULE_STOPPER,
  FORWARD_RULE_UPDATER,
} from "./forwards.writer.js";

/**
 * `GET /api/forwards` (list, read), `POST /api/forwards` (create, write),
 * `PATCH /api/forwards/:id` (update, write), `DELETE /api/forwards/:id`
 * (delete, write), the lifecycle pair `POST /api/forwards/:id/start` and
 * `POST /api/forwards/:id/stop`, `POST /api/forwards/reorder` (reorder, write),
 * `POST /api/forwards/:id/diagnose` (diagnose, read-only), and
 * `POST /api/forwards/groups/:group/stop` (group stop, write — the first migrated
 * group action; group START stays with Express for now). The `FORWARDS_READER`
 * token defaults to an empty reader and the write/lifecycle tokens plus the
 * `DIAGNOSTIC_READER`/`FORWARD_GROUP_STOPPER` to fresh isolated in-memory managers
 * (the scaffold has no real runtime wired); when the NestJS server becomes the
 * active runtime they are bound to the shared `ForwardManager`, and tests override
 * them with a seeded manager. The diagnose service also injects the shared
 * `CLOCK_READER` (registered here) so its volatile `diagnosedAt` can be pinned in
 * parity tests.
 */
@Module({
  controllers: [ForwardsController],
  providers: [
    ForwardsService,
    CreateForwardRuleService,
    UpdateForwardRuleService,
    DeleteForwardRuleService,
    StartForwardRuleService,
    StopForwardRuleService,
    ReorderForwardRulesService,
    DiagnoseForwardRuleService,
    StopForwardGroupService,
    { provide: FORWARDS_READER, useValue: emptyForwardsReader },
    { provide: FORWARD_RULE_CREATOR, useFactory: createDefaultForwardRuleCreator },
    { provide: FORWARD_RULE_UPDATER, useFactory: createDefaultForwardRuleUpdater },
    { provide: FORWARD_RULE_DELETER, useFactory: createDefaultForwardRuleDeleter },
    { provide: FORWARD_RULE_STARTER, useFactory: createDefaultForwardRuleStarter },
    { provide: FORWARD_RULE_STOPPER, useFactory: createDefaultForwardRuleStopper },
    { provide: FORWARD_RULES_REORDERER, useFactory: createDefaultForwardRulesReorderer },
    { provide: DIAGNOSTIC_READER, useFactory: createDefaultDiagnosticReader },
    { provide: FORWARD_GROUP_STOPPER, useFactory: createDefaultForwardGroupStopper },
    { provide: CLOCK_READER, useValue: defaultClockReader },
  ],
})
export class ForwardsModule {}
