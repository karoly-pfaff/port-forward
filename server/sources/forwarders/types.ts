import type { ForwardStatus } from "@portier/shared";

export interface Forwarder {
  start(): Promise<void>;
  stop(): Promise<void>;
  getStatus(): ForwardStatus;
}
