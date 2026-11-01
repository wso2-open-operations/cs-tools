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
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type UseInfiniteQueryResult,
  type UseMutationResult,
  type UseQueryResult,
  type InfiniteData,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type {
  Allocation,
  AllocationUpdate,
  AllocationUpdatesResponse,
  PostAllocationUpdatePayload,
  SearchAllocationsPayload,
  SearchAllocationsResponse,
} from "../types";

/** Root of every allocation query key, so one invalidation reaches them all. */
export const ALLOCATIONS_KEY = "csm-allocations";

/** Updates fetched per "load more". */
export const UPDATES_PAGE_SIZE = 20;

const STALE_MS = 30_000;

/** `POST /engagement-allocations/search`: one filtered, sorted, paged list. */
export function useSearchAllocations(
  payload: SearchAllocationsPayload,
): UseQueryResult<SearchAllocationsResponse, Error> {
  const api = useBackendApi();
  return useQuery<SearchAllocationsResponse, Error>({
    queryKey: [ALLOCATIONS_KEY, "search", payload],
    queryFn: () =>
      api.post<SearchAllocationsPayload, SearchAllocationsResponse>(
        "/engagement-allocations/search",
        payload,
      ),
    placeholderData: keepPreviousData,
    staleTime: STALE_MS,
  });
}

/** `GET /engagement-allocations/{id}`. Resolves to `null` when it does not exist. */
export function useAllocation(id: string): UseQueryResult<Allocation | null, Error> {
  const api = useBackendApi();
  return useQuery<Allocation | null, Error>({
    queryKey: [ALLOCATIONS_KEY, "detail", id],
    queryFn: () => api.get<Allocation>(`/engagement-allocations/${encodeURIComponent(id)}`),
    enabled: id.length > 0,
    staleTime: STALE_MS,
  });
}

/** `GET /engagement-allocations/{id}/updates`, newest first, paged with "load more". */
export function useAllocationUpdates(
  id: string,
): UseInfiniteQueryResult<InfiniteData<AllocationUpdatesResponse>, Error> {
  const api = useBackendApi();
  return useInfiniteQuery<
    AllocationUpdatesResponse,
    Error,
    InfiniteData<AllocationUpdatesResponse>,
    readonly unknown[],
    number
  >({
    queryKey: [ALLOCATIONS_KEY, "updates", id],
    queryFn: async ({ pageParam }) => {
      const res = await api.get<AllocationUpdatesResponse>(
        `/engagement-allocations/${encodeURIComponent(id)}/updates?limit=${UPDATES_PAGE_SIZE}&offset=${pageParam}`,
      );
      return res ?? { updates: [], total: 0, limit: UPDATES_PAGE_SIZE, offset: pageParam, hasMore: false };
    },
    initialPageParam: 0,
    getNextPageParam: (last, all) => {
      if (!last.hasMore || last.updates.length === 0) return undefined;
      return all.reduce((n, p) => n + p.updates.length, 0);
    },
    enabled: id.length > 0,
    staleTime: STALE_MS,
  });
}

/**
 * `POST /engagement-allocations/{id}/updates`. Whether the caller may post is
 * the server's call: a refusal arrives as a `BackendApiError` carrying its own
 * message. On success the updates, the allocation (its latest update date) and
 * the list are refetched.
 */
export function usePostAllocationUpdate(
  id: string,
): UseMutationResult<AllocationUpdate, Error, PostAllocationUpdatePayload> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<AllocationUpdate, Error, PostAllocationUpdatePayload>({
    mutationFn: (body) =>
      api.post<PostAllocationUpdatePayload, AllocationUpdate>(
        `/engagement-allocations/${encodeURIComponent(id)}/updates`,
        body,
      ),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: [ALLOCATIONS_KEY] });
    },
  });
}
