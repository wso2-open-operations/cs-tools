import { useProject } from "@root/src/context/project";
import { ItemsListContentSkeleton, usePaginationSubtitleOverride, type ModeType } from "@root/src/pages/AllItemsPage";
import { changeRequests } from "@root/src/services/changes";
import type { GetChangeRequestsRquestDto } from "@root/src/types";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { InfiniteScroll } from "../../shared";
import EmptyState from "../../shared/EmptyState";
import { Typography } from "@wso2/oxygen-ui";
import { ItemCardExtended } from "./ItemCardExtended";
import {
  ACTION_REQUIRED_CHANGE_REQUEST_STATUS_IDS,
  ITEM_DETAIL_PATHS,
  OUTSTANDING_CHANGE_REQUESTS_STATUS_IDS,
  RESOLVED_CHANGE_REQUEST_STATUS_IDS,
} from "@root/src/config/constants";
import { GroupAccordion } from "../../ui/GroupAccordion";
import { useEffect } from "react";
import { useResolvedDateRange } from "@root/src/utils/useResolvedDateRange";

export function ChangeRequestListContent({
  filter,
  search,
  mode,
  grouped = false,
  onCountChange,
}: {
  filter: string;
  search: string;
  mode?: ModeType;
  grouped?: boolean;
  onCountChange?: (count: number | undefined) => void;
}) {
  const { projectId } = useProject();

  const filters: GetChangeRequestsRquestDto["filters"] = {};

  const resolvedDateRange = useResolvedDateRange(mode);

  if (mode) {
    switch (mode.type) {
      case "status":
        switch (mode.status) {
          case "action_required":
            filters.stateKeys = ACTION_REQUIRED_CHANGE_REQUEST_STATUS_IDS;
            break;

          case "outstanding":
            filters.stateKeys = OUTSTANDING_CHANGE_REQUESTS_STATUS_IDS;
            break;

          case "resolved": {
            const now = new Date();
            const past = new Date();
            past.setDate(now.getDate() - 30);

            filters.stateKeys = RESOLVED_CHANGE_REQUEST_STATUS_IDS;
            filters.closedStartDate = resolvedDateRange?.closedStartDate;
            filters.closedEndDate = resolvedDateRange?.closedEndDate;
            break;
          }
        }
    }
  }

  if (filter !== "all") {
    filters.stateKeys = [Number(filter)];
  }

  if (search) {
    filters.searchQuery = search;
  }

  const totalQuery = useQuery(changeRequests.all(projectId!));
  const query = useInfiniteQuery(changeRequests.paginated(projectId!, { filters }));

  const total = totalQuery.data?.pagination.totalRecords;
  const count = query.data?.pages[0].pagination.totalRecords;

  usePaginationSubtitleOverride(grouped ? null : count, grouped ? null : total);

  useEffect(() => {
    onCountChange?.(count);
  }, [count]);

  const body = (
    <InfiniteScroll
      {...query}
      sentinel={<ItemsListContentSkeleton />}
      tail={
        grouped ? undefined : count === 0 ? (
          <EmptyState />
        ) : (
          <Typography variant="subtitle2" textAlign="center">
            You're all caught up!
          </Typography>
        )
      }
    >
      {(data) => (
        <>
          {data &&
            data.pages.map((page) =>
              page.map((item) => (
                <ItemCardExtended key={item.id} type="change" to={ITEM_DETAIL_PATHS.change(item.id)} {...item} />
              )),
            )}
        </>
      )}
    </InfiniteScroll>
  );

  if (grouped)
    return (
      <GroupAccordion type="change" count={count}>
        {body}
      </GroupAccordion>
    );

  return body;
}
