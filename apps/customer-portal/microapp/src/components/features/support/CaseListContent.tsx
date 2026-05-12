import { useProject } from "@root/src/context/project";
import { ItemsListContentSkeleton, usePaginationSubtitleOverride, type ModeType } from "@root/src/pages/AllItemsPage";
import { cases } from "@root/src/services/cases";
import type { GetCasesRequestDto } from "@root/src/types";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { InfiniteScroll } from "../../shared";
import EmptyState from "../../shared/EmptyState";
import { Typography } from "@wso2/oxygen-ui";
import React, { useEffect } from "react";
import { ItemCardExtended } from "./ItemCardExtended";
import {
  ACTION_REQUIRED_CASE_STATUS_IDS,
  ITEM_DETAIL_PATHS,
  OUTSTANDING_CASE_STATUS_IDS,
  RESOLVED_CASE_STATUS_IDS,
} from "@root/src/config/constants";
import { GroupAccordion } from "../../ui/GroupAccordion";
import { useResolvedDateRange } from "@root/src/utils/useResolvedDateRange";

export function CaseListContent({
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
            filters.statusIds = ACTION_REQUIRED_CASE_STATUS_IDS;
            break;

          case "outstanding":
            filters.statusIds = OUTSTANDING_CASE_STATUS_IDS;
            break;

          case "resolved": {
            const now = new Date();
            const past = new Date();
            past.setDate(now.getDate() - 30);

            filters.statusIds = RESOLVED_CASE_STATUS_IDS;
            filters.closedStartDate = resolvedDateRange?.closedStartDate;
            filters.closedEndDate = resolvedDateRange?.closedEndDate;
            break;
          }
        }
        break;

      case "severity":
        switch (mode.type) {
          case "severity":
            filters.statusIds = OUTSTANDING_CASE_STATUS_IDS;
            filters.severityId = Number(mode.id);
        }
    }
  }

  if (filter !== "all") {
    filters.statusIds = [Number(filter)];
  }

  if (search) {
    filters.searchQuery = search;
  }

  const totalQuery = useQuery(cases.all(projectId!));
  const query = useInfiniteQuery(cases.paginated(projectId!, { filters }));

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
            data.pages.map((page, pageIndex) => (
              <React.Fragment key={pageIndex}>
                {page.map((item) => (
                  <ItemCardExtended key={item.id} type="case" to={ITEM_DETAIL_PATHS.case(item.id)} {...item} />
                ))}
              </React.Fragment>
            ))}
        </>
      )}
    </InfiniteScroll>
  );

  if (grouped)
    return (
      <GroupAccordion type="case" count={count}>
        {body}
      </GroupAccordion>
    );

  return body;
}
