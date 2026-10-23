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
import { useBackendApi } from "@api/backend/client";
import type {
  SearchSlaDefinitionsResponse,
  SlaDefinition,
  SlaDefinitionStatus,
  UpsertSlaDefinitionRequest,
} from "../types";

const SLA_DEFINITIONS_KEY = ["sla", "definitions"] as const;
const slaDefinitionKey = (id: string) => [...SLA_DEFINITIONS_KEY, id] as const;

export interface SearchSlaDefinitionsParams {
  workItemType?: string;
  status?: SlaDefinitionStatus;
}

const SLA_DEFINITIONS_SEARCH_PAGE_SIZE = 50;
// A bound on how many pages this ever fetches -- real admin configuration,
// not user-generated data, so this is just a safety ceiling against an
// unexpected runaway loop, not a limit anyone should ever actually hit.
const SLA_DEFINITIONS_SEARCH_MAX_PAGES = 200;

/** POST /sla/definitions/search — every definition, optionally narrowed by
 *  work item type or status. entity-service caps a single page at 50, so
 *  this pages through the full result rather than silently dropping
 *  anything past the first page (the editor's "also completes" picker and
 *  the notification rule list both need the complete set, not page one). */
export function useSlaDefinitions(
  params: SearchSlaDefinitionsParams = {},
): UseQueryResult<SearchSlaDefinitionsResponse, Error> {
  const api = useBackendApi();
  return useQuery<SearchSlaDefinitionsResponse, Error>({
    queryKey: [...SLA_DEFINITIONS_KEY, "search", params.workItemType ?? "", params.status ?? ""],
    queryFn: async () => {
      const all: SlaDefinition[] = [];
      let total = 0;
      for (let page = 0; page < SLA_DEFINITIONS_SEARCH_MAX_PAGES; page++) {
        const res = await api.post<
          { workItemType?: string; status?: string; pagination: { limit: number; offset: number } },
          SearchSlaDefinitionsResponse
        >("/sla/definitions/search", {
          workItemType: params.workItemType,
          status: params.status,
          pagination: { limit: SLA_DEFINITIONS_SEARCH_PAGE_SIZE, offset: page * SLA_DEFINITIONS_SEARCH_PAGE_SIZE },
        });
        all.push(...res.definitions);
        total = res.total;
        if (res.definitions.length < SLA_DEFINITIONS_SEARCH_PAGE_SIZE || all.length >= total) break;
      }
      return { definitions: all, total, limit: all.length, offset: 0 };
    },
    staleTime: 30 * 1000,
  });
}

export function useSlaDefinition(id: string | undefined): UseQueryResult<SlaDefinition, Error> {
  const api = useBackendApi();
  return useQuery<SlaDefinition, Error>({
    queryKey: slaDefinitionKey(id ?? ""),
    queryFn: async () => {
      const res = await api.get<SlaDefinition>(`/sla/definitions/${encodeURIComponent(id ?? "")}`);
      if (res === null) throw new Error(`GET /sla/definitions/${id} returned 404`);
      return res;
    },
    enabled: Boolean(id),
  });
}

function invalidateDefinitions(qc: ReturnType<typeof useQueryClient>, id?: string): void {
  void qc.invalidateQueries({ queryKey: SLA_DEFINITIONS_KEY });
  if (id) void qc.invalidateQueries({ queryKey: slaDefinitionKey(id) });
}

/** POST /sla/definitions — always lands in "draft" (R14). */
export function useCreateSlaDefinition(): UseMutationResult<SlaDefinition, Error, UpsertSlaDefinitionRequest> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<SlaDefinition, Error, UpsertSlaDefinitionRequest>({
    mutationFn: (body) => api.post<UpsertSlaDefinitionRequest, SlaDefinition>("/sla/definitions", body),
    onSettled: () => invalidateDefinitions(qc),
  });
}

export function useUpdateSlaDefinition(): UseMutationResult<
  SlaDefinition,
  Error,
  { id: string; body: UpsertSlaDefinitionRequest }
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<SlaDefinition, Error, { id: string; body: UpsertSlaDefinitionRequest }>({
    mutationFn: ({ id, body }) =>
      api.patch<UpsertSlaDefinitionRequest, SlaDefinition>(`/sla/definitions/${encodeURIComponent(id)}`, body),
    onSettled: (_data, _error, vars) => invalidateDefinitions(qc, vars.id),
  });
}

/** PATCH /sla/definitions/{id}/status — draft → published → retired (R14).
 *  Deliberately its own call, separate from an ordinary field edit, so
 *  publishing is always a conscious act. */
export function useSetSlaDefinitionStatus(): UseMutationResult<
  SlaDefinition,
  Error,
  { id: string; status: SlaDefinitionStatus }
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<SlaDefinition, Error, { id: string; status: SlaDefinitionStatus }>({
    mutationFn: ({ id, status }) =>
      api.patch<{ status: SlaDefinitionStatus }, SlaDefinition>(
        `/sla/definitions/${encodeURIComponent(id)}/status`,
        { status },
      ),
    onSettled: (_data, _error, vars) => invalidateDefinitions(qc, vars.id),
  });
}
