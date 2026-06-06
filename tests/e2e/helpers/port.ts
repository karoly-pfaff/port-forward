import { createServer } from "net";
import { createSocket } from "dgram";

export function getFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = createServer();
    srv.on("error", reject);
    srv.listen(0, "127.0.0.1", () => {
      const addr = srv.address();
      srv.close((err) => {
        if (err || !addr || typeof addr === "string") {
          reject(err ?? new Error("Could not determine free TCP port"));
        } else {
          resolve(addr.port);
        }
      });
    });
  });
}

/** Alias so callers can be explicit about TCP. */
export const getFreeTcpPort = getFreePort;

export function getFreeUdpPort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const sock = createSocket("udp4");
    sock.on("error", reject);
    sock.bind(0, "127.0.0.1", () => {
      const addr = sock.address();
      sock.close(() => resolve(addr.port));
    });
  });
}
