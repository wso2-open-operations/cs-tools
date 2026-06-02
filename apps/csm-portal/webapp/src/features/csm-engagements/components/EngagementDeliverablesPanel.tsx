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

import { Box, Card, Chip, Tooltip, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import {
  ENGAGEMENT_DELIVERABLE_STATUS_COLOR,
  ENGAGEMENT_DELIVERABLE_STATUS_LABEL,
  formatDateOnly,
} from "@features/csm-engagements/utils/engagements";
import type { CsmEngagementDeliverable } from "@features/csm-engagements/types/csmEngagements";

interface EngagementDeliverablesPanelProps {
  deliverables: CsmEngagementDeliverable[];
}

export default function EngagementDeliverablesPanel({
  deliverables,
}: EngagementDeliverablesPanelProps): JSX.Element {
  const accepted = deliverables.filter((d) => d.status === "accepted").length;
  return (
    <Card variant="outlined" sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Typography variant="subtitle2">Deliverables</Typography>
        <Chip
          size="small"
          variant="outlined"
          color="success"
          label={`${accepted}/${deliverables.length} accepted`}
        />
      </Box>
      {deliverables.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No deliverables defined.
        </Typography>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {deliverables.map((d) => (
            <Box
              key={d.id}
              sx={{
                p: 1.25,
                border: 1,
                borderRadius: 1,
                borderColor: "divider",
                display: "flex",
                alignItems: "flex-start",
                gap: 1.5,
                flexWrap: "wrap",
              }}
            >
              <Box sx={{ flex: 1, minWidth: 220 }}>
                <Typography variant="body2" sx={{ fontWeight: 500 }}>
                  {d.name}
                </Typography>
                {d.waiverReason && (
                  <Typography variant="caption" color="warning.main">
                    Waived: {d.waiverReason}
                  </Typography>
                )}
              </Box>
              <Chip
                size="small"
                variant="outlined"
                color={ENGAGEMENT_DELIVERABLE_STATUS_COLOR[d.status]}
                label={ENGAGEMENT_DELIVERABLE_STATUS_LABEL[d.status]}
              />
              <Tooltip title={d.completedAt ? "Completed" : "Due date"}>
                <Typography variant="caption" color="text.secondary">
                  {d.completedAt
                    ? `Done ${formatDateOnly(d.completedAt)}`
                    : `Due ${formatDateOnly(d.dueDate)}`}
                </Typography>
              </Tooltip>
            </Box>
          ))}
        </Box>
      )}
    </Card>
  );
}
