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
import { useBackendApi } from "@api/backend/client";
import { skillKeys } from "@features/csm-skills/api/skillKeys";
import type {
  ApproveSkillRequestRequest,
  CheckSkillRequestResponse,
  CreateSkillCategoryRequest,
  CreateSkillRequestRequest,
  SearchSkillDirectoryRequest,
  SearchSkillDirectoryResponse,
  SkillCapabilities,
  SkillCategory,
  SkillHistoryResponse,
  SkillRequest,
  SkillRequestStatus,
  SkillRequestsResponse,
  UpdateSkillCategoryRequest,
  UserSkillProfileResponse,
} from "@features/csm-skills/types";

const NO_CAPABILITIES: SkillCapabilities = {
  canViewDirectory: false,
  canReviewRequests: false,
  canManageCatalogue: false,
};

/**
 * Which gated skills tasks the caller holds (the directory, reviewing
 * requests, managing the catalogue). The page builds its tabs from this; the
 * backend still checks every call.
 */
export function useSkillCapabilities(): UseQueryResult<SkillCapabilities, Error> {
  const api = useBackendApi();
  return useQuery<SkillCapabilities, Error>({
    queryKey: skillKeys.capabilities,
    queryFn: async () => (await api.get<SkillCapabilities>("/skills/me/capabilities")) ?? NO_CAPABILITIES,
    staleTime: 60_000,
  });
}

// --- requests ---

/** The duplicate preview for a skill name, while the request form is typed in. */
export function useCheckSkillRequest(name: string): UseQueryResult<CheckSkillRequestResponse, Error> {
  const api = useBackendApi();
  const trimmed = name.trim();
  return useQuery<CheckSkillRequestResponse, Error>({
    queryKey: skillKeys.requestCheck(trimmed.toLowerCase()),
    queryFn: async () =>
      (await api.post<{ name: string }, CheckSkillRequestResponse>("/skills/requests/check", { name: trimmed })) ?? {
        similar: [],
      },
    enabled: trimmed.length >= 2,
    placeholderData: keepPreviousData,
  });
}

export function useMySkillRequests(): UseQueryResult<SkillRequestsResponse, Error> {
  const api = useBackendApi();
  return useQuery<SkillRequestsResponse, Error>({
    queryKey: skillKeys.myRequests,
    queryFn: async () =>
      (await api.get<SkillRequestsResponse>("/skills/requests/mine")) ?? { requests: [], total: 0, limit: 0, offset: 0 },
  });
}

/** A request write changes the queue, the counts, the catalogue and (on approval) a profile. */
function useRefreshRequests(): () => Promise<unknown> {
  const qc = useQueryClient();
  return () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: [...skillKeys.root, "requests"] }),
      qc.invalidateQueries({ queryKey: skillKeys.capabilities }),
      qc.invalidateQueries({ queryKey: skillKeys.catalogue }),
      qc.invalidateQueries({ queryKey: skillKeys.mine }),
      qc.invalidateQueries({ queryKey: [...skillKeys.root, "history"] }),
    ]);
}

export function useCreateSkillRequest(): UseMutationResult<SkillRequest, Error, CreateSkillRequestRequest> {
  const api = useBackendApi();
  const refresh = useRefreshRequests();
  return useMutation<SkillRequest, Error, CreateSkillRequestRequest>({
    mutationFn: (body) => api.post<CreateSkillRequestRequest, SkillRequest>("/skills/requests", body),
    onSuccess: refresh,
  });
}

export function useWithdrawSkillRequest(): UseMutationResult<SkillRequest, Error, string> {
  const api = useBackendApi();
  const refresh = useRefreshRequests();
  return useMutation<SkillRequest, Error, string>({
    mutationFn: (id) => api.post<Record<string, never>, SkillRequest>(`/skills/requests/${encodeURIComponent(id)}/withdraw`, {}),
    onSuccess: refresh,
  });
}

export interface SkillRequestQueueQuery {
  statuses: SkillRequestStatus[];
  limit: number;
  offset: number;
}

export function useSkillRequestQueue(q: SkillRequestQueueQuery, enabled = true): UseQueryResult<SkillRequestsResponse, Error> {
  const api = useBackendApi();
  const body = { statuses: q.statuses, pagination: { limit: q.limit, offset: q.offset } };
  return useQuery<SkillRequestsResponse, Error>({
    queryKey: skillKeys.requestQueue(body),
    queryFn: async () =>
      (await api.post<typeof body, SkillRequestsResponse>("/skills/requests/search", body)) ?? {
        requests: [],
        total: 0,
        limit: q.limit,
        offset: q.offset,
      },
    enabled,
    placeholderData: keepPreviousData,
  });
}

export function useApproveSkillRequest(): UseMutationResult<
  SkillRequest,
  Error,
  { id: string; body: ApproveSkillRequestRequest }
> {
  const api = useBackendApi();
  const refresh = useRefreshRequests();
  const qc = useQueryClient();
  return useMutation<SkillRequest, Error, { id: string; body: ApproveSkillRequestRequest }>({
    mutationFn: ({ id, body }) =>
      api.post<ApproveSkillRequestRequest, SkillRequest>(`/skills/requests/${encodeURIComponent(id)}/approve`, body),
    onSuccess: () => Promise.all([refresh(), qc.invalidateQueries({ queryKey: skillKeys.categories })]),
    // A 409 (decided by someone else meanwhile) also means the queue is stale.
    onError: refresh,
  });
}

export function useRejectSkillRequest(): UseMutationResult<SkillRequest, Error, { id: string; reason: string }> {
  const api = useBackendApi();
  const refresh = useRefreshRequests();
  return useMutation<SkillRequest, Error, { id: string; reason: string }>({
    mutationFn: ({ id, reason }) =>
      api.post<{ reason: string }, SkillRequest>(`/skills/requests/${encodeURIComponent(id)}/reject`, { reason }),
    onSuccess: refresh,
    onError: refresh,
  });
}

// --- categories ---

function useRefreshCategories(): () => Promise<unknown> {
  const qc = useQueryClient();
  return () => qc.invalidateQueries({ queryKey: skillKeys.root });
}

export function useCreateSkillCategory(): UseMutationResult<SkillCategory, Error, CreateSkillCategoryRequest> {
  const api = useBackendApi();
  const refresh = useRefreshCategories();
  return useMutation<SkillCategory, Error, CreateSkillCategoryRequest>({
    mutationFn: (body) => api.post<CreateSkillCategoryRequest, SkillCategory>("/skills/categories", body),
    onSuccess: refresh,
  });
}

export function useUpdateSkillCategory(): UseMutationResult<
  SkillCategory,
  Error,
  { id: string; body: UpdateSkillCategoryRequest }
> {
  const api = useBackendApi();
  const refresh = useRefreshCategories();
  return useMutation<SkillCategory, Error, { id: string; body: UpdateSkillCategoryRequest }>({
    mutationFn: ({ id, body }) =>
      api.patch<UpdateSkillCategoryRequest, SkillCategory>(`/skills/categories/${encodeURIComponent(id)}`, body),
    onSuccess: refresh,
  });
}

// --- directory ---

export function useSkillDirectory(
  request: SearchSkillDirectoryRequest,
  enabled = true,
): UseQueryResult<SearchSkillDirectoryResponse, Error> {
  const api = useBackendApi();
  return useQuery<SearchSkillDirectoryResponse, Error>({
    queryKey: skillKeys.directory(request),
    queryFn: async () =>
      (await api.post<SearchSkillDirectoryRequest, SearchSkillDirectoryResponse>("/skills/directory/search", request)) ?? {
        items: [],
        total: 0,
        limit: request.pagination?.limit ?? 0,
        offset: request.pagination?.offset ?? 0,
      },
    enabled,
    placeholderData: keepPreviousData,
  });
}

export function useUserSkillProfile(userId: string | undefined): UseQueryResult<UserSkillProfileResponse, Error> {
  const api = useBackendApi();
  return useQuery<UserSkillProfileResponse, Error>({
    queryKey: skillKeys.person(userId ?? ""),
    queryFn: async () =>
      (await api.get<UserSkillProfileResponse>(`/skills/users/${encodeURIComponent(userId as string)}`)) ?? {
        userId: userId as string,
        teamKeys: [],
        skills: [],
      },
    enabled: !!userId,
  });
}

export function useUserSkillHistory(
  userId: string | undefined,
  skillId: string,
  limit: number,
  offset: number,
): UseQueryResult<SkillHistoryResponse, Error> {
  const api = useBackendApi();
  return useQuery<SkillHistoryResponse, Error>({
    queryKey: skillKeys.personHistory(userId ?? "", skillId, limit, offset),
    queryFn: async () => {
      const q = new URLSearchParams({ limit: String(limit), offset: String(offset) });
      if (skillId) q.set("skillId", skillId);
      return (
        (await api.get<SkillHistoryResponse>(`/skills/users/${encodeURIComponent(userId as string)}/history?${q}`)) ?? {
          history: [],
          total: 0,
          limit,
          offset,
        }
      );
    },
    enabled: !!userId,
    placeholderData: keepPreviousData,
  });
}
