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
