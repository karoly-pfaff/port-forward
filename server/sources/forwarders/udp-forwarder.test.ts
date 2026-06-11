import dgram from "node:dgram";
import { afterEach, describe, expect, it } from "vitest";
import type { ActivityEventInput, ForwardRule } from "@portier/shared";
import { UdpForwarder } from "./udp-forwarder.js";
import { bindUdpSocketOnFreePort, getFreeUdpPort, startForwarderOnFreePort } from "../test-helpers.js";
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

// startUdpForwarder builds a UdpForwarder on a fresh ephemeral listen port and
// starts it via the Test-A bind-retry helper (retries only on EADDRINUSE, only
// the bind), returning the started forwarder and the listen port it actually
// bound. Replaces the allocate-close-rebind pattern (getFreeUdpPort →
// new UdpForwarder(listenPort) → start) that had a TOCTOU window. Tests that
// must force a bind conflict (see "start() rejects when port is already bound")
// deliberately bypass this helper.
function startUdpForwarder(
  overrides: Partial<ForwardRule>,
  onEvent?: (e: ActivityEventInput) => void,
  timeoutMs?: number,
  reg?: UdpSessionRegistry
): Promise<{ forwarder: UdpForwarder; listenPort: number }> {
  return startForwarderOnFreePort(
    getFreeUdpPort,
    (listenPort) => new UdpForwarder(makeRule({ ...overrides, listenPort }), onEvent, timeoutMs, reg)
  );
}

function closeUdpSocket(socket: dgram.Socket): Promise<void> {
  return new Promise((resolve) => socket.close(() => resolve()));
}

function bindUdpClient(): Promise<dgram.Socket> {
  const socket = dgram.createSocket("udp4");
  return new Promise((resolve) => socket.bind(0, "127.0.0.1", () => resolve(socket)));
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
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    const received = receiveOne(targetSocket);
    const { forwarder, listenPort } = await startUdpForwarder({ targetPort });
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    cleanup.push(() => closeUdpSocket(client));
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
    // Target that echoes back messages
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    targetSocket.on("message", (msg, rinfo) => {
      targetSocket.send(Buffer.from(`echo:${msg.toString()}`), rinfo.port, rinfo.address);
    });

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-multi-client" },
      undefined,
      200 // short timeout for tests
    );
    cleanup.push(() => forwarder.stop());

    // Two independent clients
    const clientA = await bindUdpClient();
    const clientB = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(clientA));
    cleanup.push(() => closeUdpSocket(clientB));

    const responseA = receiveOne(clientA);
    const responseB = receiveOne(clientB);

    sendPacket(clientA, "from-A", listenPort);
    sendPacket(clientB, "from-B", listenPort);

    const [rA, rB] = await Promise.all([responseA, responseB]);
    expect(rA).toBe("echo:from-A");
    expect(rB).toBe("echo:from-B");
  });

  it("tracks active session count in status", async () => {
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-multi-client" },
      undefined,
      100
    );
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

    sendPacket(client, "hello", listenPort);
    // Session appears once the inbound packet is processed.
    await waitUntil(() => forwarder.getStatus().activeUdpSessions === 1);
    expect(forwarder.getStatus().activeUdpSessions).toBe(1);

    // Wait for the idle session (100ms timeout) to expire.
    await waitUntil(() => forwarder.getStatus().activeUdpSessions === 0);
    expect(forwarder.getStatus().activeUdpSessions).toBe(0);
  });

  it("cleans up sessions on stop", async () => {
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-multi-client" },
      undefined,
      5000 // long timeout so session doesn't expire naturally
    );

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

    sendPacket(client, "hello", listenPort);
    await waitUntil(() => forwarder.getStatus().activeUdpSessions === 1);
    expect(forwarder.getStatus().activeUdpSessions).toBe(1);

    await forwarder.stop();
    expect(forwarder.getStatus().activeUdpSessions).toBe(0);
  });
});

describe("UdpForwarder bidirectional-last-client mode", () => {
  it("still works as expected after multi-client refactor", async () => {
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    targetSocket.on("message", (msg, rinfo) => {
      targetSocket.send(Buffer.from(`echo:${msg.toString()}`), rinfo.port, rinfo.address);
    });

    const { forwarder, listenPort } = await startUdpForwarder({
      targetPort,
      udpMode: "bidirectional-last-client"
    });
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

    const response = receiveOne(client);
    sendPacket(client, "hello", listenPort);

    await expect(response).resolves.toBe("echo:hello");
  });
});

describe("UdpForwarder – lifecycle", () => {
  it("start() is a no-op when already running", async () => {
    const { forwarder } = await startUdpForwarder({});
    cleanup.push(() => forwarder.stop());
    await expect(forwarder.start()).resolves.toBeUndefined();
    expect(forwarder.getStatus().running).toBe(true);
  });

  it("start() rejects when port is already bound", async () => {
    // Intentional EADDRINUSE: a live blocker holds the port so the forwarder's
    // bind deterministically fails. This must NOT use the bind-retry helper —
    // it verifies the product surfaces a real port conflict.
    const port = await getFreeUdpPort();
    const blocker = dgram.createSocket("udp4");
    await new Promise<void>((resolve) => blocker.bind(port, "127.0.0.1", resolve));
    cleanup.push(() => closeUdpSocket(blocker));

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
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    let forwardedResolve!: () => void;
    const forwardedPromise = new Promise<void>((r) => { forwardedResolve = r; });

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort },
      (e: ActivityEventInput) => {
        if (e.type === "udp.packet.forwarded") forwardedResolve();
      }
    );
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    cleanup.push(() => closeUdpSocket(client));
    sendPacket(client, "hello", listenPort);

    await forwardedPromise;
  });

  it("rate-limits repeated forwarded events within 1s window", async () => {
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    let forwardedCount = 0;
    let bothReceived!: () => void;
    const bothReceivedPromise = new Promise<void>((r) => { bothReceived = r; });
    let receiveCount = 0;
    targetSocket.on("message", () => {
      if (++receiveCount === 2) bothReceived();
    });

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort },
      (e: ActivityEventInput) => { if (e.type === "udp.packet.forwarded") forwardedCount++; }
    );
    cleanup.push(() => forwarder.stop());

    const client = dgram.createSocket("udp4");
    cleanup.push(() => closeUdpSocket(client));
    sendPacket(client, "p1", listenPort);
    sendPacket(client, "p2", listenPort);

    await bothReceivedPromise;
    // Both packets forwarded; the rate-limit must collapse them to one event.
    // Poll until the (single) forwarded event is observed, then assert no more.
    await waitUntil(() => forwardedCount >= 1);
    expect(forwardedCount).toBe(1);
  });
});

describe("UdpForwarder bidirectional-last-client – stats and events", () => {
  it("tracks bytesOut and packetsOut for reply packets", async () => {
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    targetSocket.on("message", (msg, rinfo) => {
      targetSocket.send(Buffer.from(`reply:${msg.toString()}`), rinfo.port, rinfo.address);
    });

    const { forwarder, listenPort } = await startUdpForwarder({
      targetPort,
      udpMode: "bidirectional-last-client"
    });
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

    const response = receiveOne(client);
    sendPacket(client, "hi", listenPort);
    await response;

    const status = forwarder.getStatus();
    expect(status.bytesOut).toBeGreaterThan(0);
    expect(status.packetsOut).toBe(1);
  });

  it("emits udp.packet.returned event via onEvent", async () => {
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    targetSocket.on("message", (msg, rinfo) => {
      targetSocket.send(Buffer.from("pong"), rinfo.port, rinfo.address);
    });

    let returnedResolve!: () => void;
    const returnedPromise = new Promise<void>((r) => { returnedResolve = r; });

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-last-client" },
      (e: ActivityEventInput) => {
        if (e.type === "udp.packet.returned") returnedResolve();
      }
    );
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

    sendPacket(client, "ping", listenPort);
    await returnedPromise;
  });
});

describe("UdpForwarder bidirectional-multi-client – session management", () => {
  it("resets idle timer on second packet from same client", async () => {
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-multi-client" },
      undefined,
      80 // 80ms idle timeout
    );
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

    sendPacket(client, "first", listenPort);
    await waitUntil(() => forwarder.getStatus().activeUdpSessions === 1);
    expect(forwarder.getStatus().activeUdpSessions).toBe(1);

    // Reset the timer: second packet arrives before the 80ms window expires
    sendPacket(client, "second", listenPort);
    await new Promise<void>((resolve) => setTimeout(resolve, 60));
    // The original timer would have fired by now, but it was reset; session still alive
    expect(forwarder.getStatus().activeUdpSessions).toBe(1);

    // Wait past the reset timeout (80ms from second packet)
    await waitUntil(() => forwarder.getStatus().activeUdpSessions === 0);
    expect(forwarder.getStatus().activeUdpSessions).toBe(0);
  });

  it("emits udp.session.opened and udp.session.closed events via onEvent", async () => {
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    const events: string[] = [];
    let openedResolve!: () => void;
    let closedResolve!: () => void;
    const openedPromise = new Promise<void>((r) => { openedResolve = r; });
    const closedPromise = new Promise<void>((r) => { closedResolve = r; });

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-multi-client" },
      (e: ActivityEventInput) => {
        events.push(e.type);
        if (e.type === "udp.session.opened") openedResolve();
        if (e.type === "udp.session.closed") closedResolve();
      },
      50 // 50ms idle timeout
    );
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

    sendPacket(client, "hello", listenPort);
    await openedPromise;
    expect(events).toContain("udp.session.opened");

    await closedPromise;
    expect(events).toContain("udp.session.closed");
  });

  it("rate-limits udp.packet.forwarded event in multi-client mode", async () => {
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    let forwardedCount = 0;
    let bothArrived!: () => void;
    const bothArrivedPromise = new Promise<void>((r) => { bothArrived = r; });
    let receiveCount = 0;
    targetSocket.on("message", () => {
      if (++receiveCount === 2) bothArrived();
    });

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-multi-client" },
      (e: ActivityEventInput) => { if (e.type === "udp.packet.forwarded") forwardedCount++; },
      5000
    );
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

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
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    const { forwarder, listenPort } = await startUdpForwarder({ targetPort }, undefined, undefined, reg);
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

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
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    const { forwarder, listenPort } = await startUdpForwarder({ targetPort }, undefined, undefined, reg);
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

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
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    targetSocket.on("message", (msg, rinfo) => {
      targetSocket.send(Buffer.from(`reply:${msg.toString()}`), rinfo.port, rinfo.address);
    });

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-last-client" },
      undefined,
      undefined,
      reg
    );
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

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
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-last-client" },
      undefined,
      undefined,
      reg
    );
    cleanup.push(() => forwarder.stop());

    const clientA = await bindUdpClient();
    const clientB = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(clientA));
    cleanup.push(() => closeUdpSocket(clientB));

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
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-multi-client" },
      undefined,
      5000,
      reg
    );
    cleanup.push(() => forwarder.stop());

    const clientA = await bindUdpClient();
    const clientB = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(clientA));
    cleanup.push(() => closeUdpSocket(clientB));

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
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-multi-client" },
      undefined,
      5000,
      reg
    );
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

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
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-multi-client" },
      undefined,
      5000,
      reg
    );

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

    sendPacket(client, "hello", listenPort);
    await waitFor(() => reg.snapshot().length > 0);
    expect(reg.snapshot()).toHaveLength(1);

    await forwarder.stop();
    expect(reg.snapshot()).toHaveLength(0);
  });

  it("multi-client session timeout removes registry entry", async () => {
    const reg = new UdpSessionRegistry();
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-multi-client" },
      undefined,
      50, // 50ms timeout so session expires quickly
      reg
    );
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

    sendPacket(client, "hello", listenPort);
    await waitFor(() => reg.snapshot().length > 0);

    await waitFor(() => reg.snapshot().length === 0, 500);
  });

  it("snapshot contains no payload fields beyond UdpSessionInfo shape", async () => {
    const reg = new UdpSessionRegistry();
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    const { forwarder, listenPort } = await startUdpForwarder({ targetPort }, undefined, undefined, reg);
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

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
    const { forwarder } = await startUdpForwarder({});
    cleanup.push(() => forwarder.stop());

    (forwarder as any).listenSocket.emit("error", new Error("listen error"));

    expect(forwarder.getStatus().lastError).toBe("listen error");
  });

  it("records one-way target socket error in lastError", async () => {
    const { forwarder } = await startUdpForwarder({});
    cleanup.push(() => forwarder.stop());

    (forwarder as any).targetSocket.emit("error", new Error("target error"));

    expect(forwarder.getStatus().lastError).toBe("target error");
  });

  it("records multi-client session target socket error in lastError", async () => {
    const { socket: tgt, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(tgt));

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-multi-client" },
      undefined,
      5000
    );
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));

    sendPacket(client, "hi", listenPort);

    const sessionKey = `127.0.0.1:${client.address().port}`;
    await waitUntil(() => (forwarder as any).sessions.has(sessionKey));
    const session = (forwarder as any).sessions.get(sessionKey);
    expect(session).toBeDefined();

    session.targetSocket.emit("error", new Error("session error"));
    expect(forwarder.getStatus().lastError).toBe("session error");
  });
});

describe("UdpForwarder – emitted event shape (facade regression)", () => {
  // These lock the full ActivityEventInput payload the UdpEventEmitter facade
  // produces (severity, ruleId, ruleName, protocol, message, and details
  // presence/absence) through the public forwarder — the other UDP tests only
  // assert event type + message, which would not catch a dropped/renamed field.

  it("emits udp.packet.error with the full envelope and no details", async () => {
    const events: ActivityEventInput[] = [];
    const { forwarder, listenPort } = await startUdpForwarder({}, (e) => events.push(e));
    cleanup.push(() => forwarder.stop());

    (forwarder as any).targetSocket.send = failingSend;

    const client = dgram.createSocket("udp4");
    cleanup.push(() => closeUdpSocket(client));
    sendPacket(client, "hi", listenPort);

    await waitUntil(() => events.some((e) => e.type === "udp.packet.error"));
    const event = events.find((e) => e.type === "udp.packet.error")!;
    expect(event).toEqual({
      type: "udp.packet.error",
      severity: "error",
      ruleId: "udp-test",
      ruleName: "UDP test",
      protocol: "udp",
      message: "UDP send error: send boom"
    });
    expect(event.details).toBeUndefined();
  });

  it("emits udp.packet.forwarded with the full envelope and details", async () => {
    const { socket: targetSocket, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(targetSocket));

    const events: ActivityEventInput[] = [];
    const { forwarder, listenPort } = await startUdpForwarder({ targetPort }, (e) => events.push(e));
    cleanup.push(() => forwarder.stop());

    // Bind the client first so its source port is known and the message/details
    // are deterministic.
    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));
    const clientPort = (client.address() as { port: number }).port;
    sendPacket(client, "hello", listenPort);

    await waitUntil(() => events.some((e) => e.type === "udp.packet.forwarded"));
    const event = events.find((e) => e.type === "udp.packet.forwarded")!;
    expect(event).toEqual({
      type: "udp.packet.forwarded",
      severity: "info",
      ruleId: "udp-test",
      ruleName: "UDP test",
      protocol: "udp",
      message: `UDP packet forwarded from 127.0.0.1:${clientPort} to 127.0.0.1:${targetPort}.`,
      details: {
        fromAddress: "127.0.0.1",
        fromPort: clientPort,
        targetHost: "127.0.0.1",
        targetPort,
        bytes: 5
      }
    });
  });
});

describe("UdpForwarder – send-callback error handling", () => {
  it("emits udp.packet.error and records lastError when a one-way target send fails", async () => {
    const events: ActivityEventInput[] = [];
    const { forwarder, listenPort } = await startUdpForwarder({}, (e) => events.push(e));
    cleanup.push(() => forwarder.stop());

    // Force the shared one-way target socket's send callback to error
    // (instance-level injection, like the existing socket.emit("error") tests).
    (forwarder as any).targetSocket.send = failingSend;

    const client = dgram.createSocket("udp4");
    cleanup.push(() => closeUdpSocket(client));
    sendPacket(client, "hi", listenPort);

    await waitUntil(() => events.some((e) => e.type === "udp.packet.error"));
    expect(events.find((e) => e.type === "udp.packet.error")?.message).toContain("UDP send error");
    expect(forwarder.getStatus().lastError).toBe("send boom");
  });

  it("emits udp.packet.error when a multi-client session send fails on reuse", async () => {
    const events: ActivityEventInput[] = [];
    // targetPort is only a rule field here (no socket is bound to it), so a bare
    // ephemeral port number is fine and raceless.
    const targetPort = await getFreeUdpPort();
    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-multi-client" },
      (e) => events.push(e),
      5000
    );
    cleanup.push(() => forwarder.stop());

    const client = await bindUdpClient();
    cleanup.push(() => closeUdpSocket(client));
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

    // Echo target replies to whatever the forwarder's target socket sends.
    const { socket: target, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(target));
    target.on("message", (_msg, rinfo) => target.send(Buffer.from("reply"), rinfo.port, rinfo.address));

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-last-client" },
      (e) => events.push(e)
    );
    cleanup.push(() => forwarder.stop());

    // Fail only the return path (listen socket); the inbound forward still works.
    (forwarder as any).listenSocket.send = failingSend;

    const client = dgram.createSocket("udp4");
    cleanup.push(() => closeUdpSocket(client));
    sendPacket(client, "hi", listenPort);

    await waitUntil(() => events.some((e) => e.type === "udp.packet.error"));
    expect(events.find((e) => e.type === "udp.packet.error")?.message).toContain("UDP return error");
  });

  it("emits udp.packet.error when a multi-client return send fails", async () => {
    const events: ActivityEventInput[] = [];

    const { socket: target, port: targetPort } = await bindUdpSocketOnFreePort();
    cleanup.push(() => closeUdpSocket(target));
    target.on("message", (_msg, rinfo) => target.send(Buffer.from("reply"), rinfo.port, rinfo.address));

    const { forwarder, listenPort } = await startUdpForwarder(
      { targetPort, udpMode: "bidirectional-multi-client" },
      (e) => events.push(e),
      5000
    );
    cleanup.push(() => forwarder.stop());

    // Fail the return path (listen socket) before the target reply arrives.
    (forwarder as any).listenSocket.send = failingSend;

    const client = dgram.createSocket("udp4");
    cleanup.push(() => closeUdpSocket(client));
    sendPacket(client, "hi", listenPort);

    await waitUntil(() => events.some((e) => e.type === "udp.packet.error"));
    expect(events.find((e) => e.type === "udp.packet.error")?.message).toContain("multi-client return error");
  });
});
