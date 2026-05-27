import { useInfiniteQuery } from "@tanstack/react-query";

import { projects } from "@features/projects/api/projects.queries";
import { useFilters } from "@features/projects/hooks";

export function useProjectsList(search?: string) {
  const { filters } = useFilters();
  return useInfiniteQuery(projects.paginated({ filters: { searchQuery: search ?? (filters.search || undefined) } }));
}
