import { createServer, connect, type Server } from "net";
import { createSocket, type Socket as UdpSocket } from "dgram";

// ── TCP helpers ───────────────────────────────────────────────────────────────

export function startTcpEchoServer(port: number): Promise<Server> {
  return new Promise((resolve, reject) => {
    const srv = createServer((socket) => {
      socket.pipe(socket);
    });
    srv.on("error", reject);
    srv.listen(port, "127.0.0.1", () => resolve(srv));
  });
}

export function closeTcpServer(srv: Server | undefined): Promise<void> {
  if (!srv) return Promise.resolve();
  return new Promise((resolve) => srv.close(() => resolve()));
}

export function sendTcpAndReceive(
  host: string,
  port: number,
  data: string,
  timeoutMs: number
): Promise<string> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(
      () => reject(new Error(`TCP receive timed out after ${timeoutMs}ms`)),
      timeoutMs
    );
    const client = connect(port, host);
    let received = "";

    client.on("error", (err) => {
      clearTimeout(timer);
      reject(err);
    });

    client.on("data", (chunk) => {
      received += chunk.toString();
      if (received.includes(data.trim())) {
        clearTimeout(timer);
        client.end();
        resolve(received);
      }
    });

    client.on("connect", () => {
      client.write(data);
    });
  });
}

// ── UDP receiver (one-way sink) ───────────────────────────────────────────────

export interface UdpReceiver {
  port: number;
  waitForMessage(timeoutMs: number): Promise<string>;
  close(): Promise<void>;
}

export function createUdpReceiver(): Promise<UdpReceiver> {
  return new Promise((resolve, reject) => {
    const sock = createSocket("udp4");
    const queue: string[] = [];
    const waiters: Array<(msg: string) => void> = [];

    sock.on("error", reject);

    sock.on("message", (msg) => {
      const str = msg.toString();
      const waiter = waiters.shift();
      if (waiter) {
        waiter(str);
      } else {
        queue.push(str);
      }
    });

    sock.bind(0, "127.0.0.1", () => {
      const { port } = sock.address();

      resolve({
        port,
        waitForMessage: (timeoutMs) =>
          new Promise((res, rej) => {
            const queued = queue.shift();
            if (queued !== undefined) {
              res(queued);
              return;
            }
            const timer = setTimeout(
              () => rej(new Error(`UDP receiver timed out after ${timeoutMs}ms`)),
              timeoutMs
            );
            waiters.push((msg) => {
              clearTimeout(timer);
              res(msg);
            });
          }),
        close: () => new Promise((res) => sock.close(() => res())),
      });
    });
  });
}

// ── UDP echo server ───────────────────────────────────────────────────────────

export interface UdpEchoServer {
  port: number;
  close(): Promise<void>;
}

export function startUdpEchoServer(): Promise<UdpEchoServer> {
  return new Promise((resolve, reject) => {
    const sock = createSocket("udp4");

    sock.on("error", reject);

    sock.on("message", (msg, remote) => {
      sock.send(msg, remote.port, remote.address, () => {});
    });

    sock.bind(0, "127.0.0.1", () => {
      const { port } = sock.address();
      resolve({
        port,
        close: () => new Promise((res) => sock.close(() => res())),
      });
    });
  });
}

// ── UDP client (bound socket that can both send and receive) ──────────────────

export interface UdpClient {
  port: number;
  send(targetHost: string, targetPort: number, message: string): Promise<void>;
  waitForMessage(expectedContent: string, timeoutMs: number): Promise<string>;
  close(): Promise<void>;
}

export function createUdpClient(): Promise<UdpClient> {
  return new Promise((resolve, reject) => {
    const sock: UdpSocket = createSocket("udp4");
    const queue: string[] = [];
    const waiters: Map<string, Array<(msg: string) => void>> = new Map();

    sock.on("error", reject);

    sock.on("message", (msg) => {
      const str = msg.toString();
      // Notify any waiter whose expected content matches
      for (const [key, handlers] of waiters) {
        if (str.includes(key)) {
          const handler = handlers.shift();
          if (handler) {
            if (handlers.length === 0) waiters.delete(key);
            handler(str);
            return;
          }
        }
      }
      queue.push(str);
    });

    sock.bind(0, "127.0.0.1", () => {
      const { port } = sock.address();

      resolve({
        port,
        send: (host, p, message) =>
          new Promise((res, rej) => {
            sock.send(Buffer.from(message), p, host, (err) => {
              if (err) rej(err);
              else res();
            });
          }),
        waitForMessage: (expected, timeoutMs) =>
          new Promise((res, rej) => {
            // Check queue first
            const idx = queue.findIndex((m) => m.includes(expected));
            if (idx !== -1) {
              res(queue.splice(idx, 1)[0]);
              return;
            }
            const timer = setTimeout(
              () => rej(new Error(`UDP client timed out waiting for "${expected}" after ${timeoutMs}ms`)),
              timeoutMs
            );
            const handlers = waiters.get(expected) ?? [];
            handlers.push((msg) => {
              clearTimeout(timer);
              res(msg);
            });
            waiters.set(expected, handlers);
          }),
        close: () => new Promise((res) => sock.close(() => res())),
      });
    });
  });
}

/** Fire-and-forget UDP send from a throwaway socket. */
export function sendUdpMessage(host: string, port: number, message: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const sock = createSocket("udp4");
    sock.send(Buffer.from(message), port, host, (err) => {
      sock.close(() => {
        if (err) reject(err);
        else resolve();
      });
    });
  });
}
