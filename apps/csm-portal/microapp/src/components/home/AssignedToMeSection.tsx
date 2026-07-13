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

import { Button, Stack, Typography } from "@wso2/oxygen-ui";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { currentUser } from "@src/services/currentUser";
import { dashboard } from "@src/services/dashboard";
import { CaseCard, CaseCardSkeleton } from "@components/support/CaseCard";
import { EmptyState } from "@components/support/EmptyState";
import { ErrorState } from "@components/support/ErrorState";

// The signed-in user's own non-closed cases — mirrors the webapp's "Assigned to me" widget
// (apps/csm-portal/webapp/src/features/csm-dashboard/components/MyAssignedCases.tsx), capped to a
// short 5-item preview with a "View all" link out to Support, rather than paginated in place.
export function AssignedToMeSection() {
  const { data: currentUserId } = useQuery(currentUser.id());
  const { data, isPending, isError, refetch } = useQuery(dashboard.assignedToMe(currentUserId ?? null));

  const items = data?.items ?? [];

  return (
    <Stack gap={1.5}>
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Typography variant="subtitle1">Assigned to me</Typography>
        <Button component={Link} to="/support" variant="text" size="small" disableRipple>
          View all
        </Button>
      </Stack>

      {isLoading ? (
        <Stack gap={1.5}>
          {Array.from({ length: 3 }).map((_, index) => (
            <CaseCardSkeleton key={index} />
          ))}
        </Stack>
      ) : isError ? (
        <ErrorState onRetry={() => void refetch()} />
      ) : items.length === 0 ? (
        <EmptyState message="No cases assigned to you." />
      ) : (
        <Stack gap={1.5}>
          {items.map((item) => (
            <CaseCard key={item.id} item={item} />
          ))}
        </Stack>
      )}
    </Stack>
  );
}
