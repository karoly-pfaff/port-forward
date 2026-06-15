import { Module } from "@nestjs/common";
import { CreateForwardRuleService } from "./create-forward-rule.service.js";
import { UpdateForwardRuleService } from "./update-forward-rule.service.js";
import { ForwardsController } from "./forwards.controller.js";
import { emptyForwardsReader, FORWARDS_READER } from "./forwards.reader.js";
import { ForwardsService } from "./forwards.service.js";
import {
  createDefaultForwardRuleCreator,
  createDefaultForwardRuleUpdater,
  FORWARD_RULE_CREATOR,
  FORWARD_RULE_UPDATER,
} from "./forwards.writer.js";

/**
 * `GET /api/forwards` (list, read), `POST /api/forwards` (create, write), and
 * `PATCH /api/forwards/:id` (update, write). The `FORWARDS_READER` token defaults
 * to an empty reader and the `FORWARD_RULE_CREATOR`/`FORWARD_RULE_UPDATER` tokens
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
    { provide: FORWARDS_READER, useValue: emptyForwardsReader },
    { provide: FORWARD_RULE_CREATOR, useFactory: createDefaultForwardRuleCreator },
    { provide: FORWARD_RULE_UPDATER, useFactory: createDefaultForwardRuleUpdater },
  ],
})
export class ForwardsModule {}
