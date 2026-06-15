import { Module } from "@nestjs/common";
import { CreateForwardRuleService } from "./create-forward-rule.service.js";
import { DeleteForwardRuleService } from "./delete-forward-rule.service.js";
import { UpdateForwardRuleService } from "./update-forward-rule.service.js";
import { ForwardsController } from "./forwards.controller.js";
import { emptyForwardsReader, FORWARDS_READER } from "./forwards.reader.js";
import { ForwardsService } from "./forwards.service.js";
import {
  createDefaultForwardRuleCreator,
  createDefaultForwardRuleDeleter,
  createDefaultForwardRuleUpdater,
  FORWARD_RULE_CREATOR,
  FORWARD_RULE_DELETER,
  FORWARD_RULE_UPDATER,
} from "./forwards.writer.js";

/**
 * `GET /api/forwards` (list, read), `POST /api/forwards` (create, write),
 * `PATCH /api/forwards/:id` (update, write), and `DELETE /api/forwards/:id`
 * (delete, write). The `FORWARDS_READER` token defaults to an empty reader and
 * the `FORWARD_RULE_CREATOR`/`FORWARD_RULE_UPDATER`/`FORWARD_RULE_DELETER` tokens
 * to fresh isolated in-memory managers (the scaffold has no real runtime wired);
 * when the NestJS server becomes the active runtime they are bound to the shared
 * `ForwardManager`, and tests override them with a seeded manager.
 */
@Module({
  controllers: [ForwardsController],
  providers: [
    ForwardsService,
    CreateForwardRuleService,
    UpdateForwardRuleService,
    DeleteForwardRuleService,
    { provide: FORWARDS_READER, useValue: emptyForwardsReader },
    { provide: FORWARD_RULE_CREATOR, useFactory: createDefaultForwardRuleCreator },
    { provide: FORWARD_RULE_UPDATER, useFactory: createDefaultForwardRuleUpdater },
    { provide: FORWARD_RULE_DELETER, useFactory: createDefaultForwardRuleDeleter },
  ],
})
export class ForwardsModule {}
