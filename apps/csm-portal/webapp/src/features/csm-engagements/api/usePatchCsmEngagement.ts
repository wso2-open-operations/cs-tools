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
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { ApiMutationKeys, ApiQueryKeys } from "@constants/apiConstants";
import { patchMockCsmEngagement } from "@features/csm-engagements/api/mocks/engagementsMocks";
import type {
  CsmEngagementDetail,
  CsmEngagementLifecycleAction,
  CsmEngagementState,
} from "@features/csm-engagements/types/csmEngagements";

export interface PatchCsmEngagementVariables {
  engagementId: string;
  /** Optional lifecycle action — translated to a state change. */
  action?: CsmEngagementLifecycleAction;
  /** Or pass an explicit state to set. */
  state?: CsmEngagementState;
  ownerId?: string;
  ownerName?: string;
  plannedEndDate?: string;
  isWatching?: boolean;
}

const ACTION_TO_STATE: Record<CsmEngagementLifecycleAction, CsmEngagementState> = {
  approve_request: "in_progress",
  start_work: "in_progress",
  put_on_hold: "on_hold",
  resume_work: "in_progress",
  complete_engagement: "completed",
  cancel_engagement: "cancelled",
  reopen: "in_progress",
};

export function usePatchCsmEngagement(): UseMutationResult<
  CsmEngagementDetail,
  Error,
  PatchCsmEngagementVariables
> {
  const authFetch = useAuthApiClient();
  const qc = useQueryClient();

  return useMutation<CsmEngagementDetail, Error, PatchCsmEngagementVariables>({
    mutationKey: ApiMutationKeys.PATCH_ENGAGEMENT,
    mutationFn: async (vars) => {
      const patch = {
        state: vars.state ?? (vars.action ? ACTION_TO_STATE[vars.action] : undefined),
        ownerId: vars.ownerId,
        ownerName: vars.ownerName,
        plannedEndDate: vars.plannedEndDate,
        isWatching: vars.isWatching,
      };
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        await new Promise((r) => setTimeout(r, 150));
        const updated = patchMockCsmEngagement(vars.engagementId, patch);
        if (!updated) throw new Error(`Engagement ${vars.engagementId} not found`);
        return updated;
      }
      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const url = `${baseUrl}/csm/engagements/${encodeURIComponent(vars.engagementId)}`;
      const response = await authFetch(url, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(patch),
      });
      if (!response.ok) {
        throw new Error(
          `Error patching engagement: ${response.statusText}`,
        );
      }
      return (await response.json()) as CsmEngagementDetail;
    },
    onSuccess: (data, vars) => {
      qc.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ENGAGEMENTS] });
      qc.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_ENGAGEMENT_DETAIL, vars.engagementId],
      });
      qc.setQueryData(
        [ApiQueryKeys.CSM_ENGAGEMENT_DETAIL, vars.engagementId],
        data,
      );
    },
  });
}
