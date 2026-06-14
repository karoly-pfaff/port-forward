import { Module } from "@nestjs/common";
import { ActivityStore } from "../../../activity/activity-store.js";
import { ActivityController } from "./activity.controller.js";
import { ActivityService } from "./activity.service.js";
import { ACTIVITY_STORE, type ActivityReader } from "./activity.reader.js";

/**
 * `GET /api/activity`. The `ACTIVITY_STORE` token defaults to a fresh in-memory
 * `ActivityStore` (empty → `{ events: [] }`); when the NestJS server becomes the
 * active runtime it will be bound to the shared store, and tests override it with
 * a seeded store. The store is the domain activity store, not Express internals.
 */
@Module({
  controllers: [ActivityController],
  providers: [
    ActivityService,
    { provide: ACTIVITY_STORE, useFactory: (): ActivityReader => new ActivityStore() },
  ],
})
export class ActivityModule {}
