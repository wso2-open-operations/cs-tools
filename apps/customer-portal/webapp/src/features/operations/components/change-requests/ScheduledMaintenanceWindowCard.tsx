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

import { type JSX, useMemo } from "react";
import { Box, Paper, Typography, Divider, colors } from "@wso2/oxygen-ui";
import { Calendar, Clock } from "@wso2/oxygen-ui-icons-react";
import type { ScheduledMaintenanceWindowCardProps } from "@features/operations/types/changeRequests";
import {
  formatChangeRequestDisplayDate,
  formatChangeRequestDuration,
} from "@features/operations/utils/changeRequests";


/**
 * Card displaying scheduled maintenance window (Planned Start, Planned End, Duration).
 * Schedule changes use Propose New Implementation Time from the change request header.
 *
 * @param {ScheduledMaintenanceWindowCardProps} props - Change request and optional action.
 * @returns {JSX.Element} The rendered card.
 */
export default function ScheduledMaintenanceWindowCard({
  changeRequest,
}: ScheduledMaintenanceWindowCardProps): JSX.Element {
  const durationText = useMemo(() => {
    const duration = (changeRequest as { duration?: string | number | null })
      .duration;
    if (duration == null) return "Not available";
    const mins =
      typeof duration === "number" ? duration : parseInt(String(duration), 10);
    return Number.isNaN(mins)
      ? "Not available"
      : formatChangeRequestDuration(mins);
  }, [changeRequest]);

  return (
    <Paper
      variant="outlined"
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 2,
      }}
    >
      <Box sx={{ px: 3, pt: 3 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 0.5 }}>
          <Calendar size={20} color={colors.grey[600]} aria-hidden />
          <Typography variant="h6" color="text.primary">
            Scheduled Maintenance Window
          </Typography>
        </Box>
      </Box>

      <Box sx={{ px: 3, pb: 3 }}>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
            gap: 3,
          }}
        >
          <Box>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
              Planned Start
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Clock size={16} color={colors.grey[400]} aria-hidden />
              <Typography variant="body2" color="text.primary">
                {formatChangeRequestDisplayDate(changeRequest?.startDate)}
              </Typography>
            </Box>
          </Box>

          <Box>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
              Planned End
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Clock size={16} color={colors.grey[400]} aria-hidden />
              <Typography variant="body2" color="text.primary">
                {formatChangeRequestDisplayDate(changeRequest?.endDate)}
              </Typography>
            </Box>
          </Box>
        </Box>

        <Divider sx={{ my: 2 }} />

        <Box>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
            Duration
          </Typography>
          <Typography variant="body2" color="text.primary">
            {durationText}
          </Typography>
        </Box>
      </Box>
    </Paper>
  );
}
