import { useProject } from "@context/project";
import type { ModeType } from "@shared/types";
import { usePaginationSubtitleOverride } from "@shared/hooks/usePaginationSubtitle";
import { useCaseListFilters } from "@features/cases/hooks/useCaseListFilters";
import { useCaseList } from "@features/cases/hooks/useCaseList";
import { InfiniteScroll } from "@components/common";
import EmptyState from "@components/common/EmptyState";
import { Typography } from "@wso2/oxygen-ui";
import { Fragment, useEffect } from "react";
import { ItemCardExtended, ItemsListContentSkeleton } from "@components/support/ItemCardExtended";
import { ITEM_DETAIL_PATHS } from "@config/constants";
import { GroupAccordion } from "@components/ui/GroupAccordion";

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
  const filters = useCaseListFilters(mode, filter, search);

  const { query, total, count } = useCaseList(projectId!, filters);

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
              <Fragment key={pageIndex}>
                {page.map((item) => (
                  <ItemCardExtended key={item.id} type="case" to={ITEM_DETAIL_PATHS.case(item.id)} {...item} />
                ))}
              </Fragment>
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
