import { ApiProperty } from "@nestjs/swagger";
import { IsArray, IsString } from "class-validator";

/**
 * Request body for `POST /api/forwards/reorder`, validated by `ApiValidationPipe`.
 *
 * Express validates inline with `Array.isArray(body.ids) && body.ids.every(id =>
 * typeof id === "string")`, rejecting anything else with the single message
 * `"ids must be an array of strings."`. That check is simple enough to re-express
 * exactly in class-validator (unlike the create/update bodies, whose shared
 * contract validators are delegated to the manager), so this is a REAL validated
 * DTO — `@IsArray` and `@IsString({ each: true })` both carry the identical Express
 * message, and with the pipe's `stopAtFirstError` whichever fails surfaces the
 * exact same `{ errors: ["ids must be an array of strings."] }` envelope (a
 * non-array, a missing `ids`, or an array with a non-string element). An empty
 * array is valid (Express: a no-op reorder).
 */
const IDS_MESSAGE = "ids must be an array of strings.";

export class ReorderForwardRulesBodyDto {
  @ApiProperty({ type: [String], description: "Rule ids in the desired order. Ids not listed keep their relative order at the end." })
  @IsArray({ message: IDS_MESSAGE })
  @IsString({ each: true, message: IDS_MESSAGE })
  ids!: string[];
}
