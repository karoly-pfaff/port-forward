import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["sources/**/*.test.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json-summary"],
      reportsDirectory: "../coverage/server",
      include: ["sources/**/*.ts"],
      // index.ts is the HTTP server entry point (startup, signal handlers, process lifecycle).
      // It is integration-tested via E2E only; unit coverage is not meaningful.
      // forwarders/types.ts is an interface-only file with no executable JavaScript.
      exclude: ["sources/**/*.test.ts", "sources/test-helpers.ts", "sources/index.ts", "sources/forwarders/types.ts", "build/**", "*.config.ts"]
    }
  }
});
