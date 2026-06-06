import crypto from "node:crypto";
import type { ActivityEvent, ActivityEventInput, ActivityEventType, ActivitySeverity } from "@portier/shared";

const MAX_EVENTS = 500;

export interface ActivityListParams {
  limit?: number;
  ruleId?: string;
  type?: ActivityEventType;
  severity?: ActivitySeverity;
}

export class ActivityStore {
  // Stored newest-first; new events are unshifted to the front.
  private events: ActivityEvent[] = [];

  add(input: ActivityEventInput): ActivityEvent {
    const event: ActivityEvent = {
      ...input,
      id: crypto.randomUUID(),
      timestamp: new Date().toISOString()
    };

    this.events.unshift(event);

    if (this.events.length > MAX_EVENTS) {
      this.events.length = MAX_EVENTS;
    }

    return event;
  }

  list(params: ActivityListParams = {}): ActivityEvent[] {
    const limit = Math.min(params.limit ?? 100, MAX_EVENTS);
    let result = this.events;

    if (params.ruleId !== undefined) {
      const ruleId = params.ruleId;
      result = result.filter((e) => e.ruleId === ruleId);
    }
    if (params.type !== undefined) {
      const type = params.type;
      result = result.filter((e) => e.type === type);
    }
    if (params.severity !== undefined) {
      const severity = params.severity;
      result = result.filter((e) => e.severity === severity);
    }

    return result.slice(0, limit);
  }

  clear(): void {
    this.events = [];
  }
}
