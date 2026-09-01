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
  type UseMutationResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeHandOffIncidentPayload,
  BeHandOffIncidentResponse,
} from "@api/backend/types";

export interface HandOffIncidentInput {
  incidentId: string;
  payload: BeHandOffIncidentPayload;
}

/**
 * Hand an incident off to its specialist group via
 * `POST /incidents/{id}/specialist-handoffs`. Deliberately not gated in the
 * frontend by the incident's current service/state — the backend's `409`
 * (wrong service, not In Progress, or already with the specialist group)
 * is the source of truth for eligibility; showing the action unconditionally
 * and letting that error speak avoids the FE's own eligibility model
 * drifting out of sync with the backend's.
 *
 * A `200` here does not necessarily mean a *clean* success — check
 * `response.handoff.githubIssueError` on every result; the ServiceNow state
 * can commit successfully while the internal GitHub issue fails.
 */
export function useHandOffIncident(): UseMutationResult<
  BeHandOffIncidentResponse,
  Error,
  HandOffIncidentInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeHandOffIncidentResponse, Error, HandOffIncidentInput>({
    mutationFn: (input): Promise<BeHandOffIncidentResponse> =>
      api.post<BeHandOffIncidentPayload, BeHandOffIncidentResponse>(
        `/incidents/${encodeURIComponent(input.incidentId)}/specialist-handoffs`,
        input.payload,
      ),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.INCIDENT_DETAILS, variables.incidentId],
      });
      void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.INCIDENTS] });
    },
  });
}
