import { useFilters } from "@context/filters";

import type { EntityReference } from "@shared/types";

export function useIssueType(id: string | undefined): Partial<EntityReference> {
  const { data: filters } = useFilters();
  return filters?.issueTypes.find((issueType) => issueType.id === id) ?? {};
}
