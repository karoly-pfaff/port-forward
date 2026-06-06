import { test, expect } from "@playwright/test";
import { clearAllRules } from "./helpers/api.js";
import { getFreePort } from "./helpers/port.js";
import { startTcpEchoServer, closeTcpServer, sendTcpAndReceive } from "./helpers/network.js";
import { addRuleViaUI, startRuleViaUI, stopRuleViaUI } from "./helpers/ui.js";

// Real TCP forwarding E2E.
// Starts an in-process TCP echo server, creates a forward rule via the UI,
// sends data through the forwarder, and verifies the echo is received.
//
// Set SKIP_TCP_E2E=true to skip when TCP is unreliable in the environment.

test.beforeEach(async ({ baseURL }) => {
  await clearAllRules(baseURL!);
});

test("TCP forwarding: data passes through forwarder to echo server", async ({ page }) => {
  if (process.env.SKIP_TCP_E2E === "true") {
    test.skip(true, "Skipped via SKIP_TCP_E2E=true");
    return;
  }

  const echoPort = await getFreePort();
  const listenPort = await getFreePort();
  const echoServer = await startTcpEchoServer(echoPort);

  try {
    await page.goto("/");

    await addRuleViaUI(page, {
      name: "E2E TCP Echo",
      protocol: "tcp",
      listenHost: "127.0.0.1",
      listenPort,
      targetHost: "127.0.0.1",
      targetPort: echoPort,
    });

    await startRuleViaUI(page, "E2E TCP Echo");

    // Send data through the forwarder and verify echo
    const received = await sendTcpAndReceive("127.0.0.1", listenPort, "e2e-tcp-ping", 5_000);
    expect(received).toContain("e2e-tcp-ping");

    // Verify Running status is still showing after the exchange
    const ruleRow = page.locator("tr", { hasText: "E2E TCP Echo" });
    await expect(ruleRow.getByText("Running")).toBeVisible();

    await stopRuleViaUI(page, "E2E TCP Echo");
  } finally {
    await closeTcpServer(echoServer);
  }
});
