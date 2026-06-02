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
import { createMockCsmEngagement } from "@features/csm-engagements/api/mocks/engagementsMocks";
import type {
  CreateCsmEngagementInput,
  CsmEngagementDetail,
} from "@features/csm-engagements/types/csmEngagements";

export function usePostCsmEngagement(): UseMutationResult<
  CsmEngagementDetail,
  Error,
  CreateCsmEngagementInput
> {
  const authFetch = useAuthApiClient();
  const qc = useQueryClient();

  return useMutation<CsmEngagementDetail, Error, CreateCsmEngagementInput>({
    mutationKey: ApiMutationKeys.CREATE_ENGAGEMENT,
    mutationFn: async (input) => {
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        await new Promise((r) => setTimeout(r, 250));
        return createMockCsmEngagement(input);
      }
      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const response = await authFetch(`${baseUrl}/csm/engagements`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(input),
      });
      if (!response.ok) {
        throw new Error(
          `Error creating engagement: ${response.statusText}`,
        );
      }
      return (await response.json()) as CsmEngagementDetail;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ENGAGEMENTS] });
    },
  });
}
