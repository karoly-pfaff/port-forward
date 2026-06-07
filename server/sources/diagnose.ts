import net from "node:net";
import dgram from "node:dgram";
import { promises as dns } from "node:dns";
import type { ForwardRule, DiagnosticCheck, RuleDiagnosticsResult } from "@portier/shared";
import { COMMON_PORTS } from "@portier/shared";

const TIMEOUT_MS = 2000;

export async function diagnoseRule(rule: ForwardRule, isRunning: boolean): Promise<RuleDiagnosticsResult> {
  const checks: DiagnosticCheck[] = [];

  // listen-host
  if (rule.listenHost === "0.0.0.0") {
    checks.push({
      id: "listen-host",
      label: "Listen address",
      status: "warn",
      message: "Listening on 0.0.0.0 exposes this port on all interfaces.",
      details: { listenHost: rule.listenHost }
    });
  } else {
    checks.push({
      id: "listen-host",
      label: "Listen address",
      status: "pass",
      message: `Listen address ${rule.listenHost} is specific.`,
      details: { listenHost: rule.listenHost }
    });
  }

  // lan-exposure
  if (rule.listenHost === "0.0.0.0") {
    checks.push({
      id: "lan-exposure",
      label: "LAN exposure",
      status: "warn",
      message: "Rule is accessible to other devices on the network because it listens on 0.0.0.0.",
      details: { listenHost: rule.listenHost }
    });
  } else {
    checks.push({
      id: "lan-exposure",
      label: "LAN exposure",
      status: "pass",
      message: "Rule is bound to a specific interface and not exposed on the LAN.",
      details: { listenHost: rule.listenHost }
    });
  }

  // privileged-port
  if (rule.listenPort < 1024) {
    checks.push({
      id: "privileged-port",
      label: "Privileged port",
      status: "warn",
      message: `Port ${rule.listenPort} is privileged and may require elevated permissions to bind.`,
      details: { listenPort: rule.listenPort }
    });
  } else {
    checks.push({
      id: "privileged-port",
      label: "Privileged port",
      status: "pass",
      message: `Port ${rule.listenPort} does not require elevated permissions.`,
      details: { listenPort: rule.listenPort }
    });
  }

  // common-port
  const commonPort = COMMON_PORTS.find((p) => p.port === rule.listenPort);
  if (commonPort) {
    checks.push({
      id: "common-port",
      label: "Common port",
      status: "warn",
      message: `Port ${rule.listenPort} is commonly used by ${commonPort.label}.`,
      details: { listenPort: rule.listenPort, service: commonPort.label }
    });
  } else {
    checks.push({
      id: "common-port",
      label: "Common port",
      status: "pass",
      message: `Port ${rule.listenPort} is not a well-known service port.`,
      details: { listenPort: rule.listenPort }
    });
  }

  // listen-bind — skip if rule is already running (it owns the socket)
  if (isRunning) {
    checks.push({
      id: "listen-bind",
      label: "Listen bind",
      status: "pass",
      message: "Rule is currently running; the listen port is already owned by Portier.",
      details: { listenHost: rule.listenHost, listenPort: rule.listenPort, ruleRunning: true }
    });
  } else {
    const bindResult = await tryBind(rule);
    checks.push({
      id: "listen-bind",
      label: "Listen bind",
      status: bindResult.status,
      message: bindResult.message,
      details: { listenHost: rule.listenHost, listenPort: rule.listenPort, ruleRunning: false }
    });
  }

  // target-host resolution
  let targetResolved = false;
  {
    try {
      await dns.lookup(rule.targetHost);
      targetResolved = true;
      checks.push({
        id: "target-host",
        label: "Target hostname",
        status: "pass",
        message: `Target host ${rule.targetHost} resolves successfully.`,
        details: { targetHost: rule.targetHost }
      });
    } catch (err) {
      checks.push({
        id: "target-host",
        label: "Target hostname",
        status: "fail",
        message: `Target host ${rule.targetHost} could not be resolved: ${(err as NodeJS.ErrnoException).code ?? "unknown error"}.`,
        details: { targetHost: rule.targetHost }
      });
    }
  }

  // target-connect — TCP attempts connection; UDP skips (unreliable without protocol response)
  if (rule.protocol === "tcp") {
    if (!targetResolved) {
      checks.push({
        id: "target-connect",
        label: "Target connection",
        status: "skip",
        message: "Skipped because target host resolution failed.",
        details: { targetHost: rule.targetHost, targetPort: rule.targetPort }
      });
    } else {
      const connectResult = await tryTcpConnect(rule.targetHost, rule.targetPort);
      checks.push({
        id: "target-connect",
        label: "Target connection",
        status: connectResult.status,
        message: connectResult.message,
        details: { targetHost: rule.targetHost, targetPort: rule.targetPort }
      });
    }
  } else {
    checks.push({
      id: "target-connect",
      label: "Target connection",
      status: "skip",
      message: "UDP reachability cannot be verified without a protocol-specific response from the target.",
      details: { targetHost: rule.targetHost, targetPort: rule.targetPort, protocol: "udp" }
    });
  }

  // udp-mode (UDP only)
  if (rule.protocol === "udp") {
    const mode = rule.udpMode ?? "one-way";
    if (mode === "bidirectional-last-client") {
      checks.push({
        id: "udp-mode",
        label: "UDP mode",
        status: "warn",
        message:
          "bidirectional-last-client sends replies to the most recently seen client only. " +
          "Suitable for single-client use cases; not reliable for concurrent clients.",
        details: { udpMode: mode }
      });
    } else {
      const modeMsg =
        mode === "one-way"
          ? "one-way: packets flow from clients to the target only; replies are not forwarded."
          : "bidirectional-multi-client: each client gets its own reply path.";
      checks.push({
        id: "udp-mode",
        label: "UDP mode",
        status: "pass",
        message: modeMsg,
        details: { udpMode: mode }
      });
    }
  }

  const summary = buildSummary(checks);

  return {
    ruleId: rule.id,
    ruleName: rule.name,
    protocol: rule.protocol,
    summary,
    checks,
    diagnosedAt: new Date().toISOString()
  };
}

function buildSummary(checks: DiagnosticCheck[]): { status: "pass" | "warn" | "fail"; message: string } {
  const hasFail = checks.some((c) => c.status === "fail");
  const hasWarn = checks.some((c) => c.status === "warn");

  if (hasFail) {
    const failed = checks.filter((c) => c.status === "fail").map((c) => c.label);
    return { status: "fail", message: `${failed.length} check(s) failed: ${failed.join(", ")}.` };
  }
  if (hasWarn) {
    const warned = checks.filter((c) => c.status === "warn").map((c) => c.label);
    return { status: "warn", message: `${warned.length} check(s) need attention: ${warned.join(", ")}.` };
  }
  return { status: "pass", message: "All checks passed." };
}

async function tryBind(rule: ForwardRule): Promise<{ status: "pass" | "fail"; message: string }> {
  return rule.protocol === "tcp"
    ? tryTcpBind(rule.listenHost, rule.listenPort)
    : tryUdpBind(rule.listenHost, rule.listenPort);
}

function tryTcpBind(host: string, port: number): Promise<{ status: "pass" | "fail"; message: string }> {
  return new Promise((resolve) => {
    const server = net.createServer();

    const timer = setTimeout(() => {
      server.close();
      resolve({ status: "fail", message: `TCP bind to ${host}:${port} timed out.` });
    }, TIMEOUT_MS);

    server.listen(port, host, () => {
      clearTimeout(timer);
      server.close(() => {
        resolve({ status: "pass", message: `TCP bind to ${host}:${port} succeeded.` });
      });
    });

    server.on("error", (err) => {
      clearTimeout(timer);
      resolve({ status: "fail", message: `TCP bind to ${host}:${port} failed: ${(err as NodeJS.ErrnoException).code ?? err.message}.` });
    });
  });
}

function tryUdpBind(host: string, port: number): Promise<{ status: "pass" | "fail"; message: string }> {
  return new Promise((resolve) => {
    const socket = dgram.createSocket("udp4");

    const timer = setTimeout(() => {
      try { socket.close(); } catch { /* ignore */ }
      resolve({ status: "fail", message: `UDP bind to ${host}:${port} timed out.` });
    }, TIMEOUT_MS);

    socket.bind(port, host, () => {
      clearTimeout(timer);
      socket.close(() => {
        resolve({ status: "pass", message: `UDP bind to ${host}:${port} succeeded.` });
      });
    });

    socket.on("error", (err) => {
      clearTimeout(timer);
      resolve({ status: "fail", message: `UDP bind to ${host}:${port} failed: ${(err as NodeJS.ErrnoException).code ?? err.message}.` });
    });
  });
}

function tryTcpConnect(host: string, port: number): Promise<{ status: "pass" | "fail"; message: string }> {
  return new Promise((resolve) => {
    const socket = net.createConnection({ host, port, timeout: TIMEOUT_MS });
    let settled = false;

    const settle = (result: { status: "pass" | "fail"; message: string }) => {
      if (settled) return;
      settled = true;
      socket.destroy();
      resolve(result);
    };

    socket.on("connect", () => {
      settle({ status: "pass", message: `TCP connection to ${host}:${port} succeeded.` });
    });

    socket.on("timeout", () => {
      settle({ status: "fail", message: `TCP connection to ${host}:${port} timed out.` });
    });

    socket.on("error", (err) => {
      settle({ status: "fail", message: `TCP connection to ${host}:${port} failed: ${(err as NodeJS.ErrnoException).code ?? err.message}.` });
    });
  });
}
