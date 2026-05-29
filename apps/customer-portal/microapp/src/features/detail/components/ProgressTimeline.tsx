// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
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
