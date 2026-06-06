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
