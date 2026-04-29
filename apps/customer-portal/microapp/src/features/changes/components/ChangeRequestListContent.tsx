import { useProject } from "@context/project";
import type { ModeType } from "@shared/types";
import { usePaginationSubtitleOverride } from "@shared/hooks/usePaginationSubtitle";
import { useChangeListFilters } from "@features/changes/hooks/useChangeListFilters";
import { useChangeRequestList } from "@features/changes/hooks/useChangeRequestList";
import { InfiniteScroll } from "@components/common";
import EmptyState from "@components/common/EmptyState";
import { Typography } from "@wso2/oxygen-ui";
import { ItemCardExtended, ItemsListContentSkeleton } from "@components/support/ItemCardExtended";
import { ITEM_DETAIL_PATHS } from "@config/constants";
import { GroupAccordion } from "@components/ui/GroupAccordion";
import { useEffect } from "react";

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
  const filters = useChangeListFilters(mode, filter, search);

  const { query, total, count } = useChangeRequestList(projectId!, filters);

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
