// src/server/config/schema.ts
// Shape + invariants for config/sla-config.yaml. Fail fast at boot on any
// violation — a malformed config must never half-load.
import { z } from "zod";

export const StatusCategory = z.enum(["PRODUCT_SIDE", "CS_SIDE", "OTHER", "TRANSIENT"]);
export type StatusCategory = z.infer<typeof StatusCategory>;

export const SlaCoverage = z.enum(["24x7", "12x5_ist"]);
export type SlaCoverage = z.infer<typeof SlaCoverage>;

const RepoEntry = z.object({
  owner: z.string().min(1),
  name: z.string().min(1),
  githubProjectId: z.string().min(1),
  projectTitle: z.string().min(1),
  issueQuery: z.string().min(1),
});

const StatusEntry = z.object({
  name: z.string(), // "" is legal: the transient empty status
  category: StatusCategory,
  accruesSla: z.boolean(),
  isTerminal: z.boolean().default(false),
  sortOrder: z.number().int().default(0),
});

const AliasEntry = z.object({
  alias: z.string().min(1),
  canonical: z.string().min(1),
});

const BudgetEntry = z.object({
  priority: z.string().min(1),
  budgetHours: z.number().positive(),
  coverage: SlaCoverage,
  rank: z.number().int(),
});

const Settings = z.object({
  possibleThreshold: z.number().gt(0).lt(1).default(0.75),
  recomputeIntervalMinutes: z.number().int().positive().default(10),
  syncOverlapMinutes: z.number().int().nonnegative().default(15),
  snapshotHourUtc: z.number().int().min(0).max(23).default(0),
  seedSnapshotDays: z.number().int().positive().default(90),
  seedClosedLookbackDays: z.number().int().positive().default(90),
});

export const AppConfigSchema = z
  .object({
    repos: z.array(RepoEntry).min(1),
    taxonomy: z.object({
      statuses: z.array(StatusEntry).min(1),
      aliases: z.array(AliasEntry).default([]),
    }),
    budgets: z.array(BudgetEntry),
    settings: Settings.default({}),
  })
  .superRefine((cfg, ctx) => {
    const dup = (arr: string[], label: string) => {
      const seen = new Set<string>();
      for (const v of arr) {
        if (seen.has(v)) ctx.addIssue({ code: "custom", message: `duplicate ${label}: ${v}` });
        seen.add(v);
      }
    };
    dup(cfg.repos.map((r) => `${r.owner}/${r.name}`), "repo");
    dup(cfg.repos.map((r) => r.githubProjectId), "githubProjectId");
    dup(cfg.taxonomy.statuses.map((s) => s.name), "status name");
    dup(cfg.taxonomy.aliases.map((a) => a.alias), "alias");
    dup(cfg.budgets.map((b) => b.priority), "budget priority");

    const statusNames = new Set(cfg.taxonomy.statuses.map((s) => s.name));
    for (const a of cfg.taxonomy.aliases) {
      if (!statusNames.has(a.canonical)) {
        ctx.addIssue({ code: "custom", message: `alias "${a.alias}" maps to unknown canonical "${a.canonical}"` });
      }
    }
  });

export type AppConfig = z.infer<typeof AppConfigSchema>;
