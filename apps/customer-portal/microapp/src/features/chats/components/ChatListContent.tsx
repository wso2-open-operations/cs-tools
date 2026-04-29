import { useProject } from "@context/project";
import { usePaginationSubtitleOverride } from "@shared/hooks/usePaginationSubtitle";
import { useChatList } from "@features/chats/hooks/useChatList";
import { InfiniteScroll } from "@components/common";
import EmptyState from "@components/common/EmptyState";
import { Typography } from "@wso2/oxygen-ui";
import { ItemCardExtended, ItemsListContentSkeleton } from "@components/support/ItemCardExtended";
import { ITEM_DETAIL_PATHS } from "@config/constants";

export function ChatListContent({ filter, search }: { filter: string; search: string }) {
  const { projectId } = useProject();

  const filters: { stateKeys?: number[]; searchQuery?: string } = {};

  if (filter !== "all") filters.stateKeys = [Number(filter)];
  if (search) filters.searchQuery = search;

  const { query, total, count } = useChatList(projectId!, filters);

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
