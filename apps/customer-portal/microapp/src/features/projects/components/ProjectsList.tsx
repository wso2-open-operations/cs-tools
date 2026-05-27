import { ProjectItem, ProjectItemSkeleton } from "@features/projects/components";
import { useProjectsList } from "@features/projects/hooks";

import { EmptyState, InfiniteList } from "@shared/components/common";

export function ProjectsList() {
  const query = useProjectsList();
  const tail = query.data?.pages[0].pagination.totalRecords === 0 && <EmptyState />;

  return (
    <InfiniteList {...query} sentinel={<ProjectsListSkeleton />} tail={tail}>
      {(item) => <ProjectItem {...item} />}
    </InfiniteList>
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
