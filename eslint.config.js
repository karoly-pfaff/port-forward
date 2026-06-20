import js from "@eslint/js";
import tseslint from "typescript-eslint";
import react from "eslint-plugin-react";
import reactHooks from "eslint-plugin-react-hooks";

export default tseslint.config(
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    ignores: ["**/build/**", "**/node_modules/**", "client/index.html"]
  },
  {
    files: ["**/*.{ts,tsx}"],
    languageOptions: {
      parserOptions: {
        projectService: {
          allowDefaultProject: ["server/vitest.config.ts", "shared/vitest.config.ts"]
        }
      }
    },
    rules: {
      "@typescript-eslint/no-unused-vars": ["error", { "argsIgnorePattern": "^_", "varsIgnorePattern": "^_" }],
      // Production sources are already `any`-free; enforce it so they stay that way.
      "@typescript-eslint/no-explicit-any": "error"
    }
  },
  // React/JSX hardening — scoped to the client only (server/shared/tools are not React).
  {
    files: ["client/sources/**/*.{ts,tsx}"],
    ...react.configs.flat.recommended,
    settings: { react: { version: "detect" } }
  },
  {
    files: ["client/sources/**/*.{ts,tsx}"],
    ...react.configs.flat["jsx-runtime"]
  },
  {
    files: ["client/sources/**/*.{ts,tsx}"],
    plugins: { "react-hooks": reactHooks },
    rules: {
      // TypeScript provides prop typing; PropTypes are not used in this codebase.
      "react/prop-types": "off",
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "error"
    }
  }
);
