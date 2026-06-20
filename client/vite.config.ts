import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "build"
  },
  server: {
    // The API docs view imports the canonical OpenAPI artifact at docs/openapi.json
    // (repo root, outside the client package), so the dev server must be allowed to read it.
    fs: {
      allow: [".."]
    },
    proxy: {
      "/api": "http://127.0.0.1:47831"
    }
  }
});
