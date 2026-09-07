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

// One typed function per endpoint (SPEC §6). Port of v3's `api` object in
// src/lib/api.ts, adjusted for the routes living at the root (no /api
// prefix) and POST /sync/runs replacing POST /api/sync/manual (D3).
import { qs, request } from "./client";
import type {
  IssueDetail,
  IssueFilters,
  IssueRow,
  Overview,
  SyncStatus,
  SyncSummary,
  Taxonomy,
  TitleMap,
  Timeseries,
} from "./types";

export const api = {
  listIssues: (filters: IssueFilters = {}) =>
    request<IssueRow[]>(`/issues${qs(filters as Record<string, string | number | undefined>)}`),

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

  getTaxonomy: () => request<Taxonomy>(`/taxonomy`),

  getSyncStatus: () => request<SyncStatus>(`/sync/status`),

  postSyncRuns: () => request<SyncSummary>(`/sync/runs`, { method: "POST" }),
};
