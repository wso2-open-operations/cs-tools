import { Timeline } from "@mui/lab";

import { useFilters } from "@context/filters";

import { ProgressStep, ProgressStepSkeleton } from "@features/detail/components";
import { useChangeRequest } from "@features/detail/hooks";

import { PROGRESS_TIMELINE_META, PROGRESS_TIMELINE_RESOLVED_STAGES_FROM } from "@shared/constants";

export function ProgressTimeline() {
  const { data, isLoading } = useChangeRequest();
  const { data: filters, isLoading: isFiltersLoading } = useFilters();

  const hasResolvedData = !isLoading && !isFiltersLoading && data && filters;
  const status = filters?.changeRequestStates.find((s) => s.id === data?.statusId)?.label;
  const activeIndex = status ? PROGRESS_TIMELINE_META.findIndex((e) => e.title === status) : -1;

  return (
    <Timeline position="right" sx={{ p: 0, "& .MuiTimelineItem-root:before": { flex: 0, padding: 0 } }}>
      {PROGRESS_TIMELINE_META.map(({ title, description }, index) => {
        const last = index === PROGRESS_TIMELINE_META.length - 1;
        if (!hasResolvedData) return <ProgressStepSkeleton key={index} last={last} />;

        return (
          <ProgressStep
            key={index}
            title={title}
            description={description}
            status={((i) => {
              if (i === activeIndex) return "active";
              if (i < activeIndex && i < PROGRESS_TIMELINE_RESOLVED_STAGES_FROM) return "completed";
              return "pending";
            })(index)}
            fill={index === 3 ? (data.hasCustomerApproved ? "green" : "red") : undefined}
            end={index >= PROGRESS_TIMELINE_RESOLVED_STAGES_FROM}
            last={last}
          />
        );
      })}
    </Timeline>
  );
}
