// src/server/config/index.ts
// Load-once-at-boot config loader. Path precedence:
//   1. SLA_CONFIG_PATH env (absolute path recommended)
//   2. <project root>/config/sla-config.yaml   (repo default, committed)
// Resolution is process.cwd()-relative, NOT import.meta.url-relative: Next's
// standalone build runs server.js with cwd at the standalone root, and
// outputFileTracingIncludes (next.config.ts) copies config/sla-config.yaml
// there preserving this same relative path. import.meta.url would instead
// resolve into .next/server/... and break only in production.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { parse } from "yaml";
import { AppConfigSchema, type AppConfig } from "./schema";

export * from "./schema";

function configPath(): string {
  const fromEnv = (process.env.SLA_CONFIG_PATH ?? "").trim();
  return fromEnv ? resolve(fromEnv) : resolve(process.cwd(), "config", "sla-config.yaml");
}

let cached: AppConfig | null = null;

/** Parse + validate the YAML config. Memoized; immutable for the process lifetime. */
export function loadConfig(): AppConfig {
  if (cached) return cached;
  const path = configPath();
  let raw: string;
  try {
    raw = readFileSync(path, "utf8");
  } catch (err) {
    throw new Error(`SLA config not found at ${path} — set SLA_CONFIG_PATH or restore the file. (${(err as Error).message})`);
  }
  const parsed: unknown = parse(raw);
  const result = AppConfigSchema.safeParse(parsed);
  if (!result.success) {
    const issues = result.error.issues.map((i) => `${i.path.join(".") || "(root)"}: ${i.message}`).join("\n  ");
    throw new Error(`Invalid SLA config at ${path}:\n  ${issues}`);
  }
  cached = result.data;
  return cached;
}

/** Test hook only — resets memoization so fixtures can be swapped. */
export function __resetConfigCacheForTests(): void {
  cached = null;
}
