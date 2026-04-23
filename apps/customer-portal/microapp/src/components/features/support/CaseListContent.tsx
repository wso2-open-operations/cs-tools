import { useProject } from "@root/src/context/project";
import { ItemsListContentSkeleton, usePaginationSubtitleOverride, type ModeType } from "@root/src/pages/AllItemsPage";
import { cases } from "@root/src/services/cases";
import type { GetCasesRequestDto } from "@root/src/types";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { InfiniteScroll } from "../../shared";
import EmptyState from "../../shared/EmptyState";
import { Typography } from "@wso2/oxygen-ui";
import React from "react";
import { ItemCardExtended } from "./ItemCardExtended";
import { ITEM_DETAIL_PATHS } from "@root/src/config/constants";

export function CaseListContent({ filter, search, mode }: { filter: string; search: string; mode?: ModeType }) {
  const { projectId } = useProject();

  const filters: GetCasesRequestDto["filters"] = {};

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

  const totalQuery = useQuery(cases.all(projectId!));
  const query = useInfiniteQuery(cases.paginated(projectId!, { filters }));

  const total = totalQuery.data?.pagination.totalRecords;
  const count = query.data?.pages[0].pagination.totalRecords;

  usePaginationSubtitleOverride(count, total);

  return (
    <InfiniteScroll
      {...query}
      sentinel={<ItemsListContentSkeleton />}
      tail={
        count === 0 ? (
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
}
