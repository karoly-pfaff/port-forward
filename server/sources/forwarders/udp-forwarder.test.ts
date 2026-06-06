import dgram from "node:dgram";
import { afterEach, describe, expect, it } from "vitest";
import type { ForwardRule } from "@portier/shared";
import { UdpForwarder } from "./udp-forwarder.js";
import { getFreeUdpPort } from "../test-helpers.js";

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
