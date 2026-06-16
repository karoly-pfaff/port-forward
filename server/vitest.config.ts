import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["sources/**/*.test.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json-summary"],
      reportsDirectory: "../coverage/server",
      include: ["sources/**/*.ts"],
      // index.ts is the default (NestJS) HTTP server entry point (startup, signal handlers,
      // process lifecycle, live-dependency wiring). It is integration-tested via the live-runtime
      // integration test + E2E; unit coverage of the process entry is not meaningful (importing
      // it boots a server).
      // forwarders/types.ts is an interface-only file with no executable JavaScript.
      // openapi/generate.ts and openapi/copy-release.ts are logic-free process entries for OpenAPI
      // doc generation / release copy — their helpers (openapi/openapi.ts) are fully unit-covered.
      // **/*.schema.ts are metadata-only OpenAPI schema files: decorated `@ApiProperty` classes
      // that mirror the @portier/shared REST types for OpenAPI generation. They are NEVER instantiated
      // (controllers reference them via @ApiOkResponse; the response mappers — the covered logic, in
      // the sibling *.response.dto.ts — return plain objects), so there is no executable logic to
      // cover, only decorator metadata. Each lives in its owning feature folder.
      exclude: ["sources/**/*.test.ts", "sources/testing/test-helpers.ts", "sources/index.ts", "sources/openapi/generate.ts", "sources/openapi/copy-release.ts", "sources/**/*.schema.ts", "sources/forwarders/types.ts", "build/**", "*.config.ts"]
    }
  }
});
