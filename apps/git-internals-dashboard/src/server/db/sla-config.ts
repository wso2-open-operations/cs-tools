// src/server/db/sla-config.ts
// One builder for the runtime SLA config, derived from the in-process
// @/server/config loader.
import { loadConfig, type AppConfig } from "@/server/config";
import { buildStatusNormalizer } from "./normalize";
import type { SlaConfig, SlaCoverage } from "./sla";

export interface RuntimeSlaConfig {
  cfg: SlaConfig;
  normalize: (status: string | null) => string | null;
  knownNames: Set<string>;
}

export function slaConfigFromFile(app: AppConfig = loadConfig()): RuntimeSlaConfig {
  const budgets = new Map<string, number>();
  const coverage = new Map<string, SlaCoverage>();
  for (const b of app.budgets) {
    budgets.set(b.priority, b.budgetHours);
    coverage.set(b.priority, b.coverage);
  }

  const accrueSet = new Set(app.taxonomy.statuses.filter((s) => s.accruesSla).map((s) => s.name));
  const terminalSet = new Set(app.taxonomy.statuses.filter((s) => s.isTerminal).map((s) => s.name));
  const knownNames = new Set(app.taxonomy.statuses.map((s) => s.name));

  const cfg: SlaConfig = {
    budgets,
    coverage,
    accrues: (status) => status != null && accrueSet.has(status),
    isTerminal: (status) => status != null && terminalSet.has(status),
    possibleThreshold: app.settings.possibleThreshold,
  };

  return {
    cfg,
    normalize: buildStatusNormalizer(app.taxonomy.aliases),
    knownNames,
  };
}

/** The shape of one `repos` entry in the config file — reused wherever code imports `RepoConfig`. */
export type ConfigRepo = AppConfig["repos"][number];
