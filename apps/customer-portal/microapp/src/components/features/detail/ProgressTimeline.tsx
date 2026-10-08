import { TIMELINE_META } from "@root/src/config/constants";
import { ProgressTimelineEntrySkeleton, TimelineEntry } from "./TimelineEntry";
import { Timeline } from "../../ui";
import type { CasesFiltersDto, ChangeRequest } from "@root/src/types";
import { useQuery } from "@tanstack/react-query";
import { changeRequests } from "@root/src/services/changes";
import { useFilters } from "@root/src/context/filters";
import { timelineStepStates } from "@root/src/utils/changeRequestProgress";

interface ProgressTimelineViewProps {
  hasCustomerApproved?: boolean;
  activeIndex: number;
}

function ProgressTimelineView({ hasCustomerApproved, activeIndex }: ProgressTimelineViewProps) {
  const steps = timelineStepStates(
    TIMELINE_META.map((step) => step.title),
    activeIndex,
    !!hasCustomerApproved,
  );

  return (
    <Timeline>
      {TIMELINE_META.map((step, index) => (
        <TimelineEntry
          key={index}
          variant="progress"
          status={steps[index].status}
          title={step.title}
          description={step.description}
          fill={steps[index].fill}
          end={steps[index].end}
          last={steps[index].last}
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

  // A time WSO2 accepted (and the change moved on because of it) counts as the customer's approval here: the flag
  // itself stays false. The Customer Approval step is green for either, never red for a step still to come or still
  // waiting (see `customerApprovalStepFill`).
  return (
    <ProgressTimelineView
      hasCustomerApproved={data.hasCustomerApproved || data.isProposedTimeAccepted}
      activeIndex={activeIndex}
    />
  );
}
