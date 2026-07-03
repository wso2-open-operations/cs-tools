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

import { Fragment, type JSX } from "react";
import { Box, Tooltip, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import { Info, TriangleAlert } from "@wso2/oxygen-ui-icons-react";
import { usePostCaseEscalationsSearch } from "@features/support/api/usePostCaseEscalationsSearch";
import type { EscalationRecord } from "@features/support/types/cases";
import { ESCALATION_NEXT_LEVEL } from "@features/support/constants/supportConstants";
import CaseDetailsCard from "./CaseDetailsCard";

const TOTAL_LEVELS = 5;
const LEAD_WARNING_THRESHOLD = 3;
const NON_LEAD_AVAILABLE_MAX = 3;

const LEVEL_ROLE: Record<number, string> = Object.fromEntries(
  Array.from({ length: TOTAL_LEVELS }, (_, i) => [
    i + 1,
    ESCALATION_NEXT_LEVEL[String(i) as keyof typeof ESCALATION_NEXT_LEVEL]?.notifiedLabel ?? `EL${i + 1}`,
  ]),
);

type StepStatus = "previous" | "active" | "available" | "locked";

function getStepStatus(levelNum: number, currentLevelNum: number, isLead: boolean): StepStatus {
  if (levelNum < currentLevelNum) return "previous";
  if (levelNum === currentLevelNum) return "active";
  if (isLead || levelNum <= NON_LEAD_AVAILABLE_MAX) return "available";
  return "locked";
}

function TooltipContent({ levelNum, record }: { levelNum: number; record: EscalationRecord | undefined }): JSX.Element {
  return (
    <Box sx={{ p: 0.5, maxWidth: 240 }}>
      <Typography variant="caption" sx={{ fontWeight: 700, display: "block", mb: 0.5 }}>
        {LEVEL_ROLE[levelNum]} Escalation
      </Typography>
      {record?.reason && (
        <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 0.5 }}>
          {record.reason}
        </Typography>
      )}
      {record?.notificationSentTo && record.notificationSentTo.length > 0 && (
        <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 0.5 }}>
          Notified: {record.notificationSentTo.map((u) => u.name ?? u.userName).join(", ")}
        </Typography>
      )}
      <Typography variant="caption" sx={{ fontWeight: 500, display: "block" }}>
        → {LEVEL_ROLE[levelNum]}
      </Typography>
    </Box>
  );
}

type Props = {
  caseId: string;
  currentLevelId?: number | string | null;
  isLead?: boolean;
};

export default function CaseEscalationLevelsCard({ caseId, currentLevelId, isLead }: Props): JSX.Element {
  const theme = useTheme();
  const isDark = theme.palette.mode === "dark";
  const { data: escalationData } = usePostCaseEscalationsSearch(caseId);

  const currentLevelNum = Math.max(0, Number(currentLevelId ?? 0));

  const recordByLevel = new Map<number, EscalationRecord>();
  for (const record of escalationData?.escalations ?? []) {
    const id = Number(record.currentLevel?.id ?? 0);
    if (id > 0 && !recordByLevel.has(id)) {
      recordByLevel.set(id, record);
    }
  }

  const showLeadWarning = currentLevelNum >= LEAD_WARNING_THRESHOLD;

  const neutralGrayLegend = isDark ? theme.palette.grey[400] : theme.palette.grey[500];
  const LEGEND = [
    { label: "Previous", color: neutralGrayLegend, outlined: false },
    { label: "Active", color: theme.palette.warning.main, outlined: false },
    { label: "Available", color: theme.palette.primary.main, outlined: true },
    { label: "Locked", color: neutralGrayLegend, outlined: true, dim: true },
  ] as const;

  return (
    <CaseDetailsCard title="Escalation Levels" icon={<TriangleAlert size={20} aria-hidden />}>
      {/* Stepper */}
      <Box sx={{ display: "flex", alignItems: "flex-start", width: "100%", px: 2, pt: 2, pb: 1 }}>
        {Array.from({ length: TOTAL_LEVELS }, (_, i) => i + 1).map((levelNum, idx) => {
          const status: StepStatus = getStepStatus(levelNum, currentLevelNum, isLead ?? false);
          const record = recordByLevel.get(levelNum);
          const canShowTooltip = status === "previous" || status === "active";

          const neutralGray = isDark ? theme.palette.grey[400] : theme.palette.grey[500];

          let circleSx: object;
          if (status === "active") {
            circleSx = {
              bgcolor: theme.palette.warning.main,
              color: "#fff",
              border: `2px solid ${theme.palette.warning.main}`,
              boxShadow: `0 0 0 5px ${alpha(theme.palette.warning.main, 0.22)}`,
            };
          } else if (status === "previous") {
            circleSx = {
              bgcolor: neutralGray,
              color: isDark ? theme.palette.grey[900] : "#fff",
              border: `2px solid ${neutralGray}`,
            };
          } else if (status === "available") {
            circleSx = {
              bgcolor: "transparent",
              color: theme.palette.primary.main,
              border: `2px solid ${theme.palette.primary.main}`,
            };
          } else {
            circleSx = {
              bgcolor: "transparent",
              color: neutralGray,
              border: `2px solid ${neutralGray}`,
            };
          }

          const labelColor =
            status === "active"
              ? "warning.main"
              : status === "available"
                ? "primary.main"
                : neutralGray;

          const stepNode = (
            <Box
              sx={{
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                gap: 0.75,
                flexShrink: 0,
                opacity: status === "locked" ? (isDark ? 0.85 : 0.65) : 1,
              }}
            >
              <Box
                sx={{
                  width: 40,
                  height: 40,
                  borderRadius: "50%",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontWeight: 700,
                  fontSize: "0.9rem",
                  ...circleSx,
                }}
              >
                {levelNum}
              </Box>
              <Typography variant="caption" sx={{ fontWeight: 500, color: labelColor }}>
                EL{levelNum}
              </Typography>
            </Box>
          );

          return (
            <Fragment key={levelNum}>
              {idx > 0 && (
                <Box
                  sx={{
                    flex: 1,
                    height: 2,
                    bgcolor: neutralGray,
                    opacity: levelNum <= currentLevelNum ? 1 : (isDark ? 0.5 : 0.35),
                    alignSelf: "flex-start",
                    mt: "19px",
                    mx: 0.5,
                  }}
                />
              )}
              {canShowTooltip ? (
                <Tooltip
                  title={<TooltipContent levelNum={levelNum} record={record} />}
                  placement="bottom"
                  arrow
                >
                  <Box>{stepNode}</Box>
                </Tooltip>
              ) : (
                stepNode
              )}
            </Fragment>
          );
        })}
      </Box>

      {/* Legend */}
      <Box sx={{ display: "flex", justifyContent: "center", gap: 3, mb: 2 }}>
        {LEGEND.map(({ label, color, outlined, ...rest }) => {
          const dim = "dim" in rest && rest.dim;
          return (
          <Box key={label} sx={{ display: "flex", alignItems: "center", gap: 0.5, opacity: dim ? 0.55 : 1 }}>
            <Box
              sx={{
                width: 10,
                height: 10,
                borderRadius: "50%",
                bgcolor: outlined ? "transparent" : color,
                border: `2px solid ${color}`,
                flexShrink: 0,
              }}
            />
            <Typography variant="caption" color="text.secondary">
              {label}
            </Typography>
          </Box>
          );
        })}
      </Box>

      {/* Lead-required warning */}
      {showLeadWarning && (
        <Box
          sx={{
            display: "flex",
            alignItems: "flex-start",
            gap: 1,
            p: 1.5,
            borderRadius: 1,
            border: "1px solid",
            borderColor: alpha(theme.palette.warning.main, 0.3),
            bgcolor: alpha(theme.palette.warning.main, 0.06),
          }}
        >
          <Info size={16} color={theme.palette.warning.main} style={{ flexShrink: 0, marginTop: 2 }} />
          <Box>
            <Typography variant="caption" sx={{ fontWeight: 600, color: "text.primary", display: "block" }}>
              Need to escalate beyond EL3?
            </Typography>
            <Typography variant="caption" sx={{ color: "warning.dark" }}>
              Contact your lead to escalate to higher levels (EL4-EL5)
            </Typography>
          </Box>
        </Box>
      )}
    </CaseDetailsCard>
  );
}
