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
import type { JSX } from "react";
import { useNavigate } from "react-router";
import SectionCard from "@features/csm-dashboard/components/SectionCard";
import {
  SEVERITY_COLOR,
  SLA_CLOCK_LABEL,
  formatTimeToBreach,
  stateLabel,
} from "@features/csm-dashboard/utils/abtDashboard";
import { casesHref } from "@features/csm-cases/utils/casesFiltersUrl";
import type { CsmSlaAtRiskCase } from "@features/csm-dashboard/types/abtDashboard";

interface SlaAtRiskSectionProps {
  cases?: CsmSlaAtRiskCase[];
  isLoading: boolean;
}

export default function SlaAtRiskSection({
  cases,
  isLoading,
}: SlaAtRiskSectionProps): JSX.Element {
  const navigate = useNavigate();
  const theme = useTheme();
  const go = (href: string) => navigate(href);
  const breachedCount = (cases ?? []).filter((c) => c.minutesToBreach < 0).length;

  return (
    <SectionCard
      title="SLA at risk"
      subtitle="Cases breaching or about to breach"
      action={
        !isLoading && (cases?.length ?? 0) > 0 ? (
          <Chip
            size="small"
            clickable
            color={breachedCount > 0 ? "error" : "warning"}
            label={
              breachedCount > 0
                ? `${breachedCount} breached`
                : `${cases?.length ?? 0} at risk`
            }
            onClick={() =>
              go(
                casesHref({
                  sla: breachedCount > 0 ? "breached" : "at_risk",
                }),
              )
            }
          />
        ) : undefined
      }
    >
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        {isLoading && (
          <>
            <Skeleton variant="rectangular" height={56} />
            <Skeleton variant="rectangular" height={56} />
            <Skeleton variant="rectangular" height={56} />
          </>
        )}
        {!isLoading && (cases?.length ?? 0) === 0 && (
          <Typography variant="body2" color="text.secondary">
            Nothing at risk right now.
          </Typography>
        )}
        {!isLoading &&
          cases?.map((c) => (
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
                borderColor: c.minutesToBreach < 0 ? "error.main" : "divider",
                borderRadius: 1,
                cursor: "pointer",
                transition: "border-color 0.15s ease, background-color 0.15s ease",
                "&:hover": {
                  bgcolor: "action.hover",
                  ...(c.minutesToBreach >= 0 && { borderColor: "primary.main" }),
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
                  {c.customer} · {stateLabel(c.state)} · Assignee: {c.assignee}
                </Typography>
              </Box>
              <Box sx={{ textAlign: "right" }}>
                <Typography
                  variant="body2"
                  color={c.minutesToBreach < 0 ? "error" : "warning.main"}
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
