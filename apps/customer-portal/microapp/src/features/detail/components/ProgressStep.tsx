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
import { TimelineConnector, TimelineContent, TimelineItem, TimelineSeparator } from "@mui/lab";
import { Box, pxToRem, Skeleton, Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import { Circle, CircleCheck, CircleDot } from "@wso2/oxygen-ui-icons-react";

type StepStatus = "completed" | "active" | "pending";

export interface ProgressStepProps {
  /** The main label displayed for this step. */
  title: string;
  /** Supporting text shown below the title. */
  description: string;
  /** Controls which indicator icon and connector color is rendered. */
  status: StepStatus;
  /** Overrides the default primary color of the indicator icon. Useful for conveying approval state (e.g. green/red). */
  fill?: string;
  /**
   * When true, the connector below this step will not be highlighted regardless of status.
   * Used for steps at the tail end of the timeline that should never appear as progressed.
   */
  end?: boolean;
  /** When true, the connector below this step is hidden. Should be set on the final step. */
  last?: boolean;
}

export function ProgressStep({
  title,
  description,
  status,
  fill: fillOverride,
  end = false,
  last = false,
}: ProgressStepProps) {
  const theme = useTheme();
  const fill = fillOverride ?? theme.palette.primary.main;
  const completed = status === "completed";
  const active = status === "active";

  return (
    <TimelineItem sx={{ minHeight: "auto" }}>
      <TimelineSeparator>
        <Box color={completed || active ? "primary.contrastText" : "text.disabled"}>
          {completed && <CircleCheck size={pxToRem(24)} fill={fill} />}
          {active && <CircleDot size={pxToRem(24)} fill={fill} />}
          {!completed && !active && <Circle size={pxToRem(20)} />}
        </Box>

        {!last && <TimelineConnector sx={{ bgcolor: completed && !end ? "primary.main" : undefined }} />}
      </TimelineSeparator>

      <TimelineContent sx={{ p: 0, pb: last ? 0 : 3 }} ml={1.5}>
        <Stack gap={0.5}>
          <Typography variant="body2" fontWeight="medium">
            {title}
          </Typography>

          <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
            {description}
          </Typography>
        </Stack>
      </TimelineContent>
    </TimelineItem>
  );
}

export function ProgressStepSkeleton({ last = false }: { last?: boolean }) {
  return (
    <TimelineItem sx={{ minHeight: "auto" }}>
      <TimelineSeparator>
        <Skeleton variant="circular" width={24} height={24} animation="wave" />
        {!last && <TimelineConnector sx={{ width: 2, bgcolor: "action.hover" }} />}
      </TimelineSeparator>

      <TimelineContent sx={{ p: 0, pb: last ? 0 : 3 }} ml={1.5}>
        <Skeleton variant="text" width="40%" height={24} />
        <Skeleton variant="text" width="85%" height={20} />
      </TimelineContent>
    </TimelineItem>
  );
}
