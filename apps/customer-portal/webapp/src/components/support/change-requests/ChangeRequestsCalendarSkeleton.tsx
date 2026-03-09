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

import { Box, Skeleton, Paper, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";

const DAYS_OF_WEEK = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

/**
 * Skeleton loading component for Change Requests calendar view.
 *
 * @returns {JSX.Element} The rendered skeleton.
 */
export default function ChangeRequestsCalendarSkeleton(): JSX.Element {
  return (
    <Paper sx={{ p: 0, overflow: "hidden" }}>
      {/* Header */}
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          p: 2,
          borderBottom: 1,
          borderColor: "divider",
        }}
      >
        <Skeleton variant="rounded" width={40} height={40} />
      </Box>

      {/* Days of week header */}
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: "repeat(7, 1fr)",
          borderBottom: 1,
          borderColor: "divider",
        }}
      >
        {DAYS_OF_WEEK.map((day) => (
          <Box
            key={day}
            sx={{
              p: 1,
              textAlign: "center",
            }}
          >
            <Typography variant="body2" fontWeight={500} color="text.secondary">
              {day}
            </Typography>
          </Box>
        ))}
      </Box>

      {/* Calendar grid */}
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: "repeat(7, 1fr)",
        }}
      >
        {[...Array(35)].map((_, index) => (
          <Box
            key={index}
            sx={{
              minHeight: 128,
              p: 1,
            }}
          >
            <Skeleton variant="text" width={20} height={20} sx={{ mb: 1 }} />
            {index % 3 === 0 && (
              <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
                <Skeleton variant="rounded" width="100%" height={45} />
                {index % 5 === 0 && (
                  <Skeleton variant="rounded" width="100%" height={45} />
                )}
              </Box>
            )}
          </Box>
        ))}
      </Box>
    </Paper>
  );
}
