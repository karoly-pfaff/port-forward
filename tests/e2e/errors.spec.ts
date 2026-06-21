import net from "node:net";
import { test, expect } from "@playwright/test";
import { clearAllRules, createRule, startRule } from "./helpers/api.js";

// Reset all rules before each test for isolation.
test.beforeEach(async ({ baseURL }) => {
  await clearAllRules(baseURL!);
});

// Open a TCP connection through the rule's listen port. The forwarder accepts,
// dials the (unreachable) target, fails, and records a tcp.connection.error
// activity event. Resolves once the socket settles either way.
function pokeListener(port: number): Promise<void> {
  return new Promise((resolve) => {
    const socket = net.connect(port, "127.0.0.1");
    const done = (): void => {
      socket.destroy();
      resolve();
    };
    socket.on("connect", () => socket.write("ping"));
    socket.on("error", done);
    socket.on("close", done);
    setTimeout(done, 1_000);
  });
}

// Poll the activity API until a tcp.connection.error error-severity event lands,
// proving the failure is recorded in the log (independent of the UI).
async function waitForConnectionError(baseURL: string): Promise<void> {
  for (let attempt = 0; attempt < 60; attempt += 1) {
    const resp = await fetch(`${baseURL}/api/activity?severity=error&limit=50`);
    if (resp.ok) {
      const body = (await resp.json()) as { events: { type: string }[] };
      if (body.events.some((e) => e.type === "tcp.connection.error")) return;
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error("tcp.connection.error activity event did not appear");
}

// An errored, started rule whose target (127.0.0.1:1) refuses connections.
async function startErroringRule(baseURL: string, name: string, listenPort: number): Promise<void> {
  const { id } = await createRule(baseURL, {
    name,
    protocol: "tcp",
    listenHost: "127.0.0.1",
    listenPort,
    targetHost: "127.0.0.1",
    targetPort: 1, // nothing listens here -> connection refused
  });
  await startRule(baseURL, id);
  await pokeListener(listenPort);
  await waitForConnectionError(baseURL);
}

test("a connection error is recorded and reachable from the Error summary card", async ({ page, baseURL }) => {
  await startErroringRule(baseURL!, "Refused Target", 48060);

  await page.goto("/");
  await expect(page.locator("tbody").getByText("Refused Target")).toBeVisible();

  // The Error summary card opens the Activity Log filtered to errors.
  await page.getByRole("button", { name: /Needs attention/ }).click();

  await expect(page.getByText("Recent forwarding and rule events")).toBeVisible();
  await expect(page.getByRole("combobox", { name: "Filter by severity" })).toHaveValue("error");
  // Match the event message ("TCP connection error: <detail>") — the trailing
  // colon avoids the type-filter <option> labelled "TCP connection error".
  await expect(page.getByText(/TCP connection error:/i).first()).toBeVisible({ timeout: 10_000 });
});

test("a rule's error health badge opens the Activity Log filtered to that rule and errors", async ({ page, baseURL }) => {
  await startErroringRule(baseURL!, "Refused Target", 48061);

  await page.goto("/");
  await expect(page.locator("tbody").getByText("Refused Target")).toBeVisible();

  // The Health column shows an error badge linking to the rule's error activity.
  await page.getByRole("button", { name: "View error activity for Refused Target" }).click();

  await expect(page.getByText("Recent forwarding and rule events")).toBeVisible();
  await expect(page.getByRole("combobox", { name: "Filter by severity" })).toHaveValue("error");
  await expect(page.getByText(/Filtered to rule:/)).toBeVisible();
  // Match the event message ("TCP connection error: <detail>") — the trailing
  // colon avoids the type-filter <option> labelled "TCP connection error".
  await expect(page.getByText(/TCP connection error:/i).first()).toBeVisible({ timeout: 10_000 });
});
