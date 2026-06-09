import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["sources/**/*.test.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json-summary"],
      reportsDirectory: "../coverage/shared",
      include: ["sources/**/*.ts"],
      // activity.ts, connections.ts, plan.ts are type-only files (export type / interface only).
      // They produce no executable JavaScript and will always report 0% under V8 coverage.
      // Excluding them from the include set keeps the aggregate denominator honest.
      exclude: ["sources/**/*.test.ts", "sources/activity.ts", "sources/connections.ts", "sources/plan.ts", "build/**", "*.config.ts"]
    }
  }
});
