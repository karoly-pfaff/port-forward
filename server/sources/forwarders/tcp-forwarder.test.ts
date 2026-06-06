import net from "node:net";
import { afterEach, describe, expect, it } from "vitest";
import type { ForwardRule } from "@portier/shared";
import { TcpForwarder } from "./tcp-forwarder.js";
import { getFreeTcpPort } from "../test-helpers.js";

const cleanup: Array<() => Promise<void>> = [];

afterEach(async () => {
  await Promise.all(cleanup.splice(0).map((fn) => fn()));
});

describe("TcpForwarder", () => {
  it("forwards TCP data bidirectionally", async () => {
    const targetPort = await getFreeTcpPort();
    const listenPort = await getFreeTcpPort();
    const targetServer = net.createServer((socket) => {
      socket.on("data", (data) => {
        socket.write(Buffer.from(`echo:${data.toString()}`));
      });
    });
    await new Promise<void>((resolve) => targetServer.listen(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetServer.close(() => resolve())));

    const rule: ForwardRule = {
      id: "tcp-test",
      name: "TCP test",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort,
      enabled: true
    };
    const forwarder = new TcpForwarder(rule);
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const response = await new Promise<string>((resolve, reject) => {
      const client = net.createConnection({ host: "127.0.0.1", port: listenPort }, () => {
        client.write("hello");
      });
      client.once("data", (data) => {
        resolve(data.toString());
        client.end();
      });
      client.once("error", reject);
    });

    expect(response).toBe("echo:hello");
    const status = forwarder.getStatus();
    expect(status.bytesIn).toBeGreaterThan(0);
    expect(status.bytesOut).toBeGreaterThan(0);
  });
});
