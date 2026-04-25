import { TIMELINE_META } from "@root/src/config/constants";
import { ProgressTimelineEntrySkeleton, TimelineEntry } from "./TimelineEntry";
import { Timeline } from "../../ui";
import type { CasesFiltersDto, ChangeRequest } from "@root/src/types";
import { useQuery } from "@tanstack/react-query";
import { changeRequests } from "@root/src/services/changes";
import { useFilters } from "@root/src/context/filters";

interface ProgressTimelineViewProps {
  hasCustomerApproved?: boolean;
  activeIndex: number;
}

function ProgressTimelineView({ hasCustomerApproved, activeIndex }: ProgressTimelineViewProps) {
  return (
    <Timeline>
      {TIMELINE_META.map((step, index) => (
        <TimelineEntry
          key={index}
          variant="progress"
          status={
            index >= TIMELINE_META.length - 3
              ? index === activeIndex
                ? "active"
                : "pending"
              : index === activeIndex
                ? "active"
                : index < activeIndex
                  ? "completed"
                  : "pending"
          }
          title={step.title}
          description={step.description}
          fill={index === 3 ? (hasCustomerApproved ? "green" : "red") : undefined}
          end={index > TIMELINE_META.length - 4}
          last={index === TIMELINE_META.length - 1}
        />
      ))}
    </Timeline>
  );
}

function ProgressTimelineSkeletonView() {
  return (
    <Timeline>
      {TIMELINE_META.map((_, index) => (
        <ProgressTimelineEntrySkeleton key={index} last={index === TIMELINE_META.length - 1} />
      ))}
    </Timeline>
  );
}

export function useProgressTimelineModel(data?: ChangeRequest, filters?: CasesFiltersDto) {
  const status = filters?.changeRequestStates.find((t) => t.id === data?.statusId)?.label;

  const activeIndex = status ? TIMELINE_META.findIndex((e) => e.title === status) : -1;

  return {
    activeIndex,
  };
}

export function ProgressTimeline({ id }: { id: string }) {
  const { data, isLoading } = useQuery(changeRequests.get(id));
  const { data: filters, isLoading: isFiltersLoading } = useFilters();
  const { activeIndex } = useProgressTimelineModel(data, filters);

  if (isLoading || isFiltersLoading || !data || !filters) return <ProgressTimelineSkeletonView />;

  return <ProgressTimelineView activeIndex={activeIndex} />;
}
