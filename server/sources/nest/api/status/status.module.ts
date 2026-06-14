import { Module } from "@nestjs/common";
import { StatusController } from "./status.controller.js";
import { emptyStatusReader, STATUS_READER } from "./status.reader.js";
import { StatusService } from "./status.service.js";

/**
 * `GET /api/status`. The `STATUS_READER` token defaults to an empty reader (the
 * scaffold has no runtime wired); when the NestJS server becomes the active
 * runtime it is bound to the shared `ForwardManager`, and tests override it with
 * a seeded manager. Production status code holds no manager/store dependency.
 */
@Module({
  controllers: [StatusController],
  providers: [StatusService, { provide: STATUS_READER, useValue: emptyStatusReader }],
})
export class StatusModule {}
