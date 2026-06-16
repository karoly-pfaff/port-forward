import { describe, expect, it } from "vitest";
import { ApiBadRequestException } from "../common/api-errors.js";
import { ApiValidationPipe } from "../common/api-validation.pipe.js";
import { ReorderForwardRulesBodyDto } from "./reorder-forward-rules.body.dto.js";

const pipe = new ApiValidationPipe(ReorderForwardRulesBodyDto);
const MESSAGE = "ids must be an array of strings.";

async function expectRejected(body: unknown): Promise<void> {
  await expect(pipe.transform(body)).rejects.toBeInstanceOf(ApiBadRequestException);
  try {
    await pipe.transform(body);
  } catch (error) {
    expect((error as ApiBadRequestException).getResponse()).toEqual({ errors: [MESSAGE] });
  }
}

describe("ReorderForwardRulesBodyDto via ApiValidationPipe", () => {
  it("accepts an array of string ids", async () => {
    const result = await pipe.transform({ ids: ["a", "b", "c"] });
    expect(result.ids).toEqual(["a", "b", "c"]);
  });

  it("accepts an empty array (a no-op reorder)", async () => {
    const result = await pipe.transform({ ids: [] });
    expect(result.ids).toEqual([]);
  });

  it("strips unknown extra fields (whitelist), ignoring them", async () => {
    const result = (await pipe.transform({ ids: ["a"], bogus: 1 })) as unknown as Record<string, unknown>;
    expect(result.ids).toEqual(["a"]);
    expect(result).not.toHaveProperty("bogus");
  });

  it("rejects a missing ids with the exact contract message", async () => {
    await expectRejected({});
  });

  it("rejects a non-array ids", async () => {
    await expectRejected({ ids: "notarray" });
  });

  it("rejects an array containing a non-string element", async () => {
    await expectRejected({ ids: ["ok", 1] });
  });
});
