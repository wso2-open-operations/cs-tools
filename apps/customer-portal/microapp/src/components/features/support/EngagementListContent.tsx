import { useProject } from "@root/src/context/project";
import { ItemsListContentSkeleton, usePaginationSubtitleOverride } from "@root/src/pages/AllItemsPage";
import type { GetCasesRequestDto } from "@root/src/types";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { InfiniteScroll } from "../../shared";
import EmptyState from "../../shared/EmptyState";
import { Typography } from "@wso2/oxygen-ui";
import { ItemCardExtended } from "./ItemCardExtended";
import { ITEM_DETAIL_PATHS } from "@root/src/config/constants";
import { engagements } from "@root/src/services/engagements";

export function EngagementListContent({ filter, search }: { filter: string; search: string }) {
  const { projectId } = useProject();

  const filters: GetCasesRequestDto["filters"] = {};

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
}
