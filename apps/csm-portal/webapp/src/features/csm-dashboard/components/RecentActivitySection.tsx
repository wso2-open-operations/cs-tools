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

import { Box, Chip, Skeleton, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import SectionCard from "@features/csm-dashboard/components/SectionCard";
import RelativeTime from "@components/RelativeTime";
import type {
  CsmRecentActivity,
  CsmRecentActivityType,
} from "@features/csm-dashboard/types/abtDashboard";

interface RecentActivitySectionProps {
  activity?: CsmRecentActivity[];
  isLoading: boolean;
}

const TYPE_LABEL: Record<CsmRecentActivityType, string> = {
  comment: "Comment",
  state_change: "State change",
  case_created: "Created",
  case_closed: "Closed",
  sla_breach: "SLA breach",
};

const TYPE_COLOR: Record<
  CsmRecentActivityType,
  "default" | "primary" | "success" | "warning" | "error" | "info"
> = {
  comment: "info",
  state_change: "default",
  case_created: "primary",
  case_closed: "success",
  sla_breach: "error",
};

export default function RecentActivitySection({
  activity,
  isLoading,
}: RecentActivitySectionProps): JSX.Element {
  return (
    <SectionCard title="Recent activity" subtitle="Across ABT cases">
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.25 }}>
        {isLoading &&
          [0, 1, 2, 3].map((i) => (
            <Skeleton key={i} variant="rectangular" height={44} />
          ))}
        {!isLoading && (activity?.length ?? 0) === 0 && (
          <Typography variant="body2" color="text.secondary">
            No recent activity.
          </Typography>
        )}
        {!isLoading &&
          activity?.map((a) => (
            <Box
              key={a.id}
              sx={{
                display: "flex",
                alignItems: "flex-start",
                gap: 1.5,
                p: 1.25,
                border: 1,
                borderColor: "divider",
                borderRadius: 1,
              }}
            >
              <Chip
                size="small"
                color={TYPE_COLOR[a.type]}
                label={TYPE_LABEL[a.type]}
                sx={{ minWidth: 96 }}
              />
              <Box sx={{ flex: 1, minWidth: 0 }}>
                <Typography variant="body2">
                  <strong>{a.caseNumber}</strong> · {a.summary}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {a.customer} · {a.who} · <RelativeTime iso={a.whenAt} />
                </Typography>
              </Box>
            </Box>
          ))}
      </Box>
    </SectionCard>
  );
}
