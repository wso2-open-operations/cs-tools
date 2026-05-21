import { useEffect, useMemo } from "react";

import { useQuery } from "@tanstack/react-query";

import { useProject } from "@context/project";

import { projects } from "@features/projects/api/projects.queries";
import { useFilters } from "@features/projects/hooks";

export function useProjectsList() {
  const { filters } = useFilters();
  const { setProjectId } = useProject();
  const query = useQuery(projects.all());

  useEffect(() => {
    if (!query.data || query.data?.length > 1) return;
    setProjectId(query.data[0].id);
  }, [query.data]);

  const filtered = useMemo(() => {
    if (!filters.search) return query.data;
    const normalized = filters.search.toLowerCase();
    return query.data?.filter(
      (project) =>
        project.id.toLowerCase().includes(normalized) ||
        project.projectKey.toLowerCase().includes(normalized) ||
        project.name.toLowerCase().includes(normalized),
    );
  }, [query.data, filters.search]);

  return { ...query, data: filtered };
}
