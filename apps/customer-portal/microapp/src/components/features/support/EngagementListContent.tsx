import { useProject } from "@root/src/context/project";
import { ItemsListContentSkeleton, usePaginationSubtitleOverride, type ModeType } from "@root/src/pages/AllItemsPage";
import type { GetCasesRequestDto } from "@root/src/types";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { InfiniteScroll } from "../../shared";
import EmptyState from "../../shared/EmptyState";
import { Typography } from "@wso2/oxygen-ui";
import { ItemCardExtended } from "./ItemCardExtended";
import { ITEM_DETAIL_PATHS } from "@root/src/config/constants";
import { engagements } from "@root/src/services/engagements";
import { useEffect } from "react";
import { GroupAccordion } from "../../ui/GroupAccordion";
import { useResolvedDateRange } from "@root/src/utils/useResolvedDateRange";

export function EngagementListContent({
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

  const filters: GetCasesRequestDto["filters"] = {};

  const resolvedDateRange = useResolvedDateRange(mode);

  if (mode) {
    switch (mode.type) {
      case "status":
        switch (mode.status) {
          case "action_required":
            filters.statusIds = [18, 6];
            break;

          case "outstanding":
            filters.statusIds = [1, 10, 6, 1006];
            break;

          case "resolved":
            filters.statusIds = [3];
            filters.closedStartDate = resolvedDateRange?.closedStartDate;
            filters.closedEndDate = resolvedDateRange?.closedEndDate;
            break;
        }
    }
  }

  if (filter !== "all") {
    filters.statusIds = [Number(filter)];
  }

  if (search) {
    filters.searchQuery = search;
  }

  const totalQuery = useQuery(engagements.all(projectId!));
  const query = useInfiniteQuery(engagements.paginated(projectId!, { filters }));

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
                <ItemCardExtended
                  key={item.id}
                  type="engagement"
                  to={ITEM_DETAIL_PATHS.engagement(item.id)}
                  {...item}
                />
              )),
            )}
        </>
      )}
    </InfiniteScroll>
  );

  if (grouped)
    return (
      <GroupAccordion type="engagement" count={count}>
        {body}
      </GroupAccordion>
    );

  return body;
}
