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

import {
  useMutation,
  useQueryClient,
  type QueryClient,
  type UseMutationResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BePatchIncidentResponse,
  BeUpdateIncidentPayload,
} from "@api/backend/types";
import { CHILD_INCIDENTS_QUERY_KEY } from "@features/csm-operations/api/useSearchChildIncidents";

/**
 * entity-service's incident flows (the report task on In Progress; the report,
 * alert tasks and problem on Resolved) run in a background drain every 5s,
 * after this PATCH has returned. The detail and its tasks are fetched again
 * once that has had time to happen, so the new task or problem shows on the
 * Related tab without a reload.
 */
const INCIDENT_FLOW_SETTLE_MS = 7_000;
const STATES_WITH_FLOWS = new Set(["IN_PROGRESS", "RESOLVED"]);

export interface PatchIncidentInput {
  id: string;
  patch: BeUpdateIncidentPayload;
}

/**
 * Update an incident via `PATCH /incidents/{id}`. The BE requires at least
 * one field in `patch`. On success the detail and any cached list/search are
 * invalidated so the new values show (see `refreshIncidentRecords`).
 */
export function usePatchIncident(): UseMutationResult<
  BePatchIncidentResponse,
  Error,
  PatchIncidentInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BePatchIncidentResponse, Error, PatchIncidentInput>({
    mutationFn: (input): Promise<BePatchIncidentResponse> =>
      api.patch<BeUpdateIncidentPayload, BePatchIncidentResponse>(
        `/incidents/${encodeURIComponent(input.id)}`,
        input.patch,
      ),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.INCIDENTS],
      });
      refreshIncidentRecords(queryClient, [variables]);
    },
  });
}

/**
 * What an incident's PATCH makes stale, beside the list: its detail, its
 * tasks (a move to Closed or Cancelled closes them), its activity feed and
 * its notes (the update's own, and the flows'), and once more after the
 * background flows of In Progress / Resolved have had time to run. Shared by
 * the single-record edit and the list's bulk edit, so both refresh the same.
 *
 * A move to Resolved also resolves the incident's open child incidents (and
 * theirs) in the same update, each with notes and activity of its own. Their
 * ids are not known here, so every incident's detail, activity and notes are
 * refreshed, and the child lists with them. A bulk run passes all its updates
 * at once: those prefix-wide refreshes then happen once for the run (and once
 * more after the flows, on one timer), not once per incident, since each one
 * refetches every incident page in the cache.
 */
export function refreshIncidentRecords(
  queryClient: QueryClient,
  updates: ReadonlyArray<{ id: string; patch: BeUpdateIncidentPayload }>,
): void {
  if (updates.length === 0) return;
  const resolvesChildren = updates.some((u) => u.patch.state === "RESOLVED");
  const withFlows = updates.filter((u) => u.patch.state && STATES_WITH_FLOWS.has(u.patch.state));
  const invalidate = (queryKey: readonly unknown[]): void => {
    void queryClient.invalidateQueries({ queryKey });
  };
  // Detail and notes: every incident's when a child may have been resolved, else each one's own.
  const refreshDetailAndNotes = (ids: string[]): void => {
    if (resolvesChildren) {
      invalidate([ApiQueryKeys.INCIDENT_DETAILS]);
      invalidate([ApiQueryKeys.INCIDENT_COMMENTS]);
      return;
    }
    for (const id of ids) {
      invalidate([ApiQueryKeys.INCIDENT_DETAILS, id]);
      invalidate([ApiQueryKeys.INCIDENT_COMMENTS, id]);
    }
  };
  const ids = updates.map((u) => u.id);
  refreshDetailAndNotes(ids);
  for (const id of ids) invalidate([ApiQueryKeys.CSM_INCIDENT_TASKS, id]);
  // The update writes a field-change entry to the activity feed.
  if (resolvesChildren) invalidate([ApiQueryKeys.INCIDENT_ACTIVITIES]);
  else for (const id of ids) invalidate([ApiQueryKeys.INCIDENT_ACTIVITIES, id]);
  if (withFlows.length === 0) return;
  setTimeout(() => {
    // The flows add tasks and work notes of their own.
    const flowIds = withFlows.map((u) => u.id);
    refreshDetailAndNotes(flowIds);
    for (const id of flowIds) invalidate([ApiQueryKeys.CSM_INCIDENT_TASKS, id]);
    if (resolvesChildren) {
      // The children's activity rows are written after the commit.
      invalidate([ApiQueryKeys.INCIDENT_ACTIVITIES]);
      invalidate(CHILD_INCIDENTS_QUERY_KEY);
    }
  }, INCIDENT_FLOW_SETTLE_MS);
}
