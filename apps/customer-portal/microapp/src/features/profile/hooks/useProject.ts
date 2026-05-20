import { useQuery } from "@tanstack/react-query";

import { useProject as useActiveProject } from "@context/project";

import { projects } from "@features/projects/api/projects.queries";

export function useProject() {
  const { projectId } = useActiveProject();
  const project = useQuery(projects.get(projectId!));
  return project;
}
