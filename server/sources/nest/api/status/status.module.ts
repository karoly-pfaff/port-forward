import { Module } from "@nestjs/common";
import { APP_RUNTIME, type AppRuntime } from "../../common/runtime-context.js";
import { StatusController } from "./status.controller.js";
import { emptyStatusReader, STATUS_READER, type StatusReader } from "./status.reader.js";
import { StatusService } from "./status.service.js";

/**
 * `GET /api/status`. `STATUS_READER` resolves to the live `ForwardManager` when one
 * is wired (the active NestJS runtime), else an empty reader; tests override it with
 * a seeded manager. The status code itself holds no manager/store dependency.
 */
@Module({
  controllers: [StatusController],
  providers: [
    StatusService,
    {
      provide: STATUS_READER,
      useFactory: (rt: AppRuntime | null): StatusReader => rt?.manager ?? emptyStatusReader,
      inject: [APP_RUNTIME],
    },
  ],
})
export class StatusModule {}
