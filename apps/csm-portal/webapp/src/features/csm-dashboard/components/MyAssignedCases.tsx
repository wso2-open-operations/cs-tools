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

import { Link, TablePagination, Typography } from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import { Link as RouterLink } from "react-router";
import { ASSIGNEE_ME_TOKEN } from "@features/csm-cases/utils/assignee";
import CasesList from "@features/csm-cases/components/CasesList";
import {
  MY_OPEN_CASES_LINK_STATES,
  MY_OPEN_CASES_PAGE_SIZE,
  useGetMyAssignedOpenCases,
} from "@features/csm-dashboard/api/useGetMyAssignedOpenCases";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import SectionCard from "@features/csm-dashboard/components/SectionCard";

// Deep-link to the full cases list, pre-filtered to the caller's non-closed
// cases. `assignees=@me` resolves against the current user server-side; the
// states mirror the widget's non-closed set (minus `reopened`, which the list
// filter vocabulary has no option for).
const VIEW_ALL_HREF = `/cases?assignees=${encodeURIComponent(
  ASSIGNEE_ME_TOKEN,
)}&states=${MY_OPEN_CASES_LINK_STATES}`;

/**
 * Dashboard widget: the signed-in engineer's non-closed cases (their active
 * workload). Wraps the shared {@link CasesList} in a {@link SectionCard} and
 * paginates a small page of the list; the data is filtered server-side by
 * assignee + state in {@link useGetMyAssignedOpenCases}. "View all" jumps to
 * the full cases page with the same filter.
 */
export default function MyAssignedCases(): JSX.Element {
  const currentUser = useCurrentUser();
  const [page, setPage] = useState(0);
  const { data, isLoading, isError } = useGetMyAssignedOpenCases(
    page,
    MY_OPEN_CASES_PAGE_SIZE,
  );

  // `/users/me` returned but carried no id (entity service down): we can't tell
  // which cases are the caller's, so say so rather than showing nothing.
  const cannotIdentify = !currentUser.isLoading && !currentUser.user?.id;

  const loading = isLoading || currentUser.isLoading;
  const total = data?.total ?? 0;
  // Clamp to the last valid page when the queue shrinks (a case closed /
  // reassigned drops the total below the current offset). Guarded setState
  // during render is React's documented pattern for deriving from changed
  // inputs — not an effect. Mirrors CsmIssuesView.
  const lastPage = total === 0 ? 0 : Math.ceil(total / MY_OPEN_CASES_PAGE_SIZE) - 1;
  if (data !== undefined && page > lastPage) {
    setPage(lastPage);
  }
  // Once loaded, an empty queue is reported via the subtitle alone (CasesList's
  // own "no cases match the current filters" copy would be misleading here).
  const isEmpty = !loading && !isError && !cannotIdentify && total === 0;
  const subtitle =
    !loading && !isError && !cannotIdentify
      ? total === 0
        ? "You have no open cases."
        : `${total} open ${total === 1 ? "case" : "cases"} assigned to you`
      : undefined;

  return (
    <SectionCard
      title="Assigned to me"
      subtitle={subtitle}
      action={
        total > 0 ? (
          <Link
            component={RouterLink}
            to={VIEW_ALL_HREF}
            underline="hover"
            variant="body2"
          >
            View all
          </Link>
        ) : undefined
      }
    >
      {isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load your cases.
        </Typography>
      ) : cannotIdentify ? (
        <Typography variant="body2" color="text.secondary">
          We couldn't identify your account, so your assigned cases can't be
          loaded right now.
        </Typography>
      ) : isEmpty ? null : (
        <>
          <CasesList cases={data?.cases ?? []} isLoading={loading} />
          {/* Only pager past the first page when there's more than one page. */}
          {total > MY_OPEN_CASES_PAGE_SIZE && (
            <TablePagination
              component="div"
              count={data === undefined ? -1 : total}
              page={page}
              onPageChange={(_, next) => setPage(next)}
              rowsPerPage={MY_OPEN_CASES_PAGE_SIZE}
              rowsPerPageOptions={[]}
              showFirstButton
              showLastButton
              // The pager's toolbar is a fixed 52px tall by default, which
              // leaves an airy dead band under the list. Collapse it to its
              // content height and drop the left inset so it sits snug.
              sx={{
                "& .MuiTablePagination-toolbar": { minHeight: 0, pl: 0 },
              }}
            />
          )}
        </>
      )}
    </SectionCard>
  );
}
