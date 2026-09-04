// src/lib/api.ts
"use client";

import { useQuery } from "@tanstack/react-query";
import { getAccessToken } from "./auth-token";

export type SlaState = "NO_SLA" | "OK" | "AT_RISK" | "VIOLATED" | "TERMINAL";

export interface Sla {
  budgetHours: number | null;
  consumedHours: number | null;
  remainingHours: number | null;
  pctConsumed: number | null;
  slaState: SlaState | null;
  slaRunning: boolean | null;
}

export interface IssueRow {
  id: number;
  number: number | null;
  state: string | null;
  url: string | null;
  repo: string | null;
  priority: string | null;
  currentStatus: string | null;
  githubCreatedAt: string | null;
  githubUpdatedAt: string | null;
  sla: Sla | null;
}

export interface StatusEvent {
  id: number;
  previousStatus: string | null;
  status: string | null;
  occurredAt: string;
}

export interface IssueDetail extends IssueRow {
  events: StatusEvent[];
}

export type TitleMap = Record<number, string | null>;

export interface OverviewHeroStat {
  n: number;
  delta: number;
  spark: number[];
}

export interface OverviewProject {
  repoId: number;
  name: string;
  repo: string;
  violated: number;
  atRisk: number;
  cs: number;
  onTrack: number;
  openTracked: number;
  untracked: number;
  worst: boolean;
  allClear: boolean;
}

export interface OverviewPriority {
  key: string;
  code: "P1" | "P2" | "P3" | "P4";
  label: string;
  budgetHours: number;
  violated: number;
  atRisk: number;
  cs: number;
  onTrack: number;
  total: number;
}

export interface MatrixRow {
  key: string;
  code: string;
  cells: { violated: number; atRisk: number; onTrack: number; cs: number };
  total: number;
}

export interface VolumeWeek {
  weekStart: string; // ISO "YYYY-MM-DD" (Monday, UTC)
  total: number;
  byPriority: { P1: number; P2: number; P3: number; P4: number };
}

export interface VolumeProject {
  repoId: number;
  name: string;
  total: number; // sum of the 12 weeks
  weeks: VolumeWeek[];
}

export interface Overview {
  refreshedAt: string;
  filters: { repo: string | null; priority: string | null };
  hero: {
    violated: OverviewHeroStat;
    atRisk: OverviewHeroStat;
    cs: { n: number; byStatus: Array<{ status: string; n: number }> };
    productSide: OverviewHeroStat;
  };
  projects: OverviewProject[];
  priorities: OverviewPriority[];
  matrix: {
    rows: MatrixRow[];
    totals: { violated: number; atRisk: number; onTrack: number; cs: number };
    grandTotal: number;
  };
  volume: VolumeProject[];
}

export interface TimeseriesSeries {
  key: string;
  label: string;
  points: number[];
}

export interface Timeseries {
  window: number;
  metric: "violated" | "at_risk" | "total";
  groupBy: "priority" | "none";
  dates: string[];
  series: TimeseriesSeries[];
}

export interface StatusDefRow {
  name: string;
  category: string;
  accruesSla: boolean;
  isTerminal: boolean;
  sortOrder: number;
}

export interface Taxonomy {
  statuses: StatusDefRow[];
  csStatuses: string[];
}

export interface SyncStatus {
  running: boolean;
  repos: Array<{ repo: string; lastSyncedAt: string | null }>;
  lastRun: {
    kind: string | null;
    status: string | null;
    finishedAt: string | null;
    issuesProcessed: number | null;
    error: string | null;
  } | null;
}

export interface SyncRepoResult {
  repo: string;
  status: "success" | "error";
  issuesProcessed: number;
  eventsInserted: number;
  error?: string;
}

export interface SyncSummary {
  startedAt: string;
  finishedAt: string;
  repos: SyncRepoResult[];
}

export type BucketKey =
  | "all" | "violated" | "at_risk" | "on_track" | "cs" | "tracked" | "untracked" | "attention";
export type OrderKey = "budget_desc" | "updated_desc";

export interface IssueFilters {
  repo?: string;
  priority?: string;
  state?: "OPEN" | "CLOSED";
  sla_state?: SlaState;
  status?: string;
  q?: string; // issue number (digits only)
  limit?: number;
  bucket?: BucketKey;
  order?: OrderKey;
}

// Same-origin requests — no dev proxy needed, frontend and API are one process.
async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = await getAccessToken();
  const res = await fetch(`/api${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  });
  if (!res.ok) {
    const detail = await res.text().catch(() => "");
    throw new Error(`API ${res.status} on ${path}${detail ? `: ${detail}` : ""}`);
  }
  return res.json() as Promise<T>;
}

function qs(params: Record<string, string | number | undefined>): string {
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== "") sp.set(k, String(v));
  }
  const s = sp.toString();
  return s ? `?${s}` : "";
}

export const api = {
  listIssues: (f: IssueFilters = {}) =>
    request<IssueRow[]>(`/issues${qs(f as Record<string, string | number | undefined>)}`),
  getIssue: (id: number) => request<IssueDetail>(`/issues/${id}`),
  getIssueTitles: (ids: number[]) =>
    request<{ titles: TitleMap }>(`/issues/titles`, {
      method: "POST",
      body: JSON.stringify({ ids }),
    }).then((r) => r.titles),
  getTimeseries: (params: { days?: number; repo?: string; groupBy?: string; metric?: string } = {}) =>
    request<Timeseries>(`/metrics/timeseries${qs(params as Record<string, string | number | undefined>)}`),
  getOverview: (params: { repo?: string; priority?: string } = {}) =>
    request<Overview>(`/metrics/overview${qs(params as Record<string, string | number | undefined>)}`),
  getTaxonomy: () => request<Taxonomy>(`/config/taxonomy`),
  getSyncStatus: () => request<SyncStatus>(`/sync/status`),
  postSyncManual: async (): Promise<SyncSummary> => {
    const token = await getAccessToken();
    const res = await fetch(`/api/sync/manual`, {
      method: "POST",
      // No body on this request — omit Content-Type so the route's JSON
      // body parser doesn't reject it as an empty JSON payload.
      headers: {
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
    });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(body.message ?? body.error ?? `API ${res.status} on /sync/manual`);
    }
    return body as SyncSummary;
  },
};

// ── TanStack Query hooks ─────────────────────────────────────────────────────

export function useOverview(repo?: string, priority?: string) {
  return useQuery({
    queryKey: ["overview", repo, priority],
    queryFn: () => api.getOverview({ repo, priority }),
    refetchInterval: 60_000,
  });
}

// Taxonomy changes only on reseed — cache indefinitely, no background refetch.
export function useTaxonomy() {
  return useQuery({
    queryKey: ["taxonomy"],
    queryFn: () => api.getTaxonomy(),
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });
}

/** Returns `false` for every status while taxonomy is loading (no layout shift). */
export function makeIsCsStatus(csStatuses: string[] | undefined) {
  return (status: string | null | undefined): boolean =>
    csStatuses != null && csStatuses.includes(status ?? "");
}

export function useTimeseries(params: { repo?: string; metric?: string; days?: number; groupBy?: string }) {
  return useQuery({
    queryKey: ["timeseries", params],
    queryFn: () => api.getTimeseries(params),
  });
}

export function useIssues(filters: IssueFilters) {
  return useQuery({
    queryKey: ["issues", filters],
    queryFn: () => api.listIssues(filters),
  });
}

/**
 * Runtime title resolution for a set of issue ids. Titles never touch the DB;
 * they are fetched through the API's GitHub proxy at render time.
 * The stable sorted key makes reordered lists hit the same cache entry.
 */
export function useIssueTitles(ids: number[]) {
  const key = [...ids].sort((a, b) => a - b).join(",");
  return useQuery({
    queryKey: ["issue-titles", key],
    queryFn: () => api.getIssueTitles(ids),
    enabled: ids.length > 0,
    staleTime: 10 * 60_000, // matches the server-side 15-min cache order of magnitude
    retry: 1,
  });
}

export function useSyncStatus() {
  return useQuery({
    queryKey: ["sync-status"],
    queryFn: () => api.getSyncStatus(),
    refetchInterval: 60_000,
  });
}

export function useIssueTimeline(id: number, enabled: boolean) {
  return useQuery({
    queryKey: ["issue", id],
    queryFn: () => api.getIssue(id),
    enabled,
  });
}
