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
