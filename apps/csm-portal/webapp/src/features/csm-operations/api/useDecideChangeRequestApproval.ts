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
  BeChangeRequestApprovalDecision,
  BeChangeRequestApprovalDecisionPayload,
  BeChangeRequestApprovalDecisionResponse,
} from "@api/backend/types";

export interface DecideChangeRequestApprovalInput {
  id: string;
  decision: BeChangeRequestApprovalDecision;
}

/**
 * Submit the caller's decision on their own pending approval via
 * `POST /change-requests/{id}/approvals/decision` (ServiceNow data source
 * only). Any user with access to the change request may attempt this;
 * ServiceNow itself enforces that only the caller's own pending approval can
 * be decided. ServiceNow's existing business rule cascades the change
 * request's own state automatically, so on success both the approvals and
 * the change request detail are invalidated so the page reflects the new
 * state/stage immediately.
 */
export function useDecideChangeRequestApproval(): UseMutationResult<
  BeChangeRequestApprovalDecisionResponse,
  Error,
  DecideChangeRequestApprovalInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<
    BeChangeRequestApprovalDecisionResponse,
    Error,
    DecideChangeRequestApprovalInput
  >({
    mutationFn: (input): Promise<BeChangeRequestApprovalDecisionResponse> =>
      api.post<
        BeChangeRequestApprovalDecisionPayload,
        BeChangeRequestApprovalDecisionResponse
      >(`/change-requests/${encodeURIComponent(input.id)}/approvals/decision`, {
        decision: input.decision,
      }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CHANGE_REQUEST_APPROVALS, variables.id],
      });
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CHANGE_REQUEST_DETAILS, variables.id],
      });
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CHANGE_REQUESTS],
      });
    },
  });
}
