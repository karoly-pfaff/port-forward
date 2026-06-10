import dgram from "node:dgram";
import { afterEach, describe, expect, it } from "vitest";
import type { ActivityEventInput, ForwardRule } from "@portier/shared";
import { UdpForwarder } from "./udp-forwarder.js";
import { getFreeUdpPort } from "../test-helpers.js";
import { UdpSessionRegistry } from "../connections/udp-session-registry.js";

const cleanup: Array<() => Promise<void>> = [];

afterEach(async () => {
  await Promise.all(cleanup.splice(0).map((fn) => fn()));
});

function makeRule(overrides: Partial<ForwardRule> = {}): ForwardRule {
  return {
    id: "udp-test",
    name: "UDP test",
    protocol: "udp",
    listenHost: "127.0.0.1",
    listenPort: 0,
    targetHost: "127.0.0.1",
    targetPort: 0,
    enabled: true,
    udpMode: "one-way",
    ...overrides
  };
}

function sendPacket(socket: dgram.Socket, message: string, port: number): void {
  socket.send(Buffer.from(message), port, "127.0.0.1");
}

function receiveOne(socket: dgram.Socket): Promise<string> {
  return new Promise((resolve) => {
    socket.once("message", (msg) => resolve(msg.toString()));
  });
}

// Deadline poll (no fixed synchronization sleep). The inner setTimeout is the
// poll interval inside a bounded loop, not a "wait this long" sleep.
async function waitUntil(cond: () => boolean, timeoutMs = 2000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (cond()) return;
    await new Promise((r) => setTimeout(r, 5));
  }
  throw new Error("condition was not met before timeout");
}

type SendStub = (msg: Buffer, port: number, host: string, cb: (err?: Error) => void) => void;
const failingSend: SendStub = (_msg, _port, _host, cb) => cb(new Error("send boom"));

describe("UdpForwarder one-way mode", () => {
  it("forwards UDP packets to target", async () => {
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();
    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetSocket.close(() => resolve())));

    const received = receiveOne(targetSocket);
    const forwarder = new UdpForwarder(makeRule({ listenPort, targetPort }));
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    cleanup.push(() => new Promise((resolve) => client.close(() => resolve())));
    sendPacket(client, "packet", listenPort);

    await expect(received).resolves.toBe("packet");
    const status = forwarder.getStatus();
    expect(status.packetsIn).toBe(1);
    expect(status.bytesIn).toBe(6);
    expect(status.activeUdpSessions).toBeUndefined();
  });
});

describe("UdpForwarder bidirectional-multi-client mode", () => {
  it("routes responses back to the correct originating client", async () => {
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    // Target that echoes back messages
    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetSocket.close(() => resolve())));

    targetSocket.on("message", (msg, rinfo) => {
      targetSocket.send(Buffer.from(`echo:${msg.toString()}`), rinfo.port, rinfo.address);
    });

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-multi-client" }),
      undefined,
      200 // short timeout for tests
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    // Two independent clients
    const clientA = dgram.createSocket("udp4");
    const clientB = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => clientA.bind(0, "127.0.0.1", resolve));
    await new Promise<void>((resolve) => clientB.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => clientA.close(() => resolve())));
    cleanup.push(() => new Promise((resolve) => clientB.close(() => resolve())));

    const responseA = receiveOne(clientA);
    const responseB = receiveOne(clientB);

    sendPacket(clientA, "from-A", listenPort);
    sendPacket(clientB, "from-B", listenPort);

    const [rA, rB] = await Promise.all([responseA, responseB]);
    expect(rA).toBe("echo:from-A");
    expect(rB).toBe("echo:from-B");
  });

  it("tracks active session count in status", async () => {
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetSocket.close(() => resolve())));

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-multi-client" }),
      undefined,
      100
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => client.close(() => resolve())));

    sendPacket(client, "hello", listenPort);
    // Give the forwarder a tick to process
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(forwarder.getStatus().activeUdpSessions).toBe(1);

    // Wait for idle session to expire
    await new Promise((resolve) => setTimeout(resolve, 150));
    expect(forwarder.getStatus().activeUdpSessions).toBe(0);
  });

  it("cleans up sessions on stop", async () => {
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetSocket.close(() => resolve())));

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-multi-client" }),
      undefined,
      5000 // long timeout so session doesn't expire naturally
    );
    await forwarder.start();

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => client.close(() => resolve())));

    sendPacket(client, "hello", listenPort);
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(forwarder.getStatus().activeUdpSessions).toBe(1);

    await forwarder.stop();
    expect(forwarder.getStatus().activeUdpSessions).toBe(0);
  });
});

describe("UdpForwarder bidirectional-last-client mode", () => {
  it("still works as expected after multi-client refactor", async () => {
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetSocket.close(() => resolve())));

    targetSocket.on("message", (msg, rinfo) => {
      targetSocket.send(Buffer.from(`echo:${msg.toString()}`), rinfo.port, rinfo.address);
    });

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-last-client" })
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => client.close(() => resolve())));

    const response = receiveOne(client);
    sendPacket(client, "hello", listenPort);

    await expect(response).resolves.toBe("echo:hello");
  });
});

describe("UdpForwarder – lifecycle", () => {
  it("start() is a no-op when already running", async () => {
    const listenPort = await getFreeUdpPort();
    const forwarder = new UdpForwarder(makeRule({ listenPort }));
    await forwarder.start();
    cleanup.push(() => forwarder.stop());
    await expect(forwarder.start()).resolves.toBeUndefined();
    expect(forwarder.getStatus().running).toBe(true);
  });

  it("start() rejects when port is already bound", async () => {
    const port = await getFreeUdpPort();
    const blocker = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => blocker.bind(port, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => blocker.close(() => resolve())));

    const forwarder = new UdpForwarder(makeRule({ listenPort: port }));
    cleanup.push(() => forwarder.stop().catch(() => {}));
    await expect(forwarder.start()).rejects.toThrow();
    expect(forwarder.getStatus().running).toBe(false);
  });

  it("stop() resolves immediately when forwarder was never started", async () => {
    const forwarder = new UdpForwarder(makeRule());
    await expect(forwarder.stop()).resolves.toBeUndefined();
    expect(forwarder.getStatus().running).toBe(false);
  });
});

describe("UdpForwarder one-way – events", () => {
  it("emits udp.packet.forwarded event via onEvent", async () => {
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => targetSocket.close(() => resolve())));

    let forwardedResolve!: () => void;
    const forwardedPromise = new Promise<void>((r) => { forwardedResolve = r; });

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort }),
      (e: ActivityEventInput) => {
        if (e.type === "udp.packet.forwarded") forwardedResolve();
      }
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    cleanup.push(() => new Promise<void>((resolve) => client.close(() => resolve())));
    sendPacket(client, "hello", listenPort);

    await forwardedPromise;
  });

  it("rate-limits repeated forwarded events within 1s window", async () => {
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => targetSocket.close(() => resolve())));

    let forwardedCount = 0;
    let bothReceived!: () => void;
    const bothReceivedPromise = new Promise<void>((r) => { bothReceived = r; });
    let receiveCount = 0;
    targetSocket.on("message", () => {
      if (++receiveCount === 2) bothReceived();
    });

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort }),
      (e: ActivityEventInput) => { if (e.type === "udp.packet.forwarded") forwardedCount++; }
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    cleanup.push(() => new Promise<void>((resolve) => client.close(() => resolve())));
    sendPacket(client, "p1", listenPort);
    sendPacket(client, "p2", listenPort);

    await bothReceivedPromise;
    // Give send callbacks a tick to complete
    await new Promise<void>((resolve) => setTimeout(resolve, 20));
    expect(forwardedCount).toBe(1);
  });
});

describe("UdpForwarder bidirectional-last-client – stats and events", () => {
  it("tracks bytesOut and packetsOut for reply packets", async () => {
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => targetSocket.close(() => resolve())));

    targetSocket.on("message", (msg, rinfo) => {
      targetSocket.send(Buffer.from(`reply:${msg.toString()}`), rinfo.port, rinfo.address);
    });

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-last-client" })
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => client.close(() => resolve())));

    const response = receiveOne(client);
    sendPacket(client, "hi", listenPort);
    await response;

    const status = forwarder.getStatus();
    expect(status.bytesOut).toBeGreaterThan(0);
    expect(status.packetsOut).toBe(1);
  });

  it("emits udp.packet.returned event via onEvent", async () => {
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => targetSocket.close(() => resolve())));

    targetSocket.on("message", (msg, rinfo) => {
      targetSocket.send(Buffer.from("pong"), rinfo.port, rinfo.address);
    });

    let returnedResolve!: () => void;
    const returnedPromise = new Promise<void>((r) => { returnedResolve = r; });

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-last-client" }),
      (e: ActivityEventInput) => {
        if (e.type === "udp.packet.returned") returnedResolve();
      }
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => client.close(() => resolve())));

    sendPacket(client, "ping", listenPort);
    await returnedPromise;
  });
});

describe("UdpForwarder bidirectional-multi-client – session management", () => {
  it("resets idle timer on second packet from same client", async () => {
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => targetSocket.close(() => resolve())));

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-multi-client" }),
      undefined,
      80 // 80ms idle timeout
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => client.close(() => resolve())));

    sendPacket(client, "first", listenPort);
    await new Promise<void>((resolve) => setTimeout(resolve, 20));
    expect(forwarder.getStatus().activeUdpSessions).toBe(1);

    // Reset the timer: second packet arrives before the 80ms window expires
    sendPacket(client, "second", listenPort);
    await new Promise<void>((resolve) => setTimeout(resolve, 60));
    // The original timer would have fired by now, but it was reset; session still alive
    expect(forwarder.getStatus().activeUdpSessions).toBe(1);

    // Wait past the reset timeout (80ms from second packet)
    await new Promise<void>((resolve) => setTimeout(resolve, 60));
    expect(forwarder.getStatus().activeUdpSessions).toBe(0);
  });

  it("emits udp.session.opened and udp.session.closed events via onEvent", async () => {
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => targetSocket.close(() => resolve())));

    const events: string[] = [];
    let openedResolve!: () => void;
    let closedResolve!: () => void;
    const openedPromise = new Promise<void>((r) => { openedResolve = r; });
    const closedPromise = new Promise<void>((r) => { closedResolve = r; });

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-multi-client" }),
      (e: ActivityEventInput) => {
        events.push(e.type);
        if (e.type === "udp.session.opened") openedResolve();
        if (e.type === "udp.session.closed") closedResolve();
      },
      50 // 50ms idle timeout
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => client.close(() => resolve())));

    sendPacket(client, "hello", listenPort);
    await openedPromise;
    expect(events).toContain("udp.session.opened");

    await closedPromise;
    expect(events).toContain("udp.session.closed");
  });

  it("rate-limits udp.packet.forwarded event in multi-client mode", async () => {
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => targetSocket.close(() => resolve())));

    let forwardedCount = 0;
    let bothArrived!: () => void;
    const bothArrivedPromise = new Promise<void>((r) => { bothArrived = r; });
    let receiveCount = 0;
    targetSocket.on("message", () => {
      if (++receiveCount === 2) bothArrived();
    });

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-multi-client" }),
      (e: ActivityEventInput) => { if (e.type === "udp.packet.forwarded") forwardedCount++; },
      5000
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => client.close(() => resolve())));

    sendPacket(client, "p1", listenPort);
    sendPacket(client, "p2", listenPort);

    await bothArrivedPromise;
    expect(forwardedCount).toBe(1);
  });
});

async function waitFor(condition: () => boolean, timeoutMs = 2000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (!condition()) {
    if (Date.now() >= deadline) throw new Error("Timeout waiting for condition");
    await new Promise<void>((resolve) => setTimeout(resolve, 10));
  }
}

describe("UdpForwarder – live tracking integration", () => {
  it("one-way inbound packet creates a session entry", async () => {
    const reg = new UdpSessionRegistry();
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetSocket.close(() => resolve())));

    const forwarder = new UdpForwarder(makeRule({ listenPort, targetPort }), undefined, undefined, reg);
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => client.close(() => resolve())));

    sendPacket(client, "hello", listenPort);
    await waitFor(() => reg.snapshot().length > 0);

    const snap = reg.snapshot();
    expect(snap).toHaveLength(1);
    expect(snap[0].protocol).toBe("udp");
    expect(snap[0].mode).toBe("one-way");
    expect(snap[0].ruleId).toBe("udp-test");
    expect(snap[0].status).toBe("active");
  });

  it("one-way inbound packet increments bytesIn and packetsIn", async () => {
    const reg = new UdpSessionRegistry();
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetSocket.close(() => resolve())));

    const forwarder = new UdpForwarder(makeRule({ listenPort, targetPort }), undefined, undefined, reg);
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => client.close(() => resolve())));

    sendPacket(client, "hello", listenPort); // 5 bytes
    await waitFor(() => reg.snapshot().length > 0 && reg.snapshot()[0].packetsIn >= 1);
    sendPacket(client, "world", listenPort); // 5 bytes
    await waitFor(() => reg.snapshot()[0].packetsIn >= 2);

    const snap = reg.snapshot()[0];
    expect(snap.packetsIn).toBe(2);
    expect(snap.bytesIn).toBe(10);
    expect(snap.packetsOut).toBe(0); // no return in one-way
    expect(snap.bytesOut).toBe(0);
  });

  it("last-client inbound creates session and response updates bytesOut/packetsOut", async () => {
    const reg = new UdpSessionRegistry();
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetSocket.close(() => resolve())));

    targetSocket.on("message", (msg, rinfo) => {
      targetSocket.send(Buffer.from(`reply:${msg.toString()}`), rinfo.port, rinfo.address);
    });

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-last-client" }),
      undefined,
      undefined,
      reg
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => client.close(() => resolve())));

    const response = receiveOne(client);
    sendPacket(client, "ping", listenPort);
    await response;

    await waitFor(() => {
      const s = reg.snapshot();
      return s.length > 0 && s[0].packetsOut >= 1;
    });

    const snap = reg.snapshot()[0];
    expect(snap.mode).toBe("bidirectional-last-client");
    expect(snap.packetsIn).toBe(1);
    expect(snap.bytesIn).toBe(4);
    expect(snap.packetsOut).toBe(1);
    expect(snap.bytesOut).toBeGreaterThan(0);
  });

  it("last-client new client closes old session and opens new one", async () => {
    const reg = new UdpSessionRegistry();
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetSocket.close(() => resolve())));

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-last-client" }),
      undefined,
      undefined,
      reg
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const clientA = dgram.createSocket("udp4");
    const clientB = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => clientA.bind(0, "127.0.0.1", resolve));
    await new Promise<void>((resolve) => clientB.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => clientA.close(() => resolve())));
    cleanup.push(() => new Promise((resolve) => clientB.close(() => resolve())));

    sendPacket(clientA, "from-A", listenPort);
    await waitFor(() => reg.snapshot().length > 0);
    const idA = reg.snapshot()[0].id;

    sendPacket(clientB, "from-B", listenPort);
    await waitFor(() => {
      const snap = reg.snapshot();
      return snap.length > 0 && snap[0].id !== idA;
    });

    const snap = reg.snapshot();
    expect(snap).toHaveLength(1);
    expect(snap[0].id).not.toBe(idA);
    expect(snap[0].clientPort).toBe(clientB.address().port);
  });

  it("multi-client creates separate sessions per client", async () => {
    const reg = new UdpSessionRegistry();
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetSocket.close(() => resolve())));

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-multi-client" }),
      undefined,
      5000,
      reg
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const clientA = dgram.createSocket("udp4");
    const clientB = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => clientA.bind(0, "127.0.0.1", resolve));
    await new Promise<void>((resolve) => clientB.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => clientA.close(() => resolve())));
    cleanup.push(() => new Promise((resolve) => clientB.close(() => resolve())));

    sendPacket(clientA, "from-A", listenPort);
    sendPacket(clientB, "from-B", listenPort);

    await waitFor(() => reg.snapshot().length >= 2);

    const snap = reg.snapshot();
    expect(snap).toHaveLength(2);
    const modes = snap.map((s) => s.mode);
    expect(modes.every((m) => m === "bidirectional-multi-client")).toBe(true);
    const ports = snap.map((s) => s.clientPort).sort();
    expect(ports[0]).not.toBe(ports[1]);
  });

  it("repeated multi-client packet from same client touches same session", async () => {
    const reg = new UdpSessionRegistry();
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetSocket.close(() => resolve())));

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-multi-client" }),
      undefined,
      5000,
      reg
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => client.close(() => resolve())));

    sendPacket(client, "p1", listenPort);
    await waitFor(() => reg.snapshot().length > 0);
    const firstId = reg.snapshot()[0].id;

    sendPacket(client, "p2", listenPort);
    await waitFor(() => reg.snapshot()[0].packetsIn >= 2);

    expect(reg.snapshot()).toHaveLength(1);
    expect(reg.snapshot()[0].id).toBe(firstId);
    expect(reg.snapshot()[0].packetsIn).toBe(2);
  });

  it("stop() clears live sessions from registry", async () => {
    const reg = new UdpSessionRegistry();
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetSocket.close(() => resolve())));

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-multi-client" }),
      undefined,
      5000,
      reg
    );
    await forwarder.start();

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => client.close(() => resolve())));

    sendPacket(client, "hello", listenPort);
    await waitFor(() => reg.snapshot().length > 0);
    expect(reg.snapshot()).toHaveLength(1);

    await forwarder.stop();
    expect(reg.snapshot()).toHaveLength(0);
  });

  it("multi-client session timeout removes registry entry", async () => {
    const reg = new UdpSessionRegistry();
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetSocket.close(() => resolve())));

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-multi-client" }),
      undefined,
      50, // 50ms timeout so session expires quickly
      reg
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => client.close(() => resolve())));

    sendPacket(client, "hello", listenPort);
    await waitFor(() => reg.snapshot().length > 0);

    await waitFor(() => reg.snapshot().length === 0, 500);
  });

  it("snapshot contains no payload fields beyond UdpSessionInfo shape", async () => {
    const reg = new UdpSessionRegistry();
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const targetSocket = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => targetSocket.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => targetSocket.close(() => resolve())));

    const forwarder = new UdpForwarder(makeRule({ listenPort, targetPort }), undefined, undefined, reg);
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise((resolve) => client.close(() => resolve())));

    sendPacket(client, "hello", listenPort);
    await waitFor(() => reg.snapshot().length > 0);

    const keys = Object.keys(reg.snapshot()[0]).sort();
    expect(keys).toEqual([
      "bytesIn",
      "bytesOut",
      "clientAddress",
      "clientPort",
      "id",
      "idleMs",
      "lastSeenAt",
      "mode",
      "packetsIn",
      "packetsOut",
      "protocol",
      "ruleId",
      "ruleName",
      "startedAt",
      "status",
      "targetAddress",
      "targetPort"
    ]);
  });
});

describe("UdpForwarder – error handling", () => {
  it("records listen socket error in lastError", async () => {
    const listenPort = await getFreeUdpPort();
    const forwarder = new UdpForwarder(makeRule({ listenPort }));
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    (forwarder as any).listenSocket.emit("error", new Error("listen error"));

    expect(forwarder.getStatus().lastError).toBe("listen error");
  });

  it("records one-way target socket error in lastError", async () => {
    const listenPort = await getFreeUdpPort();
    const forwarder = new UdpForwarder(makeRule({ listenPort }));
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    (forwarder as any).targetSocket.emit("error", new Error("target error"));

    expect(forwarder.getStatus().lastError).toBe("target error");
  });

  it("records multi-client session target socket error in lastError", async () => {
    const targetPort = await getFreeUdpPort();
    const listenPort = await getFreeUdpPort();

    const tgt = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => tgt.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => tgt.close(() => resolve())));

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-multi-client" }),
      undefined,
      5000
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((resolve) => client.close(() => resolve())));

    sendPacket(client, "hi", listenPort);
    await new Promise<void>((resolve) => setTimeout(resolve, 20));

    const sessionKey = `127.0.0.1:${client.address().port}`;
    const session = (forwarder as any).sessions.get(sessionKey);
    expect(session).toBeDefined();

    session.targetSocket.emit("error", new Error("session error"));
    expect(forwarder.getStatus().lastError).toBe("session error");
  });
});

describe("UdpForwarder – send-callback error handling", () => {
  it("emits udp.packet.error and records lastError when a one-way target send fails", async () => {
    const events: ActivityEventInput[] = [];
    const listenPort = await getFreeUdpPort();
    const forwarder = new UdpForwarder(makeRule({ listenPort }), (e) => events.push(e));
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    // Force the shared one-way target socket's send callback to error
    // (instance-level injection, like the existing socket.emit("error") tests).
    (forwarder as any).targetSocket.send = failingSend;

    const client = dgram.createSocket("udp4");
    cleanup.push(() => new Promise<void>((r) => client.close(() => r())));
    sendPacket(client, "hi", listenPort);

    await waitUntil(() => events.some((e) => e.type === "udp.packet.error"));
    expect(events.find((e) => e.type === "udp.packet.error")?.message).toContain("UDP send error");
    expect(forwarder.getStatus().lastError).toBe("send boom");
  });

  it("emits udp.packet.error when a multi-client session send fails on reuse", async () => {
    const events: ActivityEventInput[] = [];
    const listenPort = await getFreeUdpPort();
    const targetPort = await getFreeUdpPort();
    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-multi-client" }),
      (e) => events.push(e),
      5000
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => client.bind(0, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((r) => client.close(() => r())));
    const clientPort = client.address().port;

    // First packet creates the session (first send succeeds).
    sendPacket(client, "first", listenPort);
    await waitUntil(() => (forwarder as any).sessions.size >= 1);

    // Inject a send failure on the session socket, then reuse the session.
    const session = (forwarder as any).sessions.get(`127.0.0.1:${clientPort}`);
    expect(session).toBeDefined();
    session.targetSocket.send = failingSend;
    sendPacket(client, "second", listenPort);

    await waitUntil(() => events.some((e) => e.type === "udp.packet.error"));
    expect(events.find((e) => e.type === "udp.packet.error")?.message).toContain("multi-client send error");
  });

  it("emits udp.packet.error when a last-client return send fails", async () => {
    const events: ActivityEventInput[] = [];
    const listenPort = await getFreeUdpPort();
    const targetPort = await getFreeUdpPort();

    // Echo target replies to whatever the forwarder's target socket sends.
    const target = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => target.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((r) => target.close(() => r())));
    target.on("message", (_msg, rinfo) => target.send(Buffer.from("reply"), rinfo.port, rinfo.address));

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-last-client" }),
      (e) => events.push(e)
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    // Fail only the return path (listen socket); the inbound forward still works.
    (forwarder as any).listenSocket.send = failingSend;

    const client = dgram.createSocket("udp4");
    cleanup.push(() => new Promise<void>((r) => client.close(() => r())));
    sendPacket(client, "hi", listenPort);

    await waitUntil(() => events.some((e) => e.type === "udp.packet.error"));
    expect(events.find((e) => e.type === "udp.packet.error")?.message).toContain("UDP return error");
  });

  it("emits udp.packet.error when a multi-client return send fails", async () => {
    const events: ActivityEventInput[] = [];
    const listenPort = await getFreeUdpPort();
    const targetPort = await getFreeUdpPort();

    const target = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => target.bind(targetPort, "127.0.0.1", resolve));
    cleanup.push(() => new Promise<void>((r) => target.close(() => r())));
    target.on("message", (_msg, rinfo) => target.send(Buffer.from("reply"), rinfo.port, rinfo.address));

    const forwarder = new UdpForwarder(
      makeRule({ listenPort, targetPort, udpMode: "bidirectional-multi-client" }),
      (e) => events.push(e),
      5000
    );
    await forwarder.start();
    cleanup.push(() => forwarder.stop());

    // Fail the return path (listen socket) before the target reply arrives.
    (forwarder as any).listenSocket.send = failingSend;

    const client = dgram.createSocket("udp4");
    cleanup.push(() => new Promise<void>((r) => client.close(() => r())));
    sendPacket(client, "hi", listenPort);

    await waitUntil(() => events.some((e) => e.type === "udp.packet.error"));
    expect(events.find((e) => e.type === "udp.packet.error")?.message).toContain("multi-client return error");
  });
});
