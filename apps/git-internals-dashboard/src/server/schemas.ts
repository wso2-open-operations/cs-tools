// src/server/schemas.ts
import { z } from "zod";

// "owner/name" — both segments limited to GitHub's allowed charset.
const repoRef = z
  .string()
  .regex(/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/, "repo must be owner/name")
  .optional();

export const issueListQuery = z.object({
  repo: repoRef,
  priority: z.string().max(50).optional(),
  state: z.enum(["OPEN", "CLOSED"]).optional(),
  sla_state: z.enum(["NO_SLA", "OK", "AT_RISK", "VIOLATED", "TERMINAL"]).optional(),
  status: z.string().max(50).optional(),
  q: z.string().regex(/^\d{1,10}$/, "q must be an issue number").optional(),
  limit: z.coerce.number().int().min(1).max(500).default(200),
  bucket: z
    .enum(["all", "violated", "at_risk", "on_track", "cs", "tracked", "untracked", "attention"])
    .optional(),
  order: z.enum(["budget_desc", "updated_desc"]).default("updated_desc"),
});
export type IssueListQuery = z.infer<typeof issueListQuery>;

export const titlesBody = z.object({
  ids: z.array(z.number().int().positive()).min(1).max(200),
});

export const timeseriesQuery = z.object({
  repo: repoRef,
  days: z.coerce.number().int().min(7).max(365).default(30),
  groupBy: z.enum(["priority", "none"]).default("priority"),
  metric: z.enum(["violated", "at_risk", "total"]).default("violated"),
});

export const overviewQuery = z.object({
  repo: repoRef,
  priority: z.string().max(50).optional(),
});

export const idParam = z.object({ id: z.coerce.number().int() });
