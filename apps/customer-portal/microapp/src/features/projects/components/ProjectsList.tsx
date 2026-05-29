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
