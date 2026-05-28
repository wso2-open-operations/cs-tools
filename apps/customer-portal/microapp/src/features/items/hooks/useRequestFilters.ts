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
import { useFilters } from "@context/filters";

import type { GetCasesRequestDto } from "@features/case-types/cases/types";
import type { GetChangeRequestsRequestDto } from "@features/case-types/change-requests/types";
import type { GetChatsRequestDto } from "@features/case-types/conversations/types";
import { useFilters as useListFilterParams } from "@features/items/hooks";

export function useCaseFiltersFromParams(): GetCasesRequestDto["filters"] {
  const { filters } = useListFilterParams();
  const { data: available } = useFilters();

  return {
    searchQuery: filters.search,
    statusIds: filters.statuses?.filter((id) => available?.caseStates.map((s) => s.id).includes(id)).map(Number),
    severityId: filters.severities?.[0] ? Number(filters.severities[0]) : undefined,
    closedStartDate: filters.startDate,
    closedEndDate: filters.endDate,
  };
}

export function useChatFiltersFromParams(): GetChatsRequestDto["filters"] {
  const { filters } = useListFilterParams();
  const { data: available } = useFilters();

  return {
    searchQuery: filters.search,
    stateKeys: filters.statuses
      ?.filter((id) => available?.conversationStates.map((s) => s.id).includes(id))
      .map(Number),
  };
}

export function useChangeRequestFiltersFromParams(): GetChangeRequestsRequestDto["filters"] {
  const { filters } = useListFilterParams();
  const { data: available } = useFilters();

  return {
    searchQuery: filters.search,
    stateKeys: filters.states?.filter((id) => available?.changeRequestStates.map((s) => s.id).includes(id)).map(Number),
    closedStartDate: filters.startDate,
    closedEndDate: filters.endDate,
  };
}
