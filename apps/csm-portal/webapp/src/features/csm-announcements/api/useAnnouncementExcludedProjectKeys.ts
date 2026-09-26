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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeExcludedProjectKeysResponse } from "@api/backend/types";

/**
 * Read-only display of the mandatory excluded-project-key denylist the
 * backend always applies to an "All customer projects" audience resolution
 * (`useResolveAnnouncementAudience`'s dedicated endpoint injects this
 * unconditionally — the caller can't see or change it there). This hook
 * exists purely so `AudienceScopeControls` can show *which* projects are
 * excluded, matching the real ServiceNow flow's own condition builder where
 * the excluded project keys are plainly visible, instead of leaving it an
 * opaque "a configured list."
 *
 * Configured only via CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS — there is no
 * UI to edit it, deliberately, for now.
 */
export function useAnnouncementExcludedProjectKeys(): UseQueryResult<string[], Error> {
  const api = useBackendApi();

  return useQuery<string[], Error>({
    queryKey: [ApiQueryKeys.CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS],
    queryFn: async (): Promise<string[]> => {
      const res = await api.get<BeExcludedProjectKeysResponse>(
        "/announcements/audience/excluded-project-keys",
      );
      return res?.excludedProjectKeys ?? [];
    },
    staleTime: 60_000,
  });
}
