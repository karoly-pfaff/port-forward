/** Host/port the NestJS server listener binds to. */
export interface NestListenOptions {
  host: string;
  port: number;
}

/**
 * The NestJS server binds 127.0.0.1 only (never 0.0.0.0 — the localhost-default
 * security rule) and defaults to 47832 so it cannot collide with the management
 * server's default 47831.
 */
const DEFAULT_NEST_HOST = "127.0.0.1";
const DEFAULT_NEST_PORT = 47832;

/**
 * Resolves the server's listen options from the environment. Pure (takes the
 * env explicitly) so every branch is unit-testable: a missing or non-positive /
 * non-integer `PORTIER_NEST_PORT` falls back to the default port.
 */
export function resolveNestListenOptions(env: NodeJS.ProcessEnv): NestListenOptions {
  const host = env.PORTIER_NEST_HOST ?? DEFAULT_NEST_HOST;
  const rawPort = Number(env.PORTIER_NEST_PORT);
  const port = Number.isInteger(rawPort) && rawPort > 0 ? rawPort : DEFAULT_NEST_PORT;
  return { host, port };
}
