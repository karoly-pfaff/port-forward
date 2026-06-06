import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname } from "node:path";
import type { ForwardRule } from "@portier/shared";
import { validateForwardRule } from "@portier/shared";

export class ConfigStore {
  constructor(private readonly filePath: string) {}

  async load(): Promise<ForwardRule[]> {
    try {
      const raw = await readFile(this.filePath, "utf8");
      const parsed = JSON.parse(raw) as unknown;
      if (!Array.isArray(parsed)) {
        throw new Error("Config file must contain an array of forward rules.");
      }

      return parsed.map((item, index) => {
        const result = validateForwardRule(item);
        if (!result.valid || !result.value?.id) {
          throw new Error(`Invalid rule at index ${index}: ${result.errors.join(" ")}`);
        }
        return result.value as ForwardRule;
      });
    } catch (error) {
      if (isNodeError(error) && error.code === "ENOENT") {
        return [];
      }
      throw error;
    }
  }

  async save(rules: ForwardRule[]): Promise<void> {
    await mkdir(dirname(this.filePath), { recursive: true });
    await writeFile(this.filePath, `${JSON.stringify(rules, null, 2)}\n`, "utf8");
  }
}

function isNodeError(error: unknown): error is NodeJS.ErrnoException {
  return error instanceof Error && "code" in error;
}
