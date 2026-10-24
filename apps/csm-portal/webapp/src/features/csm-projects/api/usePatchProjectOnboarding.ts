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

/** The onboarding status labels `PATCH /projects/{id}` accepts. */
export const PROJECT_ONBOARDING_STATUSES = [
  "Not-Started",
  "In-Progress",
  "On-Hold",
  "Completed",
  "Cancelled",
  "Expired",
  "Not-Applicable",
] as const;

export type ProjectOnboardingStatusValue = (typeof PROJECT_ONBOARDING_STATUSES)[number];

/**
 * `PATCH /projects/{id}` onboarding fields. All are optional, but at least
 * one must be sent. `onboardingOwner` is the id of an active internal user.
 * The backend has no way to clear the owner or any of the dates (omitted or
 * `null` means unchanged), so a caller must never send `null`/an empty value
 * expecting that. The four date fields are calendar dates as `YYYY-MM-DD`;
 * the backend rejects a plan/go-live/expiry date earlier than the start date.
 * Note the read side names two of them differently (`goLivePlanDate`,
 * `goLiveDate`, see `ProjectDetails`).
 */
export interface ProjectOnboardingPatch {
  onboardingStatus?: ProjectOnboardingStatusValue;
  onboardingOwner?: string;
  onboardingStartDate?: string;
  onboardingGoLivePlanDate?: string;
  onboardingGoLiveDate?: string;
  onboardingExpiryDate?: string;
}

interface ProjectOnboardingPatchResponse {
  message: string;
  project: {
    id: string;
    onboardingStatus?: string | null;
    onboardingOwner?: { id: string; name: string; email: string | null } | null;
    onboardingStartDate?: string | null;
    goLivePlanDate?: string | null;
    goLiveDate?: string | null;
    onboardingExpiryDate?: string | null;
  };
}

/**
 * Set a project's onboarding status, owner and/or dates via
 * `PATCH /projects/{id}` — `onboarding_lead` or `admin` only (see
 * `usePortalAccess().canEditProjectOnboarding`, this app's mirror of the
 * backend's own check). The backend is the real gate: a caller without the
 * permission gets a 403 and a non-database project a 400, both surfaced by the
 * caller. Mirrors `usePatchProjectClosureState`'s shape, and on success
 * refreshes both this project's detail and the projects list, whose
 * onboarding column would otherwise stay stale.
 */
export function usePatchProjectOnboarding(
  projectId: string | undefined,
): UseMutationResult<ProjectOnboardingPatchResponse, Error, ProjectOnboardingPatch> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<ProjectOnboardingPatchResponse, Error, ProjectOnboardingPatch>({
    mutationFn: async (patch): Promise<ProjectOnboardingPatchResponse> => {
      if (!projectId) {
        throw new Error("Cannot update a project without an id.");
      }
      return api.patch<ProjectOnboardingPatch, ProjectOnboardingPatchResponse>(
        `/projects/${encodeURIComponent(projectId)}`,
        patch,
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_PROJECT_DETAIL, projectId ?? ""],
      });
      // The projects list (`useSearchProjects.ts`) keys its cache with a raw
      // literal, not `ApiQueryKeys.CSM_PROJECTS`.
      queryClient.invalidateQueries({
        predicate: (q) => q.queryKey[0] === "csm-projects-search",
      });
    },
  });
}
