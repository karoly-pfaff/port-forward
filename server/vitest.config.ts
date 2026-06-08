import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["sources/**/*.test.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json-summary"],
      reportsDirectory: "../coverage/server",
      include: ["sources/**/*.ts"],
      exclude: ["sources/**/*.test.ts", "sources/test-helpers.ts"]
    }
  }
});
