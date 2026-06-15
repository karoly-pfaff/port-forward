import { Inject, Injectable } from "@nestjs/common";
import type { ExportedConfig, ForwardRuleResponse, ImportResult } from "@portier/shared";
import { ApiBadRequestException } from "../../common/api-errors.js";
import { toForwardRuleResponse } from "../forwards/forward-rule-response.js";
import { CONFIG_IMPORTER, type ConfigImporter } from "./config-import.writer.js";

/** The `200` success body: the import result + the full rule list (with advisories). */
export interface ConfigImportSuccessBody {
  result: ImportResult;
  rules: ForwardRuleResponse[];
}

/** The `422` body: the import errors + the result (errors plus result — NOT the plain `{errors}` envelope). */
export interface ConfigImportErrorBody {
  errors: string[];
  result: ImportResult;
}

/**
 * The import outcome. The two `400`s (invalid mode / invalid config) are thrown as
 * `ApiBadRequestException` (the pure error envelope, via the shared filter). The
 * `200`/`422` are RETURNED with their status because the `422` body is `{errors,
 * result}` — NOT the plain `{errors}` envelope — so it must NOT go through the
 * error-envelope filter (which would strip `result`); this mirrors Express, which
 * returns the `422` via `response.status(422).json(...)`, not error middleware.
 */
export type ConfigImportOutcome =
  | { status: 200; body: ConfigImportSuccessBody }
  | { status: 422; body: ConfigImportErrorBody };

/**
 * Behaviour for `POST /api/config/import`: mirrors the Express route exactly. It
 * short-circuit-validates the body (mode first, then config — `400` in that order,
 * delegated to inline checks so the messages + order cannot drift from Express;
 * class-validator would accumulate both errors and diverge), imports via the
 * injected importer (`ForwardManager.importConfig` — the SAME path Express uses, so
 * the replace/merge mutation, duplicate-binding/merge-conflict rejection, persist
 * rollback, enabled-rule start, and activity emission are identical), and returns
 * `422 {errors, result}` when the import reports errors or `200 {result, rules}`
 * (the full rule list decorated with advisories) on success.
 */
@Injectable()
export class ConfigImportService {
  constructor(@Inject(CONFIG_IMPORTER) private readonly importer: ConfigImporter) {}

  async import(body: unknown): Promise<ConfigImportOutcome> {
    const { mode, config } = (body && typeof body === "object" ? body : {}) as {
      mode?: unknown;
      config?: unknown;
    };

    if (mode !== "replace" && mode !== "merge") {
      throw new ApiBadRequestException(["mode must be replace or merge."]);
    }

    const cfg = config as Partial<ExportedConfig> | undefined;
    if (!cfg || cfg.version !== "1" || !Array.isArray(cfg.rules)) {
      throw new ApiBadRequestException([
        "config must be a valid Portier config object with version 1 and a rules array.",
      ]);
    }

    const result = await this.importer.importConfig(cfg as ExportedConfig, mode);
    if (result.errors.length > 0) {
      return { status: 422, body: { errors: result.errors, result } };
    }
    return { status: 200, body: { result, rules: this.importer.listRules().map(toForwardRuleResponse) } };
  }
}
