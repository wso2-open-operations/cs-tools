import { useProject } from "@root/src/context/project";
import { ItemsListContentSkeleton, usePaginationSubtitleOverride } from "@root/src/pages/AllItemsPage";
import { chats } from "@root/src/services/chats";
import type { GetChatsRequestDto } from "@root/src/types/chat.dto";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { InfiniteScroll } from "../../shared";
import EmptyState from "../../shared/EmptyState";
import { Typography } from "@wso2/oxygen-ui";
import { ItemCardExtended } from "./ItemCardExtended";
import { ITEM_DETAIL_PATHS } from "@root/src/config/constants";

export function ChatListContent({ filter, search }: { filter: string; search: string }) {
  const { projectId } = useProject();

  const filters: GetChatsRequestDto["filters"] = {};

  if (filter !== "all") {
    filters.stateKeys = [Number(filter)];
  }

  if (search) {
    filters.searchQuery = search;
  }
  const totalQuery = useQuery(chats.all(projectId!));
  const query = useInfiniteQuery(chats.paginated(projectId!, { filters }));

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
                <ItemCardExtended key={item.id} type="chat" to={ITEM_DETAIL_PATHS.chat(item.id)} {...item} />
              )),
            )}
        </>
      )}
    </InfiniteScroll>
  );
}
