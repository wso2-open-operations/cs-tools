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

import { Box, Card, Typography } from "@wso2/oxygen-ui";
import { Check, Circle } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import {
  ENGAGEMENT_STAGE_LABEL,
  ENGAGEMENT_STAGE_ORDER,
} from "@features/csm-engagements/utils/engagements";
import type { CsmEngagementDetail } from "@features/csm-engagements/types/csmEngagements";

interface EngagementStagesTimelineProps {
  engagement: CsmEngagementDetail;
}

export default function EngagementStagesTimeline({
  engagement,
}: EngagementStagesTimelineProps): JSX.Element {
  const stages = ENGAGEMENT_STAGE_ORDER;
  return (
    <Card variant="outlined" sx={{ p: 2.5 }}>
      <Typography variant="subtitle2" sx={{ mb: 2 }}>
        Stages
      </Typography>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: `repeat(${stages.length}, minmax(0, 1fr))` },
          gap: 1.5,
          alignItems: "stretch",
        }}
      >
        {stages.map((s, idx) => {
          const status = engagement.stageStatus[s] ?? "not_started";
          const completed = status === "completed";
          const active = status === "in_progress";
          const color = completed ? "success.main" : active ? "primary.main" : "text.disabled";
          return (
            <Box
              key={s}
              sx={{
                p: 1.5,
                border: 1,
                borderRadius: 1,
                borderColor: completed || active ? color : "divider",
                bgcolor: active ? "primary.50" : "transparent",
                display: "flex",
                flexDirection: "column",
                gap: 0.5,
                position: "relative",
              }}
            >
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Box sx={{ color, display: "inline-flex" }}>
                  {completed ? (
                    <Check size={16} />
                  ) : (
                    <Circle size={12} fill={active ? "currentColor" : "none"} />
                  )}
                </Box>
                <Typography variant="caption" color="text.secondary">
                  Stage {idx + 1}
                </Typography>
              </Box>
              <Typography
                variant="body2"
                sx={{ fontWeight: completed || active ? 600 : 400 }}
              >
                {ENGAGEMENT_STAGE_LABEL[s]}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {status === "completed"
                  ? "Completed"
                  : status === "in_progress"
                  ? "In progress"
                  : "Not started"}
              </Typography>
            </Box>
          );
        })}
      </Box>
    </Card>
  );
}
