import { useProject } from "@root/src/context/project";
import { ItemsListContentSkeleton, usePaginationSubtitleOverride } from "@root/src/pages/AllItemsPage";
import { changeRequests } from "@root/src/services/changes";
import type { GetChangeRequestsRquestDto } from "@root/src/types";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { InfiniteScroll } from "../../shared";
import EmptyState from "../../shared/EmptyState";
import { Typography } from "@wso2/oxygen-ui";
import { ItemCardExtended } from "./ItemCardExtended";
import { ITEM_DETAIL_PATHS } from "@root/src/config/constants";

export function ChangeRequestListContent({ filter, search }: { filter: string; search: string }) {
  const { projectId } = useProject();

  const filters: GetChangeRequestsRquestDto["filters"] = {};

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
                <ItemCardExtended key={item.id} type="change" to={ITEM_DETAIL_PATHS.change(item.id)} {...item} />
              )),
            )}
        </>
      )}
    </InfiniteScroll>
  );
}
