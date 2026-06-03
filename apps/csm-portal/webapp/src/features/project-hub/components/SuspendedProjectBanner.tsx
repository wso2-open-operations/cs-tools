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

import { type JSX, useState } from "react";
import { Box, Button, Stack, Typography } from "@wso2/oxygen-ui";
import { AlertCircle, ChevronDown, ChevronUp } from "@wso2/oxygen-ui-icons-react";
import type { ProjectDetails } from "@features/project-hub/types/projects";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";

interface SuspendedProjectBannerProps {
  project: ProjectDetails;
}

function formatDateLabel(raw: string | null | undefined): string | null {
  if (!raw) return null;
  return (
    formatBackendTimestampForDisplay(raw, {
      year: "numeric",
      month: "short",
      day: "numeric",
    }) ?? raw
  );
}

/**
 * Non-blocking notice rendered above the project outlet when the project is
 * suspended. Internal teams (CSM portal users) are still allowed into the
 * project; the banner just makes the suspension obvious and surfaces the
 * reasons + suspended-on date.
 */
export default function SuspendedProjectBanner({
  project,
}: SuspendedProjectBannerProps): JSX.Element {
  const [expanded, setExpanded] = useState(false);
  const suspendedOn = formatDateLabel(project.suspendedOn) ?? formatDateLabel(project.endDate);
  const reasons = project.suspensionReasons ?? [];
  const hasDetails = reasons.length > 0 || suspendedOn;

  return (
    <Box
      role="status"
      sx={{
        mx: 2,
        mt: 2,
        px: 2,
        py: 1.25,
        borderRadius: 1,
        border: "1px solid",
        borderColor: "warning.main",
        backgroundColor: "warning.50",
        color: "warning.dark",
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.25 }}>
        <AlertCircle size={18} />
        <Typography variant="body2" sx={{ flex: 1 }}>
          <strong>This project is suspended.</strong> Customer access is
          restricted; internal users can continue working on the case history,
          comments, and time entries.
        </Typography>
        {hasDetails && (
          <Button
            size="small"
            variant="text"
            color="inherit"
            onClick={() => setExpanded((v) => !v)}
            endIcon={expanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
          >
            {expanded ? "Hide details" : "Details"}
          </Button>
        )}
      </Box>

      {expanded && hasDetails && (
        <Stack spacing={0.75} sx={{ mt: 1.25, pl: 3.5 }}>
          {suspendedOn && (
            <Typography variant="caption">
              Suspended on{" "}
              <strong>{suspendedOn}</strong>
              {project.endDate && project.endDate !== project.suspendedOn && (
                <> &middot; subscription ended {formatDateLabel(project.endDate)}</>
              )}
            </Typography>
          )}
          {reasons.length > 0 && (
            <Box>
              <Typography variant="caption" sx={{ fontWeight: 600 }}>
                Reasons:
              </Typography>
              <Box component="ul" sx={{ m: 0, pl: 2.5 }}>
                {reasons.map((r, i) => (
                  <Typography key={i} component="li" variant="caption">
                    {r}
                  </Typography>
                ))}
              </Box>
            </Box>
          )}
        </Stack>
      )}
    </Box>
  );
}
