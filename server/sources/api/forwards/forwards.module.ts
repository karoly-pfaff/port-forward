import { Module } from "@nestjs/common";
import { CLOCK_READER, defaultClockReader } from "../common/clock.reader.js";
import { APP_RUNTIME, type AppRuntime } from "../../app/runtime-context.js";
import { CreateForwardRuleService } from "./create-forward-rule.service.js";
import { DeleteForwardRuleService } from "./delete-forward-rule.service.js";
import { DiagnoseForwardRuleService } from "./diagnose-forward-rule.service.js";
import { ReorderForwardRulesService } from "./reorder-forward-rules.service.js";
import { StartForwardGroupService } from "./start-forward-group.service.js";
import { StartForwardRuleService } from "./start-forward-rule.service.js";
import { StopForwardGroupService } from "./stop-forward-group.service.js";
import { StopForwardRuleService } from "./stop-forward-rule.service.js";
import { UpdateForwardRuleService } from "./update-forward-rule.service.js";
import { ForwardsController } from "./forwards.controller.js";
import { createDefaultDiagnosticReader, DIAGNOSTIC_READER } from "./forwards-diagnostics.reader.js";
import { emptyForwardsReader, FORWARDS_READER } from "./forwards.reader.js";
import { ForwardsService } from "./forwards.service.js";
import {
  createDefaultForwardGroupStarter,
  createDefaultForwardGroupStopper,
  createDefaultForwardRuleCreator,
  createDefaultForwardRuleDeleter,
  createDefaultForwardRulesReorderer,
  createDefaultForwardRuleStarter,
  createDefaultForwardRuleStopper,
  createDefaultForwardRuleUpdater,
  FORWARD_GROUP_STARTER,
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
 * and the group-action pair `POST /api/forwards/groups/:group/stop` and
 * `POST /api/forwards/groups/:group/start` (write). The live `ForwardManager`
 * satisfies every reader/writer/diagnostic/group interface, so each token resolves
 * to it when a runtime is wired (the active NestJS runtime); otherwise each falls
 * back to an empty reader / fresh isolated in-memory manager, and tests override the
 * tokens with a seeded manager. The diagnose service also injects the shared
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
    StartForwardGroupService,
    { provide: FORWARDS_READER, useFactory: (rt: AppRuntime | null) => rt?.manager ?? emptyForwardsReader, inject: [APP_RUNTIME] },
    { provide: FORWARD_RULE_CREATOR, useFactory: (rt: AppRuntime | null) => rt?.manager ?? createDefaultForwardRuleCreator(), inject: [APP_RUNTIME] },
    { provide: FORWARD_RULE_UPDATER, useFactory: (rt: AppRuntime | null) => rt?.manager ?? createDefaultForwardRuleUpdater(), inject: [APP_RUNTIME] },
    { provide: FORWARD_RULE_DELETER, useFactory: (rt: AppRuntime | null) => rt?.manager ?? createDefaultForwardRuleDeleter(), inject: [APP_RUNTIME] },
    { provide: FORWARD_RULE_STARTER, useFactory: (rt: AppRuntime | null) => rt?.manager ?? createDefaultForwardRuleStarter(), inject: [APP_RUNTIME] },
    { provide: FORWARD_RULE_STOPPER, useFactory: (rt: AppRuntime | null) => rt?.manager ?? createDefaultForwardRuleStopper(), inject: [APP_RUNTIME] },
    { provide: FORWARD_RULES_REORDERER, useFactory: (rt: AppRuntime | null) => rt?.manager ?? createDefaultForwardRulesReorderer(), inject: [APP_RUNTIME] },
    { provide: DIAGNOSTIC_READER, useFactory: (rt: AppRuntime | null) => rt?.manager ?? createDefaultDiagnosticReader(), inject: [APP_RUNTIME] },
    { provide: FORWARD_GROUP_STOPPER, useFactory: (rt: AppRuntime | null) => rt?.manager ?? createDefaultForwardGroupStopper(), inject: [APP_RUNTIME] },
    { provide: FORWARD_GROUP_STARTER, useFactory: (rt: AppRuntime | null) => rt?.manager ?? createDefaultForwardGroupStarter(), inject: [APP_RUNTIME] },
    { provide: CLOCK_READER, useValue: defaultClockReader },
  ],
})
export class ForwardsModule {}
