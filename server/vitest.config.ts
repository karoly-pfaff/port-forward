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
      // nest/main.ts is the NestJS scaffold (v1.14) process entry point — logic-free wiring only:
      // its constituents (resolveNestListenOptions, bootstrap, reportBootstrapFailure) are fully
      // unit-covered, and importing main.ts would start a real listener as a side effect. Same
      // rationale as index.ts. All other nest/ files are covered at 100%.
      // nest/openapi/generate.ts is the OpenAPI-doc generation process entry — logic-free wiring
      // (its helpers in nest/openapi/openapi.ts are fully unit-covered).
      // nest/**/*.schema.ts are metadata-only OpenAPI schema files: decorated `@ApiProperty` classes
      // that mirror the @portier/shared REST types for OpenAPI generation. They are NEVER instantiated
      // (controllers reference them via @ApiOkResponse; the response mappers — the covered logic, in
      // the sibling *.response.dto.ts — return plain objects), so there is no executable logic to
      // cover, only decorator metadata. Each lives in its owning feature folder.
      exclude: ["sources/**/*.test.ts", "sources/test-helpers.ts", "sources/index.ts", "sources/nest/main.ts", "sources/nest/openapi/generate.ts", "sources/nest/**/*.schema.ts", "sources/forwarders/types.ts", "build/**", "*.config.ts"]
    }
  }
});
