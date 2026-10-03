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

import { useMutation, useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useBackendApi, BackendApiError } from "@api/backend/client";
import type { ProjectRisk } from "./customerHealthTypes";

// POST /customer-health/projects/{projectSysId}/risk
export function useOpenProjectRisk() {
  const backendApi = useBackendApi();
  return useMutation<ProjectRisk, Error, { projectSysId: string; accountSysId: string; comment: string }>({
    mutationFn: ({ projectSysId, accountSysId, comment }) =>
      backendApi.post<{ accountSysId: string; comment: string }, ProjectRisk>(
        `/customer-health/projects/${encodeURIComponent(projectSysId)}/risk`,
        { accountSysId, comment },
      ),
  });
}

// POST /customer-health/projects/{projectSysId}/mark-healthy
export function useMarkProjectHealthy() {
  const backendApi = useBackendApi();
  return useMutation<
    unknown,
    Error,
    { projectSysId: string; accountSysId: string; comment: string }
  >({
    mutationFn: ({ projectSysId, accountSysId, comment }) =>
      backendApi.post<{ accountSysId: string; comment: string }, unknown>(
        `/customer-health/projects/${encodeURIComponent(projectSysId)}/mark-healthy`,
        { accountSysId, comment },
      ),
  });
}

// PUT /customer-health/risks/{riskId}/close — riskId is an INTEGER.
// The source app read a bespoke error message off the failure
// (`err.response.data.message`) for this one call — BackendApiError's own
// `.payload?.message` is the equivalent field here.
export function useCloseProjectRisk() {
  const backendApi = useBackendApi();
  return useMutation<unknown, BackendApiError, { riskId: number; comment: string }>({
    mutationFn: ({ riskId, comment }) =>
      backendApi.put<{ comment: string }, unknown>(`/customer-health/risks/${riskId}/close`, { comment }),
  });
}

export function closeProjectRiskErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof BackendApiError && error.message) return error.message;
  return fallback;
}

// GET /customer-health/projects/{projectSysId}/risk-history — fetched
// lazily (only when the history dialog opens), so `enabled` is controlled
// by the caller rather than firing on mount.
export function useProjectRiskHistory(
  projectSysId: string,
  enabled: boolean,
): UseQueryResult<ProjectRisk[], Error> {
  const backendApi = useBackendApi();
  return useQuery<ProjectRisk[], Error>({
    queryKey: ["spl-customer-health-risk-history", projectSysId],
    enabled,
    queryFn: async () =>
      (await backendApi.get<ProjectRisk[]>(
        `/customer-health/projects/${encodeURIComponent(projectSysId)}/risk-history`,
      )) ?? [],
  });
}
