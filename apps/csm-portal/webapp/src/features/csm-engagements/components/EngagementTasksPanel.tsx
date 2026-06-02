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

import { Box, Card, Chip, Typography, useTheme } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import {
  ENGAGEMENT_STAGE_LABEL,
  ENGAGEMENT_TASK_STATE_COLOR,
  ENGAGEMENT_TASK_STATE_LABEL,
  formatDateOnly,
} from "@features/csm-engagements/utils/engagements";
import type { CsmEngagementTask } from "@features/csm-engagements/types/csmEngagements";

interface EngagementTasksPanelProps {
  tasks: CsmEngagementTask[];
}

export default function EngagementTasksPanel({
  tasks,
}: EngagementTasksPanelProps): JSX.Element {
  const theme = useTheme();
  const counts = {
    total: tasks.length,
    completed: tasks.filter((t) => t.state === "completed").length,
    blocked: tasks.filter((t) => t.state === "blocked").length,
  };

  return (
    <Card variant="outlined" sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Typography variant="subtitle2">Tasks</Typography>
        <Chip size="small" variant="outlined" label={`${counts.completed}/${counts.total} done`} />
        {counts.blocked > 0 && (
          <Chip size="small" color="error" variant="outlined" label={`${counts.blocked} blocked`} />
        )}
      </Box>
      {tasks.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No tasks yet.
        </Typography>
      ) : (
        <Box
          sx={{
            border: 1,
            borderColor: "divider",
            borderRadius: 1,
            overflow: "hidden",
            display: "grid",
            gridTemplateColumns:
              "minmax(280px, 3fr) minmax(140px, 1fr) auto minmax(140px, 1fr) minmax(100px, 0.8fr)",
            columnGap: 2,
          }}
        >
          <Box
            sx={{
              gridColumn: "1 / -1",
              display: "grid",
              gridTemplateColumns: "subgrid",
              columnGap: 2,
              alignItems: "center",
              px: 2,
              py: 1.25,
              bgcolor: "action.hover",
              borderBottom: 1,
              borderColor: "divider",
            }}
          >
            {["Task", "Stage", "State", "Assignee", "Due"].map((h) => (
              <Typography
                key={h}
                variant="caption"
                color="text.secondary"
                sx={{ fontWeight: 600 }}
              >
                {h}
              </Typography>
            ))}
          </Box>
          {tasks.map((t) => (
            <Box
              key={t.id}
              sx={{
                gridColumn: "1 / -1",
                display: "grid",
                gridTemplateColumns: "subgrid",
                columnGap: 2,
                alignItems: "center",
                px: 2,
                py: 1.25,
                borderBottom: 1,
                borderColor: "divider",
                "&:last-of-type": { borderBottom: 0 },
                "&:hover": { bgcolor: theme.palette.action.hover },
              }}
            >
              <Typography variant="body2">{t.title}</Typography>
              <Typography variant="caption" color="text.secondary">
                {ENGAGEMENT_STAGE_LABEL[t.stage]}
              </Typography>
              <Chip
                size="small"
                variant="outlined"
                color={ENGAGEMENT_TASK_STATE_COLOR[t.state]}
                label={ENGAGEMENT_TASK_STATE_LABEL[t.state]}
              />
              <Typography variant="body2">{t.assigneeName ?? "—"}</Typography>
              <Typography variant="caption" color="text.secondary">
                {formatDateOnly(t.dueDate)}
              </Typography>
            </Box>
          ))}
        </Box>
      )}
    </Card>
  );
}
