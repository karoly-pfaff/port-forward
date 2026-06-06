import { test, expect } from "@playwright/test";
import { clearAllRules } from "./helpers/api.js";
import { getFreePort, getFreeUdpPort } from "./helpers/port.js";
import {
  startTcpEchoServer,
  closeTcpServer,
  sendTcpAndReceive,
  startUdpEchoServer,
  createUdpReceiver,
  createUdpClient,
  sendUdpMessage,
  type UdpEchoServer,
  type UdpReceiver,
  type UdpClient,
} from "./helpers/network.js";
import { addRuleViaUI, startRuleViaUI, stopRuleViaUI } from "./helpers/ui.js";

test.beforeEach(async ({ baseURL }) => {
  await clearAllRules(baseURL!);
});

// ── UDP one-way ───────────────────────────────────────────────────────────────

test("UDP one-way: packet forwarded to receiver, no response expected", async ({ page }) => {
  const listenPort = await getFreeUdpPort();
  let receiver: UdpReceiver | undefined;

  try {
    receiver = await createUdpReceiver();

    await page.goto("/");
    await addRuleViaUI(page, {
      name: "E2E UDP One Way",
      protocol: "udp",
      udpMode: "one-way",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort: receiver.port,
    });

    await startRuleViaUI(page, "E2E UDP One Way");

    // Send packet through the forwarder
    await sendUdpMessage("127.0.0.1", listenPort, "e2e-udp-one-way");

    // Receiver should get the packet
    const received = await receiver.waitForMessage(5_000);
    expect(received).toContain("e2e-udp-one-way");

    const ruleRow = page.locator("tr", { hasText: "E2E UDP One Way" });
    await expect(ruleRow.getByText("Running")).toBeVisible();

    await stopRuleViaUI(page, "E2E UDP One Way");
  } finally {
    await receiver?.close();
  }
});

// ── UDP bidirectional-last-client ─────────────────────────────────────────────

test("UDP bidirectional-last-client: echo returned to the last client", async ({ page }) => {
  const listenPort = await getFreeUdpPort();
  let echoServer: UdpEchoServer | undefined;
  let client: UdpClient | undefined;

  try {
    echoServer = await startUdpEchoServer();
    client = await createUdpClient();

    await page.goto("/");
    await addRuleViaUI(page, {
      name: "E2E UDP Last Client",
      protocol: "udp",
      udpMode: "bidirectional-last-client",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort: echoServer.port,
    });

    await startRuleViaUI(page, "E2E UDP Last Client");

    // Send from the bound client socket; forwarder records the source port as lastClient
    await client.send("127.0.0.1", listenPort, "e2e-udp-last-client");

    // Forwarder → echo server → forwarder → back to client
    const reply = await client.waitForMessage("e2e-udp-last-client", 5_000);
    expect(reply).toContain("e2e-udp-last-client");

    await stopRuleViaUI(page, "E2E UDP Last Client");
  } finally {
    await client?.close();
    await echoServer?.close();
  }
});

// ── UDP bidirectional-multi-client ────────────────────────────────────────────

test("UDP bidirectional-multi-client: two clients each receive their own echo", async ({ page }) => {
  const listenPort = await getFreeUdpPort();
  let echoServer: UdpEchoServer | undefined;
  let client1: UdpClient | undefined;
  let client2: UdpClient | undefined;

  try {
    echoServer = await startUdpEchoServer();
    client1 = await createUdpClient();
    client2 = await createUdpClient();

    await page.goto("/");
    await addRuleViaUI(page, {
      name: "E2E UDP Multi Client",
      protocol: "udp",
      udpMode: "bidirectional-multi-client",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort: echoServer.port,
    });

    await startRuleViaUI(page, "E2E UDP Multi Client");

    // Each client sends a distinct payload; forwarder routes per source port
    await client1.send("127.0.0.1", listenPort, "e2e-client-one");
    await client2.send("127.0.0.1", listenPort, "e2e-client-two");

    const [reply1, reply2] = await Promise.all([
      client1.waitForMessage("e2e-client-one", 5_000),
      client2.waitForMessage("e2e-client-two", 5_000),
    ]);

    expect(reply1).toContain("e2e-client-one");
    expect(reply2).toContain("e2e-client-two");

    // Verify no cross-contamination
    expect(reply1).not.toContain("e2e-client-two");
    expect(reply2).not.toContain("e2e-client-one");

    const ruleRow = page.locator("tr", { hasText: "E2E UDP Multi Client" });
    await expect(ruleRow.getByText("Running")).toBeVisible();

    await stopRuleViaUI(page, "E2E UDP Multi Client");
  } finally {
    await client1?.close();
    await client2?.close();
    await echoServer?.close();
  }
});

// ── Activity assertions ───────────────────────────────────────────────────────

test("UDP activity: forwarding events appear in Activity log", async ({ page, baseURL }) => {
  const listenPort = await getFreeUdpPort();
  let receiver: UdpReceiver | undefined;

  try {
    receiver = await createUdpReceiver();

    await page.goto("/");
    await addRuleViaUI(page, {
      name: "E2E UDP Activity",
      protocol: "udp",
      udpMode: "one-way",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort: receiver.port,
    });

    await startRuleViaUI(page, "E2E UDP Activity");

    // Trigger at least one forwarding event
    await sendUdpMessage("127.0.0.1", listenPort, "e2e-udp-activity");
    await receiver.waitForMessage(5_000);

    await stopRuleViaUI(page, "E2E UDP Activity");

    // Navigate to Activity view
    await page.getByRole("navigation", { name: "Main navigation" })
      .getByRole("button", { name: "Activity" })
      .click();
    await expect(page.getByText("Recent forwarding and rule events")).toBeVisible();

    // Verify via API that at least one UDP event was recorded for this rule
    const resp = await fetch(`${baseURL}/api/activity`);
    const body = (await resp.json()) as { events: Array<{ type: string; ruleName: string }> };
    const udpEvents = body.events.filter(
      (e) => e.ruleName === "E2E UDP Activity" && e.type.startsWith("udp.")
    );
    expect(udpEvents.length).toBeGreaterThan(0);
  } finally {
    await receiver?.close();
  }
});

test("TCP activity: connection events appear in Activity log after forwarding", async ({ page, baseURL }) => {
  const echoPort = await getFreePort();
  const listenPort = await getFreePort();
  const echoServer = await startTcpEchoServer(echoPort);

  try {
    await page.goto("/");
    await addRuleViaUI(page, {
      name: "E2E TCP Activity",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort: echoPort,
    });

    await startRuleViaUI(page, "E2E TCP Activity");

    // Trigger a real TCP connection through the forwarder
    await sendTcpAndReceive("127.0.0.1", listenPort, "e2e-tcp-activity-ping", 5_000);

    await stopRuleViaUI(page, "E2E TCP Activity");

    // Navigate to Activity
    await page.getByRole("navigation", { name: "Main navigation" })
      .getByRole("button", { name: "Activity" })
      .click();
    await expect(page.getByText("Recent forwarding and rule events")).toBeVisible();

    // Verify TCP events via API
    const resp = await fetch(`${baseURL}/api/activity`);
    const body = (await resp.json()) as { events: Array<{ type: string; ruleName: string }> };
    const tcpEvents = body.events.filter(
      (e) => e.ruleName === "E2E TCP Activity" && e.type.startsWith("tcp.")
    );
    expect(tcpEvents.length).toBeGreaterThan(0);
  } finally {
    await closeTcpServer(echoServer);
  }
});
