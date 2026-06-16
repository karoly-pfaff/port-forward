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
      // process lifecycle, live-dependency wiring). It is integration-tested via the switch
      // integration tests + E2E; unit coverage of the process entry is not meaningful (importing
      // it boots a server). legacy/index.ts is the retained Express entry — same rationale.
      // forwarders/types.ts is an interface-only file with no executable JavaScript.
      // nest/openapi/generate.ts is the OpenAPI-doc generation process entry — logic-free wiring
      // (its helpers in nest/openapi/openapi.ts are fully unit-covered).
      // nest/**/*.schema.ts are metadata-only OpenAPI schema files: decorated `@ApiProperty` classes
      // that mirror the @portier/shared REST types for OpenAPI generation. They are NEVER instantiated
      // (controllers reference them via @ApiOkResponse; the response mappers — the covered logic, in
      // the sibling *.response.dto.ts — return plain objects), so there is no executable logic to
      // cover, only decorator metadata. Each lives in its owning feature folder.
      exclude: ["sources/**/*.test.ts", "sources/test-helpers.ts", "sources/index.ts", "sources/legacy/index.ts", "sources/nest/openapi/generate.ts", "sources/nest/**/*.schema.ts", "sources/forwarders/types.ts", "build/**", "*.config.ts"]
    }
  }
});
