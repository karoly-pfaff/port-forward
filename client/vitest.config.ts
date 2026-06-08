import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    // Point directly at shared TypeScript source so tests don't need the shared build.
    alias: {
      "@portier/shared": path.resolve(__dirname, "../shared/sources/index.ts")
    }
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./sources/test/setup.ts"],
    include: ["sources/**/*.test.ts", "sources/**/*.test.tsx"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json-summary"],
      reportsDirectory: "../coverage/client",
      include: ["sources/**/*.{ts,tsx}"],
      exclude: ["sources/**/*.test.{ts,tsx}", "sources/test/**"]
    }
  }
});
