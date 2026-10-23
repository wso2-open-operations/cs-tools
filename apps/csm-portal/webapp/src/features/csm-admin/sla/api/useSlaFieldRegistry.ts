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
import type { SlaFieldCatalogEntry, SlaWorkItemType } from "../types";

const SLA_WORK_ITEM_TYPES_KEY = ["sla", "work-item-types"] as const;
const SLA_FIELD_CATALOG_KEY = ["sla", "field-catalog"] as const;

/** Which work item types may carry SLAs (SLA SPEC.md R3) — read-only here;
 *  only `enabled` is admin-writable, and this backend has no route for it
 *  yet (the two seeded types, case/incident, cover v1's scope). */
export function useSlaWorkItemTypes(): UseQueryResult<SlaWorkItemType[], Error> {
  const api = useBackendApi();
  return useQuery<SlaWorkItemType[], Error>({
    queryKey: SLA_WORK_ITEM_TYPES_KEY,
    queryFn: async () => {
      const res = await api.get<{ workItemTypes: SlaWorkItemType[] }>("/sla/work-item-types");
      return res?.workItemTypes ?? [];
    },
    staleTime: 5 * 60 * 1000,
  });
}

/** The field catalog (R5), joined with each entry's admin enablement flag
 *  (R16). Pass a work item type to narrow it, or omit for every type. */
export function useSlaFieldCatalog(workItemType?: string): UseQueryResult<SlaFieldCatalogEntry[], Error> {
  const api = useBackendApi();
  return useQuery<SlaFieldCatalogEntry[], Error>({
    queryKey: [...SLA_FIELD_CATALOG_KEY, workItemType ?? "all"],
    queryFn: async () => {
      const path = workItemType
        ? `/sla/field-catalog?workItemType=${encodeURIComponent(workItemType)}`
        : "/sla/field-catalog";
      const res = await api.get<{ fields: SlaFieldCatalogEntry[] }>(path);
      return res?.fields ?? [];
    },
    staleTime: 60 * 1000,
  });
}

/** Toggles one catalog entry's availability to the condition rule builder
 *  (R16) — the only thing an admin may change about the catalog. */
export function useSetSlaFieldEnabled(): UseMutationResult<
  SlaFieldCatalogEntry,
  Error,
  { fieldCatalogEntryId: string; enabled: boolean }
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<SlaFieldCatalogEntry, Error, { fieldCatalogEntryId: string; enabled: boolean }>({
    mutationFn: ({ fieldCatalogEntryId, enabled }) =>
      api.patch<{ enabled: boolean }, SlaFieldCatalogEntry>(
        `/sla/field-catalog/${encodeURIComponent(fieldCatalogEntryId)}/enablement`,
        { enabled },
      ),
    onSettled: () => void qc.invalidateQueries({ queryKey: SLA_FIELD_CATALOG_KEY }),
  });
}
