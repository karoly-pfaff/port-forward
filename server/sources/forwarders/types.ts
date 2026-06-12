import type { ForwardStatus } from "@portier/shared";

// Forwarders track runtime counters but not the rule's `enabled` flag, so they
// cannot classify `health` (an enabled-but-stopped rule is a "warning"). The
// ForwardManager owns that — it derives `health` when building the public
// ForwardStatus. Forwarders therefore return everything except `health`.
export type ForwarderStatus = Omit<ForwardStatus, "health">;

export interface Forwarder {
  start(): Promise<void>;
  stop(): Promise<void>;
  getStatus(): ForwarderStatus;
}
