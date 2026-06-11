import dgram from "node:dgram";
import net from "node:net";

export async function getFreeTcpPort(): Promise<number> {
  const server = net.createServer();
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  await new Promise<void>((resolve, reject) => server.close((error) => (error ? reject(error) : resolve())));
  if (!address || typeof address === "string") {
    throw new Error("Unable to allocate TCP port.");
  }
  return address.port;
}

export async function getFreeUdpPort(): Promise<number> {
  const socket = dgram.createSocket("udp4");
  await new Promise<void>((resolve) => socket.bind(0, "127.0.0.1", resolve));
  const address = socket.address();
  await new Promise<void>((resolve) => socket.close(() => resolve()));
  return address.port;
}

// Test-A stabilization (audits/v1.6-testing-audit-1.md): getFree*Port allocates an
// ephemeral port, closes it, and returns the number — a TOCTOU window in which
// another test (vitest runs files in parallel) or process can grab the port before
// the forwarder binds it, surfacing as a transient EADDRINUSE. isAddrInUse +
// startForwarderOnFreePort close that gap by retrying the bind on a fresh port,
// and only on EADDRINUSE — never the whole test, never any other error.

export function isAddrInUse(err: unknown): boolean {
  return (err as { code?: string } | null)?.code === "EADDRINUSE";
}

interface StartableForwarder {
  start(): Promise<void>;
  stop(): Promise<void> | void;
}

// startForwarderOnFreePort builds a forwarder via mk (bound to a fresh ephemeral
// listen port) and starts it, retrying with a new port if the chosen port was
// taken between allocation and bind. Returns the started forwarder and the listen
// port it successfully bound. A partially-started forwarder is stopped before the
// next attempt so no listener leaks between retries.
export async function startForwarderOnFreePort<F extends StartableForwarder>(
  freePort: () => Promise<number>,
  mk: (listenPort: number) => F
): Promise<{ forwarder: F; listenPort: number }> {
  const maxAttempts = 10;
  let lastErr: unknown;
  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    const listenPort = await freePort();
    const forwarder = mk(listenPort);
    try {
      await forwarder.start();
      return { forwarder, listenPort };
    } catch (err) {
      await Promise.resolve(forwarder.stop()).catch(() => {});
      if (!isAddrInUse(err)) throw err;
      lastErr = err;
    }
  }
  throw new Error(`could not bind a free listen port after ${maxAttempts} attempts: ${String(lastErr)}`);
}

// startTcpServerOnFreePort binds a TCP server to port 0 (the OS picks a free
// port) and returns the LIVE server plus its actual bound port. Because the
// listener is never closed-and-rebound, there is no allocate-close-rebind TOCTOU
// window: the port cannot be stolen between allocation and use. Prefer this over
// getFreeTcpPort()+listen(port) for test target servers.
export async function startTcpServerOnFreePort(
  onConnection?: (socket: net.Socket) => void
): Promise<{ server: net.Server; port: number }> {
  const server = net.createServer(onConnection);
  await new Promise<void>((resolve, reject) => {
    const onError = (error: Error): void => reject(error);
    server.once("error", onError);
    server.listen(0, "127.0.0.1", () => {
      server.off("error", onError);
      resolve();
    });
  });
  const address = server.address();
  if (!address || typeof address === "string") {
    throw new Error("Unable to bind TCP server on a free port.");
  }
  return { server, port: address.port };
}

// bindUdpSocketOnFreePort binds a UDP socket to port 0 and returns the LIVE
// socket plus its actual bound port — same handoff guarantee as
// startTcpServerOnFreePort, for UDP target/echo sockets.
export async function bindUdpSocketOnFreePort(): Promise<{ socket: dgram.Socket; port: number }> {
  const socket = dgram.createSocket("udp4");
  await new Promise<void>((resolve, reject) => {
    const onError = (error: Error): void => reject(error);
    socket.once("error", onError);
    socket.bind(0, "127.0.0.1", () => {
      socket.off("error", onError);
      resolve();
    });
  });
  return { socket, port: socket.address().port };
}

interface RestartableManager {
  startRule(ruleId: string): Promise<unknown>;
  updateRule(ruleId: string, input: { listenPort: number }): Promise<unknown>;
}

// startRuleStable starts ruleId on a ForwardManager and, if the listen bind
// raced with another process (EADDRINUSE), rebinds the rule to a fresh free port
// via updateRule and retries — mirroring the Go service test helper of the same
// name (service/sources/manager/portretry_test.go). It retries ONLY the setup
// bind, ONLY on EADDRINUSE, with a bounded count; any other error is rethrown
// immediately so a genuine start failure is never masked. Use in place of a bare
// manager.startRule when the rule binds an ephemeral listen port from
// getFree*Port. Intentional-conflict tests (which hold a port to force
// EADDRINUSE) must NOT use this helper.
export async function startRuleStable(
  manager: RestartableManager,
  ruleId: string,
  freePort: () => Promise<number>
): Promise<void> {
  const maxAttempts = 10;
  let lastErr: unknown;
  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    try {
      await manager.startRule(ruleId);
      return;
    } catch (err) {
      if (!isAddrInUse(err)) throw err;
      lastErr = err;
      await manager.updateRule(ruleId, { listenPort: await freePort() });
    }
  }
  throw new Error(`could not start ${ruleId} on a free listen port after ${maxAttempts} attempts: ${String(lastErr)}`);
}
