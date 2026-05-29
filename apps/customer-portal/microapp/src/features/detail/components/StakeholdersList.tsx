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
import { Stack, Typography } from "@wso2/oxygen-ui";

import { StakeholderItem, StakeholderItemSkeleton } from "@features/detail/components";
import { useChangeRequest } from "@features/detail/hooks";

export function StakeholdersList() {
  const { data, isLoading } = useChangeRequest();

  if (isLoading) return <StakeholdersListSkeleton />;

  if (!data) {
    return (
      <Typography variant="body2" color="text.secondary">
        No stakeholders.
      </Typography>
    );
  }

  return (
    <Stack gap={1.5}>
      {data.createdBy && <StakeholderItem name={data.createdBy} role="owner" />}
      {data.approvedBy && <StakeholderItem name={data.approvedBy} role="approver" />}
      {data.assignedTeam && <StakeholderItem name={data.assignedTeam} role="requestor" />}
    </Stack>
  );
}

function StakeholdersListSkeleton() {
  return (
    <Stack gap={1.5}>
      {Array.from({ length: 3 }).map((_, index) => (
        <StakeholderItemSkeleton key={index} />
      ))}
    </Stack>
  );
}
