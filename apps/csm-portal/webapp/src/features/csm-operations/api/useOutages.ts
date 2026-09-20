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
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCreateOutagePayload,
  BeCreateOutageResponse,
  BeOutageDetail,
  BeOutageMetadataResponse,
  BePatchOutagePayload,
  BePatchOutageResponse,
  BeSearchOutagesPayload,
  BeSearchOutagesResponse,
} from "@api/backend/types";

/**
 * Search outages via `POST /outages/search` (ServiceNow data source only).
 * Returns the raw paged response, including `appliedBeginFrom`/
 * `beginFromDefaulted` so the caller can tell the list when an implicit
 * six-month lookback was applied. `keepPreviousData` keeps the table
 * populated while the next page/filter loads.
 */
export function useSearchOutages(
  payload: BeSearchOutagesPayload,
): UseQueryResult<BeSearchOutagesResponse, Error> {
  const api = useBackendApi();

  return useQuery<BeSearchOutagesResponse, Error>({
    queryKey: [ApiQueryKeys.OUTAGES, payload],
    queryFn: (): Promise<BeSearchOutagesResponse> =>
      api.post<BeSearchOutagesPayload, BeSearchOutagesResponse>("/outages/search", payload),
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });
}

/**
 * Look up a single outage by id via `GET /outages/{id}`. Returns `null` when
 * the id is unknown so the detail page can render a not-found state rather
 * than an error.
 */
export function useGetOutage(
  id: string | undefined,
): UseQueryResult<BeOutageDetail | null, Error> {
  const api = useBackendApi();

  return useQuery<BeOutageDetail | null, Error>({
    queryKey: [ApiQueryKeys.OUTAGE_DETAILS, id ?? ""],
    queryFn: (): Promise<BeOutageDetail | null> =>
      api.get<BeOutageDetail>(`/outages/${encodeURIComponent(id as string)}`),
    enabled: !!id,
    staleTime: 30_000,
  });
}

/**
 * The live choice lists needed to render the outage create/edit form —
 * critically `statusPageClouds`, so a CI picker can warn the engineer their
 * chosen configuration item publishes to the public status page *before*
 * they save, rather than surfacing that only as a `409` after the fact.
 * Basically static across a session; cached generously.
 */
export function useGetOutageMetadata(): UseQueryResult<BeOutageMetadataResponse | null, Error> {
  const api = useBackendApi();

  return useQuery<BeOutageMetadataResponse | null, Error>({
    queryKey: [ApiQueryKeys.OUTAGE_METADATA],
    queryFn: (): Promise<BeOutageMetadataResponse | null> =>
      api.get<BeOutageMetadataResponse>("/outages/metadata"),
    staleTime: 5 * 60_000,
  });
}

/** Create an outage via `POST /outages`. On success the outages list is
 * invalidated so the new record appears. */
export function usePostOutage(): UseMutationResult<
  BeCreateOutageResponse,
  Error,
  BeCreateOutagePayload
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeCreateOutageResponse, Error, BeCreateOutagePayload>({
    mutationFn: (payload): Promise<BeCreateOutageResponse> =>
      api.post<BeCreateOutagePayload, BeCreateOutageResponse>("/outages", payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.OUTAGES] });
    },
  });
}

export interface PatchOutageInput {
  id: string;
  patch: BePatchOutagePayload;
}

/**
 * Update (or close, via `{ end: … }`) an outage via `PATCH /outages/{id}`.
 * The backend requires at least one field. Detail and the outages list are
 * invalidated on success.
 */
export function usePatchOutage(): UseMutationResult<
  BePatchOutageResponse,
  Error,
  PatchOutageInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BePatchOutageResponse, Error, PatchOutageInput>({
    mutationFn: (input): Promise<BePatchOutageResponse> =>
      api.patch<BePatchOutagePayload, BePatchOutageResponse>(
        `/outages/${encodeURIComponent(input.id)}`,
        input.patch,
      ),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.OUTAGE_DETAILS, variables.id],
      });
      void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.OUTAGES] });
    },
  });
}
