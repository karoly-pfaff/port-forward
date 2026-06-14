import { describe, expect, it } from "vitest";
import { resolveNestListenOptions } from "./nest-options.js";

describe("resolveNestListenOptions", () => {
  it("defaults host and port when nothing is set", () => {
    expect(resolveNestListenOptions({})).toEqual({ host: "127.0.0.1", port: 47832 });
  });

  it("uses a custom host when provided", () => {
    expect(resolveNestListenOptions({ PORTIER_NEST_HOST: "0.0.0.0" })).toEqual({
      host: "0.0.0.0",
      port: 47832,
    });
  });

  it("uses a valid custom port", () => {
    expect(resolveNestListenOptions({ PORTIER_NEST_PORT: "50123" }).port).toBe(50123);
  });

  it.each(["0", "-5", "abc", "1.5", ""])(
    "falls back to the default port for the invalid value %j",
    (value) => {
      expect(resolveNestListenOptions({ PORTIER_NEST_PORT: value }).port).toBe(47832);
    }
  );
});
