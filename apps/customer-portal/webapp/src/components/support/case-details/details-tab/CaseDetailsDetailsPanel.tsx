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

import { Box, Chip, Stack, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import {
  Calendar,
  CircleCheck,
  Clock,
  Package,
  Tag,
  Building2,
  Info,
  Mail,
} from "@wso2/oxygen-ui-icons-react";
import { type ReactElement, type JSX } from "react";
import type { CaseDetails } from "@models/responses";
import { getSeverityLegendColor } from "@constants/dashboardConstants";
import AssignedEngineerDisplay from "@case-details-details/AssignedEngineerDisplay";
import CaseDetailsCard from "@case-details-details/CaseDetailsCard";
import ErrorStateIcon from "@components/common/error-state/ErrorStateIcon";
import {
  formatValue,
  formatSlaResponseTime,
  formatUtcToLocal,
  formatDateOnly,
  getAssignedEngineerLabel,
  getStatusColor,
  getStatusIconElement,
  mapSeverityToDisplay,
  resolveColorFromTheme,
  isSecurityReportAnalysisType,
} from "@utils/support";

export interface CaseDetailsDetailsPanelProps {
  data: CaseDetails | undefined;
  isError: boolean;
  isEngagement?: boolean;
}

/**
 * CaseDetailsDetailsPanel displays case overview, product & environment, and customer information
 * in three stacked cards using data from useGetCaseDetails API.
 *
 * @param {CaseDetailsDetailsPanelProps} props - Case details data and error state.
 * @returns {JSX.Element} The details panel or error state.
 */
export default function CaseDetailsDetailsPanel({
  data,
  isError,
  isEngagement = false,
}: CaseDetailsDetailsPanelProps): JSX.Element {
  const theme = useTheme();

  const isSecurityReportAnalysis = isSecurityReportAnalysisType(data?.type);
  const overviewTitle = isSecurityReportAnalysis
    ? "Security Report Analysis Overview"
    : isEngagement
    ? "Engagement Overview"
    : "Case Overview";
  const overviewIdLabel = isSecurityReportAnalysis
    ? "Security Report Analysis ID"
    : isEngagement
    ? "Engagement ID"
    : "Case ID";

  if (isError) {
    return (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          gap: 2,
          py: 10,
        }}
      >
        <ErrorStateIcon />
        <Typography variant="h4">Something Went Wrong</Typography>
      </Box>
    );
  }

  const statusLabel = data?.status?.label ?? null;
  const severityLabel = data?.severity?.label ?? null;
  const statusColorPath = getStatusColor(statusLabel ?? undefined);
  const resolvedStatusColor = resolveColorFromTheme(statusColorPath, theme);
  const statusChipIcon = getStatusIconElement(statusLabel, 12);

  const assignedEngineer = data?.assignedEngineer;

  const labelSx = {
    variant: "body2" as const,
    color: "text.secondary",
    sx: { mb: 0.5 },
  };

  const valueSx = { variant: "body2" as const, color: "text.primary" };

  const twoColumnGridSx = {
    display: "grid",
    gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
    gap: 3,
  };

  return (
    <Stack spacing={3}>
      {/* Section 1: Overview */}
      <CaseDetailsCard title={overviewTitle} icon={<Info size={20} aria-hidden />}>
        <Box sx={twoColumnGridSx}>
          <Box>
            <Typography {...labelSx}>{overviewIdLabel}</Typography>
            <Typography {...valueSx}>{formatValue(data?.number)}</Typography>
          </Box>
          <Box>
            <Typography {...labelSx}>Status</Typography>
            <Chip
              size="small"
              variant="outlined"
              label={formatValue(statusLabel)}
              icon={statusChipIcon as ReactElement}
              sx={{
                bgcolor: alpha(resolvedStatusColor, 0.1),
                color: resolvedStatusColor,
                height: 20,
                fontSize: "0.75rem",
                px: 0,
                "& .MuiChip-icon": {
                  color: "inherit",
                  ml: "6px",
                  mr: "6px",
                },
                "& .MuiChip-label": {
                  pl: 0,
                  pr: "6px",
                },
              }}
            />
          </Box>
          <Box>
            <Typography {...labelSx}>Severity</Typography>
            <Chip
              label={mapSeverityToDisplay(severityLabel ?? undefined)}
              size="small"
              variant="outlined"
              sx={{
                bgcolor: alpha(
                  getSeverityLegendColor(severityLabel ?? undefined),
                  0.1,
                ),
                color: getSeverityLegendColor(severityLabel ?? undefined),
                borderColor: alpha(
                  getSeverityLegendColor(severityLabel ?? undefined),
                  0.3,
                ),
                fontWeight: 500,
                px: 0,
                height: 20,
                fontSize: "0.75rem",
                "& .MuiChip-label": {
                  pl: "6px",
                  pr: "6px",
                },
              }}
            />
          </Box>
          <Box>
            <Typography {...labelSx}>Category</Typography>
            <Stack direction="row" alignItems="center" spacing={1}>
              <Tag size={16} color={theme.palette.text.secondary} aria-hidden />
              <Typography {...valueSx}>
                {formatValue(data?.issueType)}
              </Typography>
            </Stack>
          </Box>
          <Box>
            <Typography {...labelSx}>Created Date</Typography>
            <Stack direction="row" alignItems="center" spacing={1}>
              <Calendar
                size={16}
                color={theme.palette.text.secondary}
                aria-hidden
              />
              <Typography {...valueSx}>
                {formatDateOnly(data?.createdOn)}
              </Typography>
            </Stack>
          </Box>
          <Box>
            <Typography {...labelSx}>Last Updated</Typography>
            <Stack direction="row" alignItems="center" spacing={1}>
              <Clock
                size={16}
                color={theme.palette.text.secondary}
                aria-hidden
              />
              <Typography {...valueSx}>
                {formatDateOnly(data?.updatedOn)}
              </Typography>
            </Stack>
          </Box>
          {!isEngagement && (
            <Box>
              <Typography {...labelSx}>SLA Response Time</Typography>
              <Typography {...valueSx}>
                {formatSlaResponseTime(data?.slaResponseTime)}
              </Typography>
            </Box>
          )}
          {isSecurityReportAnalysis ? (
            <>
              <Box>
                <Typography {...labelSx}>Assigned Engineer</Typography>
                {getAssignedEngineerLabel(assignedEngineer) ? (
                  <AssignedEngineerDisplay
                    assignedEngineer={assignedEngineer}
                  />
                ) : (
                  <Typography {...valueSx}>Not available</Typography>
                )}
              </Box>
              <Box>
                <Typography {...labelSx}>Engineer Email</Typography>
                {data?.engineerEmail ? (
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <Mail
                      size={16}
                      color={theme.palette.text.secondary}
                      aria-hidden
                    />
                    <Typography {...valueSx}>{data.engineerEmail}</Typography>
                  </Stack>
                ) : (
                  <Typography {...valueSx}>Not available</Typography>
                )}
              </Box>
            </>
          ) : (
            getAssignedEngineerLabel(assignedEngineer) && (
              <Box>
                <Typography {...labelSx}>Assigned Engineer</Typography>
                <AssignedEngineerDisplay assignedEngineer={assignedEngineer} />
              </Box>
            )
          )}
        </Box>
      </CaseDetailsCard>

      {/* Section 2: Product & Environment */}
      {!isEngagement && (
        <CaseDetailsCard
          title="Product & Environment"
          icon={<Package size={20} aria-hidden />}
        >
          <Box sx={twoColumnGridSx}>
            {isSecurityReportAnalysis && (
              <Box>
                <Typography {...labelSx}>Report Type</Typography>
                {typeof data?.type === "object" && data?.type?.label ? (
                  <Typography {...valueSx}>{data.type.label}</Typography>
                ) : typeof data?.type === "string" ? (
                  <Typography {...valueSx}>{data.type}</Typography>
                ) : (
                  <Typography {...valueSx}>Not available</Typography>
                )}
              </Box>
            )}
            <Box>
              <Typography {...labelSx}>Product Name</Typography>
              <Typography {...valueSx}>
                {formatValue(
                  data?.deployedProduct?.label?.trim?.()?.length
                    ? data.deployedProduct.label
                    : null,
                )}
              </Typography>
            </Box>
            <Box>
              <Typography {...labelSx}>Product Version</Typography>
              <Typography {...valueSx}>
                {formatValue(data?.deployedProduct?.version ?? null)}
              </Typography>
            </Box>
            <Box>
              <Typography {...labelSx}>Environment Type</Typography>
              <Typography {...valueSx}>
                {formatValue(data?.deployment?.label)}
              </Typography>
            </Box>
          </Box>
        </CaseDetailsCard>
      )}

      {/* Section 3: Closed Case Details (only when case is closed) */}
      {statusLabel?.toLowerCase() === "closed" && (
        <CaseDetailsCard
          title="Closed Case Details"
          icon={<CircleCheck size={20} aria-hidden />}
        >
          <Box sx={twoColumnGridSx}>
            <Box>
              <Typography {...labelSx}>Closed On</Typography>
              <Stack direction="row" alignItems="center" spacing={1}>
                <Calendar
                  size={16}
                  color={theme.palette.text.secondary}
                  aria-hidden
                />
                <Typography {...valueSx}>
                  {formatUtcToLocal(data?.closedOn)}
                </Typography>
              </Stack>
            </Box>
            <Box>
              <Typography {...labelSx}>Closed By</Typography>
              <Typography {...valueSx}>
                {formatValue(
                  data?.closedBy?.label ?? data?.closedBy?.name ?? null,
                )}
              </Typography>
            </Box>
            <Box sx={{ gridColumn: { xs: "1", md: "1 / -1" } }}>
              <Typography {...labelSx}>Close Notes</Typography>
              <Typography {...valueSx}>
                {formatValue(data?.closeNotes)}
              </Typography>
            </Box>
          </Box>
        </CaseDetailsCard>
      )}

      {/* Section 4: Customer Information */}
      <CaseDetailsCard
        title="Customer Information"
        icon={<Building2 size={20} aria-hidden />}
      >
        <Box sx={twoColumnGridSx}>
          <Box>
            <Typography {...labelSx}>Organization</Typography>
            <Typography {...valueSx}>
              {formatValue(data?.account?.label)}
            </Typography>
          </Box>
          <Box>
            <Typography {...labelSx}>Account Type</Typography>
            <Chip
              label={formatValue(data?.account?.type)}
              size="small"
              variant="outlined"
              sx={{
                borderColor: "transparent",
                bgcolor: "action.hover",
                height: 20,
                fontSize: "0.75rem",
              }}
            />
          </Box>
          {!isSecurityReportAnalysis && (
            <>
              <Box>
                <Typography {...labelSx}>Project</Typography>
                <Typography {...valueSx}>
                  {formatValue(data?.project?.label)}
                </Typography>
              </Box>
              {getAssignedEngineerLabel(assignedEngineer) && (
                <Box>
                  <Typography {...labelSx}>Assigned Engineer</Typography>
                  <AssignedEngineerDisplay
                    assignedEngineer={assignedEngineer}
                  />
                </Box>
              )}
              {!isEngagement && (
                <Box>
                  <Typography {...labelSx}>CS Manager</Typography>
                  <Typography {...valueSx}>
                    {formatValue(data?.csManager)}
                  </Typography>
                </Box>
              )}
            </>
          )}
        </Box>
      </CaseDetailsCard>
    </Stack>
  );
}
