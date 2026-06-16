import net from "node:net";
import { afterEach, describe, expect, it } from "vitest";
import type { ActivityEventInput, ForwardRule } from "@portier/shared";
import { TcpForwarder } from "./tcp-forwarder.js";
import { TcpConnectionRegistry } from "../connections/tcp-connection-registry.js";
import { getFreeTcpPort, startForwarderOnFreePort } from "../testing/test-helpers.js";

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
    const targetServer = net.createServer((socket) => {
      socket.on("data", (data) => {
        socket.write(Buffer.from(`echo:${data.toString()}`));
      });
    });
    await new Promise<void>((resolve) => targetServer.listen(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetServer.close(() => resolve())));

    const { forwarder, listenPort } = await startForwarderOnFreePort(getFreeTcpPort, (listenPort) =>
      new TcpForwarder({
        id: "tcp-test",
        name: "TCP test",
        protocol: "tcp",
        listenHost: "127.0.0.1",
        listenPort,
        targetHost: "127.0.0.1",
        targetPort,
        enabled: true
      })
    );
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
    const { forwarder } = await startForwarderOnFreePort(getFreeTcpPort, (listenPort) =>
      new TcpForwarder(makeRule({ listenPort, targetPort: listenPort }))
    );
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

    const { forwarder, listenPort } = await startForwarderOnFreePort(getFreeTcpPort, (listenPort) =>
      new TcpForwarder(
        makeRule({ listenPort, targetPort }),
        (e: ActivityEventInput) => {
          events.push(e.type);
          if (e.type === "tcp.connection.opened") openedResolve();
          if (e.type === "tcp.connection.closed") closedResolve();
        }
      )
    );
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

    const events: string[] = [];
    let errorResolve!: () => void;
    const errorPromise = new Promise<void>((r) => { errorResolve = r; });

    const { forwarder, listenPort } = await startForwarderOnFreePort(getFreeTcpPort, (listenPort) =>
      new TcpForwarder(
        makeRule({ listenPort, targetPort: unusedPort }),
        (e: ActivityEventInput) => {
          events.push(e.type);
          if (e.type === "tcp.connection.error") errorResolve();
        }
      )
    );
    cleanup.push(() => forwarder.stop());

    const client = net.createConnection({ host: "127.0.0.1", port: listenPort });
    client.on("error", () => {});
    cleanup.push(() => new Promise<void>((resolve) => { client.destroy(); resolve(); }));

    await errorPromise;
    expect(forwarder.getStatus().lastError).toBeDefined();
    expect(events).toContain("tcp.connection.error");
  });

  it("records post-bind server error in lastError", async () => {
    const { forwarder } = await startForwarderOnFreePort(getFreeTcpPort, (listenPort) =>
      new TcpForwarder(makeRule({ listenPort }))
    );
    cleanup.push(() => forwarder.stop());

    // Simulate an error emitted on the server after it is already listening
    (forwarder as any).server.emit("error", new Error("ETIMEDOUT simulated"));

    expect(forwarder.getStatus().lastError).toBe("ETIMEDOUT simulated");
  });
});

describe("TcpForwarder – stop with active connections", () => {
  it("stop() destroys active connections and resets running state", async () => {
    const targetPort = await getFreeTcpPort();

    const targetServer = net.createServer((socket) => {
      socket.on("data", () => {});
    });
    await new Promise<void>((resolve) => targetServer.listen(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => targetServer.close(() => resolve())));

    let openedResolve!: () => void;
    const openedPromise = new Promise<void>((r) => { openedResolve = r; });

    const { forwarder, listenPort } = await startForwarderOnFreePort(getFreeTcpPort, (listenPort) =>
      new TcpForwarder(
        makeRule({ listenPort, targetPort }),
        (e: ActivityEventInput) => {
          if (e.type === "tcp.connection.opened") openedResolve();
        }
      )
    );

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

describe("TcpForwarder – live tracking integration", () => {
  async function makeEchoTarget(): Promise<{ port: number; close: () => Promise<void> }> {
    const port = await getFreeTcpPort();
    const server = net.createServer((socket) => {
      socket.on("data", (data) => {
        socket.write(Buffer.from(`echo:${data.toString()}`));
      });
    });
    await new Promise<void>((resolve) => server.listen(port, "127.0.0.1", resolve));
    return { port, close: () => new Promise<void>((resolve) => server.close(() => resolve())) };
  }

  it("opening a TCP client creates a live connection entry", async () => {
    const target = await makeEchoTarget();
    cleanup.push(target.close);
    const registry = new TcpConnectionRegistry();

    let openedResolve!: () => void;
    const openedPromise = new Promise<void>((r) => { openedResolve = r; });

    const { forwarder, listenPort } = await startForwarderOnFreePort(getFreeTcpPort, (listenPort) =>
      new TcpForwarder(
        makeRule({ listenPort, targetPort: target.port }),
        (e) => { if (e.type === "tcp.connection.opened") openedResolve(); },
        registry
      )
    );
    cleanup.push(() => forwarder.stop());

    const client = net.createConnection({ host: "127.0.0.1", port: listenPort });
    client.on("error", () => {});
    cleanup.push(() => new Promise<void>((resolve) => { client.destroy(); resolve(); }));

    await openedPromise;
    const snap = registry.snapshot();
    expect(snap).toHaveLength(1);
    expect(snap[0].ruleId).toBe("tcp-test");
    expect(snap[0].protocol).toBe("tcp");
    expect(snap[0].status).toBe("active");
  });

  it("client → target bytes update bytesIn, target → client bytes update bytesOut", async () => {
    const target = await makeEchoTarget();
    cleanup.push(target.close);
    const registry = new TcpConnectionRegistry();

    const { forwarder, listenPort } = await startForwarderOnFreePort(getFreeTcpPort, (listenPort) =>
      new TcpForwarder(
        makeRule({ listenPort, targetPort: target.port }),
        undefined,
        registry
      )
    );
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
    const snap = registry.snapshot();
    expect(snap[0].bytesIn).toBe(5);   // "hello"
    expect(snap[0].bytesOut).toBe(10); // "echo:hello"
  });

  it("connection close removes live connection from registry", async () => {
    const target = await makeEchoTarget();
    cleanup.push(target.close);
    const registry = new TcpConnectionRegistry();

    let closedResolve!: () => void;
    const closedPromise = new Promise<void>((r) => { closedResolve = r; });

    const { forwarder, listenPort } = await startForwarderOnFreePort(getFreeTcpPort, (listenPort) =>
      new TcpForwarder(
        makeRule({ listenPort, targetPort: target.port }),
        (e) => { if (e.type === "tcp.connection.closed") closedResolve(); },
        registry
      )
    );
    cleanup.push(() => forwarder.stop());

    const client = net.createConnection({ host: "127.0.0.1", port: listenPort });
    client.on("error", () => {});

    // wait for connection to appear
    await new Promise<void>((resolve) => {
      const poll = setInterval(() => {
        if (registry.snapshot().length > 0) { clearInterval(poll); resolve(); }
      }, 5);
    });
    expect(registry.snapshot()).toHaveLength(1);

    client.destroy();
    await closedPromise;
    expect(registry.snapshot()).toHaveLength(0);
  });

  it("failed target connection does not leak a live connection entry", async () => {
    const unusedPort = await getFreeTcpPort(); // nothing listening
    const registry = new TcpConnectionRegistry();

    let errorResolve!: () => void;
    const errorPromise = new Promise<void>((r) => { errorResolve = r; });

    const { forwarder, listenPort } = await startForwarderOnFreePort(getFreeTcpPort, (listenPort) =>
      new TcpForwarder(
        makeRule({ listenPort, targetPort: unusedPort }),
        (e) => { if (e.type === "tcp.connection.error") errorResolve(); },
        registry
      )
    );
    cleanup.push(() => forwarder.stop());

    const client = net.createConnection({ host: "127.0.0.1", port: listenPort });
    client.on("error", () => {});
    cleanup.push(() => new Promise<void>((resolve) => { client.destroy(); resolve(); }));

    await errorPromise;
    // after error, connection should be cleaned up
    expect(registry.snapshot()).toHaveLength(0);
  });

  it("stop() clears live connections synchronously", async () => {
    const target = await makeEchoTarget();
    cleanup.push(target.close);
    const registry = new TcpConnectionRegistry();

    let openedResolve!: () => void;
    const openedPromise = new Promise<void>((r) => { openedResolve = r; });

    const { forwarder, listenPort } = await startForwarderOnFreePort(getFreeTcpPort, (listenPort) =>
      new TcpForwarder(
        makeRule({ listenPort, targetPort: target.port }),
        (e) => { if (e.type === "tcp.connection.opened") openedResolve(); },
        registry
      )
    );

    const client = net.createConnection({ host: "127.0.0.1", port: listenPort });
    client.on("error", () => {});
    cleanup.push(() => new Promise<void>((resolve) => { client.destroy(); resolve(); }));

    await openedPromise;
    expect(registry.snapshot()).toHaveLength(1);

    await forwarder.stop();
    expect(registry.snapshot()).toHaveLength(0);
  });

  it("multiple simultaneous connections are tracked separately", async () => {
    const target = await makeEchoTarget();
    cleanup.push(target.close);
    const registry = new TcpConnectionRegistry();

    let opened = 0;
    let openedResolve!: () => void;
    const twoOpened = new Promise<void>((r) => { openedResolve = r; });

    const { forwarder, listenPort } = await startForwarderOnFreePort(getFreeTcpPort, (listenPort) =>
      new TcpForwarder(
        makeRule({ listenPort, targetPort: target.port }),
        (e) => { if (e.type === "tcp.connection.opened" && ++opened === 2) openedResolve(); },
        registry
      )
    );
    cleanup.push(() => forwarder.stop());

    const client1 = net.createConnection({ host: "127.0.0.1", port: listenPort });
    const client2 = net.createConnection({ host: "127.0.0.1", port: listenPort });
    client1.on("error", () => {});
    client2.on("error", () => {});
    cleanup.push(() => new Promise<void>((resolve) => { client1.destroy(); resolve(); }));
    cleanup.push(() => new Promise<void>((resolve) => { client2.destroy(); resolve(); }));

    await twoOpened;
    const snap = registry.snapshot();
    expect(snap).toHaveLength(2);
    expect(snap[0].id).not.toBe(snap[1].id);
  });

  it("snapshot contains no payload fields beyond the defined shape", async () => {
    const target = await makeEchoTarget();
    cleanup.push(target.close);
    const registry = new TcpConnectionRegistry();

    let openedResolve!: () => void;
    const openedPromise = new Promise<void>((r) => { openedResolve = r; });

    const { forwarder, listenPort } = await startForwarderOnFreePort(getFreeTcpPort, (listenPort) =>
      new TcpForwarder(
        makeRule({ listenPort, targetPort: target.port }),
        (e) => { if (e.type === "tcp.connection.opened") openedResolve(); },
        registry
      )
    );
    cleanup.push(() => forwarder.stop());

    const client = net.createConnection({ host: "127.0.0.1", port: listenPort });
    client.on("error", () => {});
    cleanup.push(() => new Promise<void>((resolve) => { client.destroy(); resolve(); }));

    await openedPromise;
    const snap = registry.snapshot();
    const keys = Object.keys(snap[0]).sort();
    const expected = [
      "bytesIn", "bytesOut", "clientAddress", "clientPort",
      "durationMs", "id", "protocol", "ruleId", "ruleName",
      "startedAt", "status", "targetAddress", "targetPort"
    ];
    expect(keys).toEqual(expected);
  });
});
