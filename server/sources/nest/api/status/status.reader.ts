import type { ForwardStatus } from "@portier/shared";

/**
 * Narrow read-only view of the forward manager that `GET /api/status` needs.
 * The real domain `ForwardManager` satisfies it; tests inject a seeded manager.
 * This is the runtime/manager read-provider seam future read endpoints
 * (`/api/forwards`, `/api/connections`) will reuse.
 */
export interface StatusReader {
  listStatus(): ForwardStatus[];
}

/** Injection token for the status reader. */
export const STATUS_READER = "STATUS_READER";

/**
 * Default: no forwarding runtime is wired into the NestJS app yet, so
 * the status list is empty. When the NestJS server becomes the active runtime
 * this token is bound to the shared `ForwardManager`; tests override it with a
 * seeded manager.
 */
export const emptyStatusReader: StatusReader = {
  listStatus: () => [],
};
