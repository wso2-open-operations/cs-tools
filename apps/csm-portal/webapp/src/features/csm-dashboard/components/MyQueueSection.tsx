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

import { Box, Chip, Skeleton, Typography, useTheme } from "@wso2/oxygen-ui";
import type { JSX, KeyboardEvent } from "react";
import { useNavigate } from "react-router";
import SectionCard from "@features/csm-dashboard/components/SectionCard";
import {
  SEVERITY_COLOR,
  SLA_CLOCK_LABEL,
  formatTimeToBreach,
  stateLabel,
} from "@features/csm-dashboard/utils/abtDashboard";
import { casesHref } from "@features/csm-cases/utils/casesFiltersUrl";
import { ASSIGNEE_ME_TOKEN } from "@features/csm-cases/components/CasesFilterBar";
import type { CsmQueueSummary } from "@features/csm-dashboard/types/abtDashboard";

interface MyQueueSectionProps {
  queue?: CsmQueueSummary;
  isLoading: boolean;
}

function StatPill({
  label,
  value,
  href,
  onNavigate,
}: {
  label: string;
  value: number | string;
  href?: string;
  onNavigate?: (href: string) => void;
}): JSX.Element {
  const interactive = !!href && !!onNavigate;
  const handleKey = (e: KeyboardEvent<HTMLDivElement>) => {
    if (!interactive) return;
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      onNavigate!(href!);
    }
  };
  return (
    <Box
      role={interactive ? "link" : undefined}
      tabIndex={interactive ? 0 : -1}
      aria-label={interactive ? `${label}: ${value}` : undefined}
      onClick={interactive ? () => onNavigate!(href!) : undefined}
      onKeyDown={handleKey}
      sx={{
        flex: 1,
        minWidth: 120,
        p: 1.5,
        borderRadius: 1,
        border: 1,
        borderColor: "divider",
        display: "flex",
        flexDirection: "column",
        gap: 0.5,
        ...(interactive && {
          cursor: "pointer",
          transition: "border-color 0.15s ease, background-color 0.15s ease",
          "&:hover": {
            borderColor: "primary.main",
            bgcolor: "action.hover",
          },
        }),
      }}
    >
      <Typography variant="h5">{value}</Typography>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
    </Box>
  );
}

export default function MyQueueSection({
  queue,
  isLoading,
}: MyQueueSectionProps): JSX.Element {
  const navigate = useNavigate();
  const theme = useTheme();
  const go = (href: string) => navigate(href);

  return (
    <SectionCard
      title="My queue"
      subtitle="Cases assigned to you"
    >
      <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
        <StatPill
          label="Action required"
          value={isLoading ? "—" : (queue?.actionRequiredCount ?? 0)}
          href={casesHref({ assignees: [ASSIGNEE_ME_TOKEN], states: ["open", "reopened"] })}
          onNavigate={go}
        />
        <StatPill
          label="In progress"
          value={isLoading ? "—" : (queue?.inProgressCount ?? 0)}
          href={casesHref({ assignees: [ASSIGNEE_ME_TOKEN], states: ["work_in_progress"] })}
          onNavigate={go}
        />
        <StatPill
          label="Awaiting info"
          value={isLoading ? "—" : (queue?.awaitingInfoCount ?? 0)}
          href={casesHref({ assignees: [ASSIGNEE_ME_TOKEN], states: ["awaiting_info"] })}
          onNavigate={go}
        />
        <StatPill
          label="Total open"
          value={isLoading ? "—" : (queue?.totalOpenCount ?? 0)}
          href={casesHref({ assignees: [ASSIGNEE_ME_TOKEN] })}
          onNavigate={go}
        />
      </Box>

      <Box sx={{ mt: 1, display: "flex", flexDirection: "column", gap: 1 }}>
        <Typography variant="subtitle2">Most urgent</Typography>
        {isLoading && (
          <>
            <Skeleton variant="rectangular" height={48} />
            <Skeleton variant="rectangular" height={48} />
            <Skeleton variant="rectangular" height={48} />
          </>
        )}
        {!isLoading && (queue?.topCases ?? []).length === 0 && (
          <Typography variant="body2" color="text.secondary">
            Nothing in your queue.
          </Typography>
        )}
        {!isLoading &&
          queue?.topCases.map((c) => (
            <Box
              key={c.id}
              role="link"
              tabIndex={0}
              aria-label={`${c.caseNumber} ${c.subject}`}
              onClick={() => go(casesHref({ search: c.caseNumber }))}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  go(casesHref({ search: c.caseNumber }));
                }
              }}
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1.5,
                p: 1.25,
                border: 1,
                borderColor: "divider",
                borderRadius: 1,
                cursor: "pointer",
                transition: "border-color 0.15s ease, background-color 0.15s ease",
                "&:hover": {
                  borderColor: "primary.main",
                  bgcolor: "action.hover",
                },
                "&:focus-visible": {
                  outline: `2px solid ${theme.palette.primary.main}`,
                  outlineOffset: 2,
                },
              }}
            >
              <Chip
                size="small"
                label={c.severity}
                color={SEVERITY_COLOR[c.severity]}
              />
              <Box sx={{ flex: 1, minWidth: 0 }}>
                <Typography variant="body2" noWrap>
                  <strong>{c.caseNumber}</strong> · {c.subject}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {c.customer} · {stateLabel(c.state)}
                </Typography>
              </Box>
              <Box sx={{ textAlign: "right" }}>
                <Typography
                  variant="body2"
                  color={c.minutesToBreach < 0 ? "error" : "text.primary"}
                >
                  {formatTimeToBreach(c.minutesToBreach)}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {SLA_CLOCK_LABEL[c.slaClockType]}
                </Typography>
              </Box>
            </Box>
          ))}
      </Box>
    </SectionCard>
  );
}
