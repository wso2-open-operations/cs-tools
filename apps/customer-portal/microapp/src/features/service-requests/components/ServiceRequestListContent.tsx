import { useEffect } from "react";

import { Typography } from "@wso2/oxygen-ui";

import { useProject } from "@context/project";

import { ITEM_DETAIL_PATHS } from "@config/constants";

import { useServiceRequestList } from "@features/service-requests/hooks/useServiceRequestList";
import { useServiceRequestListFilters } from "@features/service-requests/hooks/useServiceRequestListFilters";

import { usePaginationSubtitleOverride } from "@shared/hooks/usePaginationSubtitle";

import type { ModeType } from "@shared/types";

import { InfiniteScroll } from "@components/common";
import EmptyState from "@components/common/EmptyState";
import { ItemCardExtended, ItemsListContentSkeleton } from "@components/support/ItemCardExtended";
import { GroupAccordion } from "@components/ui/GroupAccordion";

export function ServiceRequestListContent({
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
  const filters = useServiceRequestListFilters(mode, filter, search);

  const { query, total, count } = useServiceRequestList(projectId!, filters);

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
                <ItemCardExtended key={item.id} type="service" to={ITEM_DETAIL_PATHS.service(item.id)} {...item} />
              )),
            )}
        </>
      )}
    </InfiniteScroll>
  );

  if (grouped)
    return (
      <GroupAccordion type="service" count={count}>
        {body}
      </GroupAccordion>
    );

  return body;
}
