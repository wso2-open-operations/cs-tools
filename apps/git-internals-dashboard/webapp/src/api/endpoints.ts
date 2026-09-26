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

// One typed function per endpoint. Routes live at the root (no /api
// prefix); a sync run is triggered via POST /sync/runs.
import { qs, request } from "./client";
import type {
  GlobalFilters,
  IssueDetail,
  IssueFilters,
  IssueListResponse,
  Overview,
  SyncStatus,
  SyncSummary,
  Taxonomy,
  Timeseries,
} from "./types";

export const api = {
  listIssues: (filters: IssueFilters = {}) =>
    request<IssueListResponse>(`/issues${qs(filters as Record<string, string | number | string[] | undefined>)}`),

  getIssue: (id: number) => request<IssueDetail>(`/issues/${id}`),

  getTimeseries: (
    params: { days?: number; repo?: string; groupBy?: string; metric?: string; abtTeam?: string } = {},
  ) => request<Timeseries>(`/metrics/timeseries${qs(params as Record<string, string | number | undefined>)}`),

  getOverview: (params: GlobalFilters = {}) =>
    request<Overview>(`/metrics/overview${qs(params as Record<string, string | number | undefined>)}`),

  getTaxonomy: () => request<Taxonomy>(`/taxonomy`),

  getSyncStatus: () => request<SyncStatus>(`/sync/status`),

  postSyncRuns: () => request<SyncSummary>(`/sync/runs`, { method: "POST" }),
};
