import net from "node:net";
import { afterEach, describe, expect, it } from "vitest";
import type { ActivityEventInput, ForwardRule } from "@portier/shared";
import { TcpForwarder } from "./tcp-forwarder.js";
import { getFreeTcpPort } from "../test-helpers.js";

const cleanup: Array<() => Promise<void>> = [];

afterEach(async () => {
  await Promise.all(cleanup.splice(0).map((fn) => fn()));
});

function makeRule(overrides: Partial<ForwardRule> = {}): ForwardRule {
  return {
    id: "tcp-test",
    name: "TCP test",
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort: 0,
    targetHost: "127.0.0.1",
    targetPort: 0,
    enabled: true,
    ...overrides
  };
}

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

describe("TcpForwarder – lifecycle", () => {
  it("start() is a no-op when already running", async () => {
    const listenPort = await getFreeTcpPort();
    const forwarder = new TcpForwarder(makeRule({ listenPort, targetPort: listenPort }));
    await forwarder.start();
    cleanup.push(() => forwarder.stop());
    await expect(forwarder.start()).resolves.toBeUndefined();
    expect(forwarder.getStatus().running).toBe(true);
  });

  it("start() rejects when listen port is already in use", async () => {
    const port = await getFreeTcpPort();
    const blocker = net.createServer();
    await new Promise<void>((resolve) => blocker.listen(port, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => blocker.close(() => resolve())));

    const forwarder = new TcpForwarder(makeRule({ listenPort: port }));
    cleanup.push(() => forwarder.stop().catch(() => {}));
    await expect(forwarder.start()).rejects.toThrow();
    expect(forwarder.getStatus().running).toBe(false);
  });

  it("stop() resolves immediately when forwarder was never started", async () => {
    const forwarder = new TcpForwarder(makeRule());
    await expect(forwarder.stop()).resolves.toBeUndefined();
    expect(forwarder.getStatus().running).toBe(false);
  });
});

describe("TcpForwarder – connection events", () => {
  it("emits tcp.connection.opened and tcp.connection.closed via onEvent", async () => {
    const targetPort = await getFreeTcpPort();
    const listenPort = await getFreeTcpPort();

    const targetServer = net.createServer((socket) => {
      socket.on("data", () => {});
    });
    await new Promise<void>((resolve) => targetServer.listen(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => targetServer.close(() => resolve())));

    const events: string[] = [];
    let openedResolve!: () => void;
    let closedResolve!: () => void;
    const openedPromise = new Promise<void>((r) => { openedResolve = r; });
    const closedPromise = new Promise<void>((r) => { closedResolve = r; });

    const forwarder = new TcpForwarder(
      makeRule({ listenPort, targetPort }),
      (e: ActivityEventInput) => {
        events.push(e.type);
        if (e.type === "tcp.connection.opened") openedResolve();
        if (e.type === "tcp.connection.closed") closedResolve();
      }
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = net.createConnection({ host: "127.0.0.1", port: listenPort });
    client.on("error", () => {});
    cleanup.push(() => new Promise<void>((resolve) => { client.destroy(); resolve(); }));

    await openedPromise;
    expect(forwarder.getStatus().activeConnections).toBe(1);

    client.end();
    await closedPromise;
    expect(forwarder.getStatus().activeConnections).toBe(0);
    // Guard that the closed event fires exactly once (countedClosed dedup)
    expect(events.filter((t) => t === "tcp.connection.closed")).toHaveLength(1);
  });
});

describe("TcpForwarder – error handling", () => {
  it("emits tcp.connection.error and sets lastError when target is unreachable", async () => {
    const unusedPort = await getFreeTcpPort(); // allocated but nothing listening
    const listenPort = await getFreeTcpPort();

    const events: string[] = [];
    let errorResolve!: () => void;
    const errorPromise = new Promise<void>((r) => { errorResolve = r; });

    const forwarder = new TcpForwarder(
      makeRule({ listenPort, targetPort: unusedPort }),
      (e: ActivityEventInput) => {
        events.push(e.type);
        if (e.type === "tcp.connection.error") errorResolve();
      }
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = net.createConnection({ host: "127.0.0.1", port: listenPort });
    client.on("error", () => {});
    cleanup.push(() => new Promise<void>((resolve) => { client.destroy(); resolve(); }));

    await errorPromise;
    expect(forwarder.getStatus().lastError).toBeDefined();
    expect(events).toContain("tcp.connection.error");
  });

  it("records post-bind server error in lastError", async () => {
    const listenPort = await getFreeTcpPort();
    const forwarder = new TcpForwarder(makeRule({ listenPort }));
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    // Simulate an error emitted on the server after it is already listening
    (forwarder as any).server.emit("error", new Error("ETIMEDOUT simulated"));

    expect(forwarder.getStatus().lastError).toBe("ETIMEDOUT simulated");
  });
});

describe("TcpForwarder – stop with active connections", () => {
  it("stop() destroys active connections and resets running state", async () => {
    const targetPort = await getFreeTcpPort();
    const listenPort = await getFreeTcpPort();

    const targetServer = net.createServer((socket) => {
      socket.on("data", () => {});
    });
    await new Promise<void>((resolve) => targetServer.listen(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => targetServer.close(() => resolve())));

    let openedResolve!: () => void;
    const openedPromise = new Promise<void>((r) => { openedResolve = r; });

    const forwarder = new TcpForwarder(
      makeRule({ listenPort, targetPort }),
      (e: ActivityEventInput) => {
        if (e.type === "tcp.connection.opened") openedResolve();
      }
    );
    await forwarder.start();

    const client = net.createConnection({ host: "127.0.0.1", port: listenPort });
    client.on("error", () => {});
    cleanup.push(() => new Promise<void>((resolve) => { client.destroy(); resolve(); }));

    await openedPromise;
    expect(forwarder.getStatus().running).toBe(true);
    expect(forwarder.getStatus().activeConnections).toBe(1);

    await forwarder.stop();
    expect(forwarder.getStatus().running).toBe(false);
    expect(forwarder.getStatus().activeConnections).toBe(0);
  });
});
