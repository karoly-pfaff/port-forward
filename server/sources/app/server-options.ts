import { dirname, resolve } from "node:path";
import { PORTIER_DEFAULT_HOST, PORTIER_DEFAULT_PORT } from "@portier/shared";

export const DEFAULT_MANAGEMENT_HOST = PORTIER_DEFAULT_HOST;
export const DEFAULT_MANAGEMENT_PORT = PORTIER_DEFAULT_PORT;

export interface ServerOptions {
  service: boolean;
  configPath: string;
  host: string;
  port: number;
  staticClientDir: string;
  staticClientDirSource: "cli" | "env" | "default";
}

export function resolveServerOptions(
  argv = process.argv.slice(2),
  env: NodeJS.ProcessEnv = process.env,
  cwd = process.cwd(),
  executablePath = process.execPath
): ServerOptions {
  const cli = parseCliArgs(argv);
  const portValue = cli.port ?? env.PORTIER_PORT;
  const port = parsePort(portValue);
  const staticDir = resolveStaticClientDir(cli.staticClientDir, env.PORTIER_STATIC_DIR, cwd, executablePath);

  return {
    service: cli.service,
    configPath: resolve(cwd, cli.configPath ?? env.PORTIER_CONFIG ?? "data/forwards.json"),
    host: cli.host ?? env.PORTIER_HOST ?? DEFAULT_MANAGEMENT_HOST,
    port: port ?? DEFAULT_MANAGEMENT_PORT,
    staticClientDir: staticDir.path,
    staticClientDirSource: staticDir.source
  };
}

interface CliOptions {
  service: boolean;
  configPath?: string;
  host?: string;
  port?: string;
  staticClientDir?: string;
}

function parseCliArgs(argv: string[]): CliOptions {
  const options: CliOptions = { service: false };

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    switch (arg) {
      case "--service":
        options.service = true;
        break;
      case "--config":
        options.configPath = requireValue(argv, index, arg);
        index += 1;
        break;
      case "--host":
        options.host = requireValue(argv, index, arg);
        index += 1;
        break;
      case "--port":
        options.port = requireValue(argv, index, arg);
        index += 1;
        break;
      case "--static-dir":
        options.staticClientDir = requireValue(argv, index, arg);
        index += 1;
        break;
      default:
        throw new Error(`Unknown CLI argument: ${arg}`);
    }
  }

  return options;
}

function requireValue(argv: string[], index: number, flag: string): string {
  const value = argv[index + 1];
  if (!value || value.startsWith("--")) {
    throw new Error(`${flag} requires a value.`);
  }
  return value;
}

function parsePort(value: string | undefined): number | undefined {
  if (value === undefined) {
    return undefined;
  }

  const port = Number(value);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error(`Invalid port: ${value}`);
  }

  return port;
}

function resolveStaticClientDir(
  cliStaticDir: string | undefined,
  envStaticDir: string | undefined,
  cwd: string,
  executablePath: string
): { path: string; source: ServerOptions["staticClientDirSource"] } {
  if (cliStaticDir) {
    return { path: resolve(cwd, cliStaticDir), source: "cli" };
  }

  if (envStaticDir) {
    return { path: resolve(cwd, envStaticDir), source: "env" };
  }

  const executableRelative = resolve(dirname(executablePath), "client", "build");
  const workingDirectoryRelative = resolve(cwd, "client", "build");

  const executableName = executablePath.toLowerCase();

  return {
    path: executableName.endsWith("node.exe") || executableName.endsWith("node") ? workingDirectoryRelative : executableRelative,
    source: "default"
  };
}
