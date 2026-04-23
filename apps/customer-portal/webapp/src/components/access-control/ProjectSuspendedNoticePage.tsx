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

import { type JSX } from "react";
import {
  Box,
  Card,
  CardContent,
  Divider,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { AlertCircle, Info } from "@wso2/oxygen-ui-icons-react";
import suspensionIllustration from "@assets/access-control/project-suspended.svg";
import type { ProjectDetails } from "@features/project-hub/types/projects";
import { parseBackendTimestamp } from "@utils/dateTime";

type ProjectSuspendedNoticePageProps = {
  project: ProjectDetails;
};

function formatDateLabel(raw: string | null | undefined): string {
  if (!raw) return "—";
  // date-only strings like "2023-10-03"
  const dateOnlyMatch = /^(\d{4})-(\d{2})-(\d{2})$/.exec(raw);
  if (dateOnlyMatch) {
    const [, yyyy, mm, dd] = dateOnlyMatch;
    const date = new Date(`${yyyy}-${mm}-${dd}T00:00:00`);
    if (!Number.isNaN(date.getTime())) {
      return new Intl.DateTimeFormat("en-US", {
        year: "numeric",
        month: "short",
        day: "numeric",
      }).format(date);
    }
  }
  const parsed = parseBackendTimestamp(raw);
  if (parsed) {
    return new Intl.DateTimeFormat("en-US", {
      year: "numeric",
      month: "short",
      day: "numeric",
    }).format(parsed);
  }
  return raw;
}

type InfoRowProps = {
  label: string;
  value: string;
};

function InfoRow({ label, value }: InfoRowProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
      <Typography variant="body2" color="text.secondary" sx={{ minWidth: 180 }}>
        {label}
      </Typography>
      <Typography variant="body2" fontWeight={500}>
        {value}
      </Typography>
    </Box>
  );
}

/**
 * Shown when a specific project has closureState === "Suspended".
 * Displays project info and suspension details.
 *
 * @param {ProjectSuspendedNoticePageProps} props - Suspended project payload.
 * @returns {JSX.Element} Suspension illustration and project information card.
 */
export default function ProjectSuspendedNoticePage({
  project,
}: ProjectSuspendedNoticePageProps): JSX.Element {
  const suspendedOnLabel = formatDateLabel(project.suspendedOn);
  const startDateLabel = formatDateLabel(project.startDate);
  const endDateLabel = formatDateLabel(project.endDate);
  const accountOwner = project.account?.ownerEmail ?? "—";
  const projectType = project.type?.label ?? "—";
  const reasons = project.suspensionReasons ?? [];

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: { xs: "column", md: "row" },
        alignItems: { xs: "stretch", md: "flex-start" },
        gap: { xs: 3, md: 4 },
        py: { xs: 4, md: 6 },
        px: 2,
        minHeight: "100%",
        width: "100%",
        maxWidth: 1100,
        mx: "auto",
      }}
    >
      <Card
        sx={{
          flex: 1,
          minWidth: 0,
          width: "100%",
          border: "1px solid",
          borderColor: "divider",
          borderRadius: 0,
          boxShadow: "none",
        }}
      >
        <CardContent sx={{ p: 0 }}>
          {/* Header */}
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1.5,
              px: 3,
              py: 2,
              borderBottom: "1px solid",
              borderColor: "divider",
            }}
          >
            <Info size={20} />
            <Typography variant="h6" fontWeight={600}>
              Project Suspension Notice
            </Typography>
          </Box>

          {/* Project details */}
          <Stack spacing={2} sx={{ px: 3, py: 3 }}>
            <InfoRow label="Project Name:" value={project.name} />
            <InfoRow label="Project Key:" value={project.key} />
            <InfoRow label="Project Type:" value={projectType} />
            <InfoRow label="Account Owner:" value={accountOwner} />
            <InfoRow label="Subscription Start Date:" value={startDateLabel} />
            <InfoRow label="Subscription End Date:" value={endDateLabel} />
          </Stack>

          <Divider />

          {/* Suspension reasons */}
          <Box sx={{ px: 3, py: 3 }}>
            <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.5, mb: 2 }}>
              <AlertCircle size={18} style={{ marginTop: 2, flexShrink: 0, color: "var(--oxygen-palette-warning-main, #f59e0b)" }} />
              <Typography variant="body2" color="text.secondary">
                This project was suspended for the following reasons,
              </Typography>
            </Box>

            {reasons.length > 0 ? (
              <Stack spacing={1} sx={{ pl: 4 }}>
                {reasons.map((reason, idx) => (
                  <Typography key={idx} variant="body2" fontWeight={500}>
                    • {reason}
                  </Typography>
                ))}
              </Stack>
            ) : (
              <Typography variant="body2" sx={{ pl: 4 }} color="text.secondary">
                No specific reasons provided.
              </Typography>
            )}
          </Box>

          {/* Suspension description */}
          {(project.suspendedOn ?? project.endDate) && (
            <>
              <Divider />
              <Box sx={{ px: 3, py: 3, bgcolor: "action.hover" }}>
                <Typography variant="body2" color="text.secondary" sx={{ lineHeight: 1.75 }}>
                  This project was suspended on{" "}
                  <Typography component="span" variant="body2" fontWeight={600} color="text.primary">
                    {suspendedOnLabel !== "—" ? suspendedOnLabel : endDateLabel}
                  </Typography>{" "}
                  due to non renewal of the contracts upon the end of previous subscription period.
                </Typography>
              </Box>
            </>
          )}
        </CardContent>
      </Card>

      <Box
        sx={{
          width: { xs: "100%", md: 300 },
          maxWidth: { xs: 360, md: 300 },
          flexShrink: 0,
          alignSelf: { xs: "center", md: "stretch" },
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <Box
          component="img"
          src={suspensionIllustration}
          alt=""
          aria-hidden
          sx={{
            width: "100%",
            maxWidth: 280,
            height: "auto",
            display: "block",
          }}
        />
      </Box>
    </Box>
  );
}
