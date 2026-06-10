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
