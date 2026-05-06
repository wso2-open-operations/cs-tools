import { useProject } from "@root/src/context/project";
import { ItemsListContentSkeleton } from "@root/src/pages/AllItemsPage";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { InfiniteScroll } from "../../shared";
import EmptyState from "../../shared/EmptyState";
import { Typography } from "@wso2/oxygen-ui";
import { ItemCardExtended } from "./ItemCardExtended";
import { ITEM_DETAIL_PATHS } from "@root/src/config/constants";
import { announcements } from "@root/src/services/announcements";
import type { GetCasesRequestDto } from "@root/src/types";
import { useLayout } from "@root/src/context/layout";
import { useLayoutEffect } from "react";

export function AnnouncementListContent({ filter, search }: { filter: string; search: string }) {
  const { setSubtitleSlotOverride } = useLayout();
  const { projectId } = useProject();

  const filters: GetCasesRequestDto["filters"] = {};

  if (filter !== "all") {
    filters.statusIds = [Number(filter)];
  }

  if (search) {
    filters.searchQuery = search;
  }

  const totalQuery = useQuery(announcements.all(projectId!));
  const query = useInfiniteQuery(announcements.paginated(projectId!, { filters }));

  const total = totalQuery.data?.pagination.totalRecords;
  const count = query.data?.pages[0].pagination.totalRecords;

  useLayoutEffect(() => {
    setSubtitleSlotOverride(count ? `${count} items` : null);

    return () => setSubtitleSlotOverride(null);

    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [total]);

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
            data.pages.map((page) =>
              page.map((item) => (
                <ItemCardExtended
                  key={item.id}
                  type="announcement"
                  to={ITEM_DETAIL_PATHS.announcement(item.id)}
                  {...item}
                />
              )),
            )}
        </>
      )}
    </InfiniteScroll>
  );
}
