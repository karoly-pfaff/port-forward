import { Module } from "@nestjs/common";
import { ForwardsController } from "./forwards.controller.js";
import { emptyForwardsReader, FORWARDS_READER } from "./forwards.reader.js";
import { ForwardsService } from "./forwards.service.js";

/**
 * `GET /api/forwards`. The `FORWARDS_READER` token defaults to an empty reader
 * (the scaffold has no runtime wired); when the NestJS server becomes the active
 * runtime it is bound to the shared `ForwardManager`, and tests override it with
 * a seeded manager. Production forwards code holds no manager/store dependency.
 */
@Module({
  controllers: [ForwardsController],
  providers: [ForwardsService, { provide: FORWARDS_READER, useValue: emptyForwardsReader }],
})
export class ForwardsModule {}
