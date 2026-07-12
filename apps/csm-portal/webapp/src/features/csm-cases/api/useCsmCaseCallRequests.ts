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
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCallRequestStateKey,
  BeCallRequestView,
  BeCreateCallRequestPayload,
  BeCreateCallRequestResponse,
  BeSearchCallRequestsPayload,
  BeSearchCallRequestsResponse,
  BeUpdateCallRequestPayload,
  BeUpdateCallRequestResponse,
} from "@api/backend/types";

/** Page size for the call-requests list; well within the BE max of 100. */
const CALL_REQUESTS_PAGE_LIMIT = 20;

/**
 * Load call requests for a case via `POST /cases/{id}/call-requests/search`.
 * A single wide page is used; the number of call requests per case is expected
 * to be small enough that pagination is not needed in the detail view.
 *
 * `states` filters server-side via `filters.states` (rather than fetching one
 * page and filtering client-side, which would silently drop matches beyond
 * the page limit).
 */
export function useGetCsmCaseCallRequests(
  caseId: string | undefined,
  states?: BeCallRequestStateKey[],
): UseQueryResult<BeCallRequestView[], Error> {
  const api = useBackendApi();

  return useQuery<BeCallRequestView[], Error>({
    queryKey: [ApiQueryKeys.CASE_CALL_REQUESTS, caseId ?? "", states ?? []],
    queryFn: async (): Promise<BeCallRequestView[]> => {
      if (!caseId) return [];

      const payload: BeSearchCallRequestsPayload = {
        ...(states && states.length > 0 ? { filters: { states } } : {}),
        pagination: { offset: 0, limit: CALL_REQUESTS_PAGE_LIMIT },
      };
      const response = await api.post<
        BeSearchCallRequestsPayload,
        BeSearchCallRequestsResponse
      >(
        `/cases/${encodeURIComponent(caseId)}/call-requests/search`,
        payload,
      );
      return response.callRequests ?? [];
    },
    enabled: !!caseId,
    staleTime: 10_000,
  });
}

export interface PostCsmCaseCallRequestInput {
  caseId: string;
  reason: string;
  /** ISO datetime strings (UTC preferred times). */
  utcTimes: string[];
  durationInMinutes: number;
}

/**
 * Create a call request on a case via `POST /cases/{id}/call-requests`.
 * Invalidates the call-requests list on success.
 */
export function usePostCsmCaseCallRequest(): UseMutationResult<
  BeCreateCallRequestResponse,
  Error,
  PostCsmCaseCallRequestInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeCreateCallRequestResponse, Error, PostCsmCaseCallRequestInput>({
    mutationFn: async (input): Promise<BeCreateCallRequestResponse> => {
      const payload: BeCreateCallRequestPayload = {
        reason: input.reason,
        utcTimes: input.utcTimes,
        durationInMinutes: input.durationInMinutes,
      };
      return api.post<BeCreateCallRequestPayload, BeCreateCallRequestResponse>(
        `/cases/${encodeURIComponent(input.caseId)}/call-requests`,
        payload,
      );
    },
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CASE_CALL_REQUESTS, variables.caseId],
      });
    },
  });
}

export interface PatchCsmCaseCallRequestInput {
  caseId: string;
  callRequestId: string;
  patch: BeUpdateCallRequestPayload;
}

/**
 * Update a call request state via `PATCH /cases/{caseId}/call-requests/{callRequestId}`.
 * Invalidates the call-requests list on success.
 */
export function usePatchCsmCaseCallRequest(): UseMutationResult<
  BeUpdateCallRequestResponse,
  Error,
  PatchCsmCaseCallRequestInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeUpdateCallRequestResponse, Error, PatchCsmCaseCallRequestInput>({
    mutationFn: async (input): Promise<BeUpdateCallRequestResponse> => {
      return api.patch<BeUpdateCallRequestPayload, BeUpdateCallRequestResponse>(
        `/cases/${encodeURIComponent(input.caseId)}/call-requests/${encodeURIComponent(input.callRequestId)}`,
        input.patch,
      );
    },
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CASE_CALL_REQUESTS, variables.caseId],
      });
    },
  });
}

