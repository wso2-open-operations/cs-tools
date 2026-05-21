import { ProjectItem, ProjectItemSkeleton } from "@features/projects/components";
import { useProjectsList } from "@features/projects/hooks";

import { EmptyState } from "@shared/components/common";

export function ProjectsList() {
  const { data, isLoading } = useProjectsList();

  if (isLoading) return <ProjectsListSkeleton />;

  if (!data?.length) return <EmptyState />;

  return (
    <>
      {data.map((props) => (
        <ProjectItem key={props.id} {...props} />
      ))}
    </>
  );
}

export function ProjectsListSkeleton() {
  return (
    <>
      {Array.from({ length: 5 }).map((_, index) => (
        <ProjectItemSkeleton key={index} />
      ))}
    </>
  );
}
