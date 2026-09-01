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
  BeAddOutageCommunicationPayload,
  BeAddOutageCommunicationResponse,
  BeSearchOutageCommunicationsPayload,
  BeSearchOutageCommunicationsResponse,
} from "@api/backend/types";

/**
 * Communication journal for a single outage, via
 * `POST /outages/{id}/communications/search`. `external` entries are
 * `isPublic: true` — they are echoed verbatim on the public status page once
 * the outage's configuration item resolves to a monitored cloud.
 */
export function useGetOutageCommunications(
  outageId: string | undefined,
): UseQueryResult<BeSearchOutageCommunicationsResponse | null, Error> {
  const api = useBackendApi();

  return useQuery<BeSearchOutageCommunicationsResponse | null, Error>({
    queryKey: [ApiQueryKeys.OUTAGE_COMMUNICATIONS, outageId ?? ""],
    queryFn: (): Promise<BeSearchOutageCommunicationsResponse | null> =>
      api.post<BeSearchOutageCommunicationsPayload, BeSearchOutageCommunicationsResponse>(
        `/outages/${encodeURIComponent(outageId as string)}/communications/search`,
        { pagination: { offset: 0, limit: 50 } },
      ),
    enabled: !!outageId,
    staleTime: 15_000,
  });
}

export interface AddOutageCommunicationInput {
  outageId: string;
  payload: BeAddOutageCommunicationPayload;
}

/**
 * Add a communication entry via `POST /outages/{id}/communications`.
 * Sending on the `external` channel is a publishing action: when the
 * outage's configuration item resolves to a monitored cloud, the same
 * `acknowledgePublicPublication`-style `409` gate applies as on create/patch
 * — the caller should have already warned the engineer before this fires
 * (see `useGetOutageMetadata`'s `statusPageClouds`), not rely on this
 * rejection as the first signal.
 */
export function useAddOutageCommunication(): UseMutationResult<
  BeAddOutageCommunicationResponse,
  Error,
  AddOutageCommunicationInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeAddOutageCommunicationResponse, Error, AddOutageCommunicationInput>({
    mutationFn: (input): Promise<BeAddOutageCommunicationResponse> =>
      api.post<BeAddOutageCommunicationPayload, BeAddOutageCommunicationResponse>(
        `/outages/${encodeURIComponent(input.outageId)}/communications`,
        input.payload,
      ),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.OUTAGE_COMMUNICATIONS, variables.outageId],
      });
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.OUTAGE_DETAILS, variables.outageId],
      });
    },
  });
}
