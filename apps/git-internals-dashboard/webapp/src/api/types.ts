// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Port of v3's src/lib/api.ts types, adjusted for D3's renames (camelCase
// `slaState` query param instead of `sla_state`; `POST /sync/runs` instead of
// `POST /api/sync/manual`). Wire shapes of successful responses are
// otherwise frozen 1:1 from v3 (SPEC §6).
export type SlaState = "NO_SLA" | "OK" | "AT_RISK" | "VIOLATED" | "TERMINAL";

export interface Sla {
  budgetHours: number | null;
  consumedHours: number | null;
  remainingHours: number | null;
  pctConsumed: number | null;
  slaState: SlaState | null;
  slaRunning: boolean | null;
}

// PRIVACY: rows carry no title, assignees, opener, or labels. Titles are
// resolved separately at render time via useIssueTitles().
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
  | "all"
  | "violated"
  | "at_risk"
  | "on_track"
  | "cs"
  | "tracked"
  | "untracked"
  | "attention";
export type OrderKey = "budget_desc" | "updated_desc";

export interface IssueFilters {
  repo?: string;
  priority?: string;
  state?: "OPEN" | "CLOSED";
  slaState?: SlaState;
  status?: string;
  q?: string; // issue number (digits only)
  limit?: number;
  bucket?: BucketKey;
  order?: OrderKey;
}
