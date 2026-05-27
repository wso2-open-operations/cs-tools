import { useProject } from "@root/src/context/project";
import { type PopoverProps, Stack, Typography } from "@wso2/oxygen-ui";
import { Inbox } from "@wso2/oxygen-ui-icons-react";

import { ProjectPopoverItem, ProjectPopoverItemSkeleton } from "@features/projects/components";
import { useProjectsList } from "@features/projects/hooks";

import { InfiniteList } from "@shared/components/common";

export function ProjectPopoverList({ search, onClose }: { search: string; onClose: PopoverProps["onClose"] }) {
  const { projectId, setProjectId } = useProject();
  const query = useProjectsList(search);

  const totalRecords = query.data?.pages[0].pagination.totalRecords;

  return (
    <InfiniteList
      {...query}
      virtualize={false}
      sentinel={<ProjectPopoverListSkeleton />}
      tail={
        totalRecords === 0 && (
          <Stack direction="row" gap={1} px={1.5} sx={{ opacity: 0.6 }}>
            <Inbox />
            <Typography>No projects found</Typography>
          </Stack>
        )
      }
    >
      {(item) => (
        <ProjectPopoverItem
          {...item}
          active={item.id === projectId}
          onClick={() => {
            setProjectId(item.id);
            onClose?.({}, "backdropClick");
          }}
        />
      )}
    </InfiniteList>
  );
}

function ProjectPopoverListSkeleton() {
  return (
    <>
      {Array.from({ length: 5 }).map((_, index) => (
        <ProjectPopoverItemSkeleton key={index} />
      ))}
    </>
  );
}
