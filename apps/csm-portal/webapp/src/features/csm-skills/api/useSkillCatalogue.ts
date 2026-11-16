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
  CreateSkillRequest,
  SearchSkillsRequest,
  SearchSkillsResponse,
  Skill,
  SkillCategory,
  SkillCategoriesResponse,
  SkillLevelType,
  SkillLevelTypesResponse,
  UpdateSkillRequest,
} from "@features/csm-skills/types";

const REFERENCE_STALE_MS = 5 * 60_000;

/** `GET /skills/levels`: every level scale, each with its ordered levels. */
export function useSkillLevelTypes(): UseQueryResult<SkillLevelType[], Error> {
  const api = useBackendApi();
  return useQuery<SkillLevelType[], Error>({
    queryKey: skillKeys.levels,
    queryFn: async () => (await api.get<SkillLevelTypesResponse>("/skills/levels"))?.levelTypes ?? [],
    staleTime: REFERENCE_STALE_MS,
  });
}

/** `GET /skills/categories`: the flat category list (the tree is built client-side). */
export function useSkillCategories(): UseQueryResult<SkillCategory[], Error> {
  const api = useBackendApi();
  return useQuery<SkillCategory[], Error>({
    queryKey: skillKeys.categories,
    queryFn: async () => (await api.get<SkillCategoriesResponse>("/skills/categories"))?.categories ?? [],
    staleTime: REFERENCE_STALE_MS,
  });
}

/**
 * `POST /skills/search`: one server-filtered, server-paged page of the catalogue.
 * The previous page stays on screen while the next one loads.
 */
export function useSearchSkills(
  request: SearchSkillsRequest,
  enabled = true,
): UseQueryResult<SearchSkillsResponse, Error> {
  const api = useBackendApi();
  return useQuery<SearchSkillsResponse, Error>({
    queryKey: skillKeys.search(request),
    queryFn: async () =>
      (await api.post<SearchSkillsRequest, SearchSkillsResponse>("/skills/search", request)) ?? {
        skills: [],
        total: 0,
        limit: request.pagination?.limit ?? 0,
        offset: request.pagination?.offset ?? 0,
      },
    enabled,
    placeholderData: keepPreviousData,
  });
}

/** Page size of a type-ahead skill search. */
const PICKER_LIMIT = 50;

/**
 * Type-ahead skill search in the shape the shared async pickers expect
 * (`AsyncEntitySelect`/`AsyncEntityMultiSelect`). `categoryId`, when given,
 * narrows the search to that category and its descendants on the server.
 */
export function useSkillPickerSearch(
  query: string,
  enabled: boolean,
  categoryId?: string,
): { data: Skill[] | undefined; isFetching: boolean; isError: boolean } {
  const request: SearchSkillsRequest = {
    ...(query ? { searchQuery: query } : {}),
    ...(categoryId ? { categoryIds: [categoryId] } : {}),
    pagination: { limit: PICKER_LIMIT, offset: 0 },
  };
  const result = useSearchSkills(request, enabled);
  return { data: result.data?.skills, isFetching: result.isFetching, isError: result.isError };
}

/** `POST /skills`: create a skill. */
const TREE_PAGE = 200;
const TREE_MAX_PAGES = 20;

/**
 * Every active skill matching the search, for the All Skills tree. The search
 * runs on the backend; the pages are read until the total is reached (the
 * catalogue is a few hundred skills, so this is one or two requests).
 */
export function useAllSkills(searchQuery: string): UseQueryResult<Skill[], Error> {
  const api = useBackendApi();
  const q = searchQuery.trim();
  return useQuery<Skill[], Error>({
    queryKey: [...skillKeys.catalogue, "all", q],
    queryFn: async () => {
      const out: Skill[] = [];
      for (let page = 0; page < TREE_MAX_PAGES; page++) {
        const body: SearchSkillsRequest = {
          ...(q ? { searchQuery: q } : {}),
          pagination: { limit: TREE_PAGE, offset: page * TREE_PAGE },
        };
        const res = await api.post<SearchSkillsRequest, SearchSkillsResponse>("/skills/search", body);
        const rows = res?.skills ?? [];
        out.push(...rows);
        if (rows.length < TREE_PAGE || out.length >= (res?.total ?? 0)) break;
      }
      return out;
    },
    placeholderData: keepPreviousData,
  });
}

/** One skill by id (e.g. to pre-select it in the Add skills dialog). */
export function useSkill(id: string | undefined): UseQueryResult<Skill | null, Error> {
  const api = useBackendApi();
  return useQuery<Skill | null, Error>({
    queryKey: [...skillKeys.catalogue, "one", id ?? ""],
    queryFn: async () => (await api.get<Skill>(`/skills/${encodeURIComponent(id as string)}`)) ?? null,
    enabled: !!id,
    staleTime: REFERENCE_STALE_MS,
  });
}

export function useCreateSkill(): UseMutationResult<Skill, Error, CreateSkillRequest> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<Skill, Error, CreateSkillRequest>({
    mutationFn: (body) => api.post<CreateSkillRequest, Skill>("/skills", body),
    onSuccess: () => qc.invalidateQueries({ queryKey: skillKeys.catalogue }),
  });
}

/** `PATCH /skills/{id}`: edit or (de)activate a skill. */
export function useUpdateSkill(): UseMutationResult<Skill, Error, { id: string; body: UpdateSkillRequest }> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<Skill, Error, { id: string; body: UpdateSkillRequest }>({
    mutationFn: ({ id, body }) =>
      api.patch<UpdateSkillRequest, Skill>(`/skills/${encodeURIComponent(id)}`, body),
    // A renamed or recategorised skill also changes what the profile rows show.
    onSuccess: () => qc.invalidateQueries({ queryKey: skillKeys.root }),
  });
}
