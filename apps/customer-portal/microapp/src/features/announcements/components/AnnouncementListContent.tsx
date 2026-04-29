import { useProject } from "@context/project";
import { useAnnouncementList } from "@features/announcements/hooks/useAnnouncementList";
import { InfiniteScroll } from "@components/common";
import EmptyState from "@components/common/EmptyState";
import { Typography } from "@wso2/oxygen-ui";
import { ItemCardExtended, ItemsListContentSkeleton } from "@components/support/ItemCardExtended";
import { ITEM_DETAIL_PATHS } from "@config/constants";
import { useLayout } from "@context/layout";
import { useLayoutEffect } from "react";

export function AnnouncementListContent({ filter, search }: { filter: string; search: string }) {
  const { setLayoutOverrides } = useLayout();
  const { projectId } = useProject();

  const filters: { statusIds?: number[]; searchQuery?: string } = {};

  if (filter !== "all") filters.statusIds = [Number(filter)];
  if (search) filters.searchQuery = search;

  const { query, total, count } = useAnnouncementList(projectId!, filters);

  useLayoutEffect(() => {
    setLayoutOverrides({ subtitle: count ? `${count} items` : null });

    return () => setLayoutOverrides({ subtitle: null });

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
