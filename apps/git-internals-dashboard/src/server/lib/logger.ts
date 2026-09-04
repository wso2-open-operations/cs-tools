// src/server/lib/logger.ts
// Minimal console-based logger matching the MinimalLogger shape the
// recompute job and incremental sync expect (previously Fastify's pino
// instance). Structured-enough for Choreo's log aggregation without pulling
// in a logging framework for a handful of call sites.
import type { MinimalLogger } from "@/server/db/sync/incremental";

function line(level: string, args: unknown[]): void {
  const time = new Date().toISOString();
  console.log(JSON.stringify({ level, time, args }));
}

export const logger: MinimalLogger = {
  info: (...args) => line("info", args),
  warn: (...args) => line("warn", args),
  error: (...args) => line("error", args),
};
