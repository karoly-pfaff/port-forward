export type LogLevel = "info" | "warn" | "error";

export interface Logger {
  info(event: string, message: string, fields?: Record<string, unknown>): void;
  warn(event: string, message: string, fields?: Record<string, unknown>): void;
  error(event: string, message: string, fields?: Record<string, unknown>): void;
}

export function createConsoleLogger(): Logger {
  return {
    info: (event, message, fields) => writeLog("info", event, message, fields),
    warn: (event, message, fields) => writeLog("warn", event, message, fields),
    error: (event, message, fields) => writeLog("error", event, message, fields)
  };
}

export function errorFields(error: unknown): Record<string, unknown> {
  if (error instanceof Error) {
    return {
      errorName: error.name,
      errorMessage: error.message,
      stack: error.stack
    };
  }

  return { errorMessage: String(error) };
}

function writeLog(level: LogLevel, event: string, message: string, fields: Record<string, unknown> = {}): void {
  const entry = {
    timestamp: new Date().toISOString(),
    level,
    event,
    message,
    ...fields
  };
  const line = JSON.stringify(entry);
  if (level === "error") {
    console.error(line);
    return;
  }
  console.log(line);
}
