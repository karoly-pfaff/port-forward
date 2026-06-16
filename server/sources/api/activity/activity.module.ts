import { Module } from "@nestjs/common";
import { ActivityStore } from "../../activity/activity-store.js";
import { APP_RUNTIME, type AppRuntime } from "../../app/runtime-context.js";
import { ActivityController } from "./activity.controller.js";
import { ActivityService } from "./activity.service.js";
import { ACTIVITY_STORE, type ActivityReader } from "./activity.reader.js";

/**
 * `GET /api/activity` + `DELETE /api/activity`. `ACTIVITY_STORE` resolves to the
 * live `ActivityStore` when one is wired (the active NestJS runtime, shared with the
 * forwarding manager so events appear), else a fresh in-memory store
 * (empty → `{ events: [] }`); tests override it with a seeded store. The store is the
 * domain activity store.
 */
@Module({
  controllers: [ActivityController],
  providers: [
    ActivityService,
    {
      provide: ACTIVITY_STORE,
      useFactory: (rt: AppRuntime | null): ActivityReader => rt?.activity ?? new ActivityStore(),
      inject: [APP_RUNTIME],
    },
  ],
})
export class ActivityModule {}
