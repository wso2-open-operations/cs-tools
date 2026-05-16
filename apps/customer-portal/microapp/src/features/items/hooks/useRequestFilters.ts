import { useFilters as useListFilterParams } from "@features/items/hooks";
import type { GetCasesRequestDto } from "@features/cases/types";
import { useFilters } from "@context/filters";

export function useCaseFiltersFromParams(): GetCasesRequestDto["filters"] {
    const { filters } = useListFilterParams();
    const { data: available } = useFilters();
  
    return {
      searchQuery: filters.search,
      statusIds: filters.states
        ?.filter((id) => available?.caseStates.map((s) => s.id).includes(id))
        .map(Number),
      severityId: filters.severities?.[0] ? Number(filters.severities[0]) : undefined,
    };
  }

export function useChatFiltersFromParams() {
const { filters } = useListFilterParams();
const { data: available } = useFilters();

    return {
        searchQuery: filters.search,
        stateKeys: filters.states?.filter((id) => available?.conversationStates.map((s) => s.id).includes(id)).map(Number),
    };
}

export function useChangeRequestFiltersFromParams() {
    const { filters } = useListFilterParams();
    const { data: available } = useFilters();
  
    return {
      searchQuery: filters.search,
      stateKeys: filters.states?.filter((id) => available?.changeRequestStates.map((s) => s.id).includes(id)).map(Number),
    };
}
