import { Module } from "@nestjs/common";
import { CreateForwardRuleService } from "./create-forward-rule.service.js";
import { ForwardsController } from "./forwards.controller.js";
import { emptyForwardsReader, FORWARDS_READER } from "./forwards.reader.js";
import { ForwardsService } from "./forwards.service.js";
import { createDefaultForwardRuleCreator, FORWARD_RULE_CREATOR } from "./forwards.writer.js";

/**
 * `GET /api/forwards` (list, read) + `POST /api/forwards` (create, write). The
 * `FORWARDS_READER` token defaults to an empty reader and the `FORWARD_RULE_CREATOR`
 * token to a fresh isolated in-memory manager (the scaffold has no real runtime
 * wired); when the NestJS server becomes the active runtime both are bound to the
 * shared `ForwardManager`, and tests override them with a seeded manager.
 */
@Module({
  controllers: [ForwardsController],
  providers: [
    ForwardsService,
    CreateForwardRuleService,
    { provide: FORWARDS_READER, useValue: emptyForwardsReader },
    { provide: FORWARD_RULE_CREATOR, useFactory: createDefaultForwardRuleCreator },
  ],
})
export class ForwardsModule {}
