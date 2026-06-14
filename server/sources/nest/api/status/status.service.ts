import { Inject, Injectable } from "@nestjs/common";
import type { ForwardStatus } from "@portier/shared";
import { STATUS_READER, type StatusReader } from "./status.reader.js";

/**
 * Behaviour for `GET /api/status`: returns the current per-rule status list from
 * the injected reader. Read-only; never throws. The shape (and `health`
 * derivation) is owned by the domain manager, so the runtimes cannot drift.
 */
@Injectable()
export class StatusService {
  constructor(@Inject(STATUS_READER) private readonly reader: StatusReader) {}

  list(): ForwardStatus[] {
    return this.reader.listStatus();
  }
}
