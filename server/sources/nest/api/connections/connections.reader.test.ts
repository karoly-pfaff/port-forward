import { describe, expect, it } from "vitest";
import { emptyConnectionsReader } from "./connections.reader.js";

describe("emptyConnectionsReader", () => {
  it("returns empty rules and connection lists (scaffold default — no runtime wired)", () => {
    expect(emptyConnectionsReader.listRules()).toEqual([]);
    expect(emptyConnectionsReader.getLiveTcpConnections()).toEqual([]);
    expect(emptyConnectionsReader.getLiveUdpSessions()).toEqual([]);
  });
});
