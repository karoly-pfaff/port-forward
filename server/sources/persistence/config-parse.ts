import type { ForwardRule } from "@portier/shared";
import { validateForwardRule } from "@portier/shared";

/**
 * Typed config-parse errors so the startup recovery loader can classify a load
 * failure by error class rather than matching message text — the TypeScript
 * mirror of the Go service `config.ErrMalformed` / `config.ErrSchemaInvalid`
 * (`service/sources/config/config.go`).
 *
 * `MalformedConfigError` marks bytes that are not a valid config container
 * (invalid JSON, or a JSON value that is not a rules array). `SchemaInvalidConfigError`
 * marks a container that decoded but contains at least one rule that fails validation.
 */
export class MalformedConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "MalformedConfigError";
  }
}

export class SchemaInvalidConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "SchemaInvalidConfigError";
  }
}

/**
 * Parse and validate raw config text into forward rules. Pure (no IO) — the
 * core of `ConfigStore.load`, factored out so the recovery loader can classify a
 * read-from-disk config without duplicating the decode/validate rules. Failures
 * throw `MalformedConfigError` (bad container) or `SchemaInvalidConfigError` (bad
 * rule); message text is preserved from the historical `ConfigStore.load`.
 */
export function parseConfig(raw: string): ForwardRule[] {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch (error) {
    /* v8 ignore next -- JSON.parse always throws a SyntaxError (an Error); the String() fallback is defensive */
    const detail = error instanceof Error ? error.message : String(error);
    throw new MalformedConfigError(`Invalid JSON: ${detail}`);
  }

  if (!Array.isArray(parsed)) {
    throw new MalformedConfigError("Config file must contain an array of forward rules.");
  }

  return parsed.map((item, index) => {
    const result = validateForwardRule(item);
    if (!result.valid || !result.value?.id) {
      throw new SchemaInvalidConfigError(`Invalid rule at index ${index}: ${result.errors.join(" ")}`);
    }
    return result.value as ForwardRule;
  });
}
