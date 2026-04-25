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

import type { CaseDetailsDetailsPanelProps } from "@features/support/types/supportComponents";
import {
  Box,
  Button,
  Chip,
  Stack,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import {
  Calendar,
  CircleCheck,
  Clock,
  Package,
  Tag,
  Building2,
  Info,
  Mail,
  FileText,
  ExternalLink,
} from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import { Link, useLocation, useParams } from "react-router";
import DOMPurify from "dompurify";
import { getSeverityLegendColor } from "@features/dashboard/utils/dashboard";
import AssignedEngineerDisplay from "@case-details-details/AssignedEngineerDisplay";
import CaseDetailsCard from "@case-details-details/CaseDetailsCard";
import ApiErrorState from "@components/error/ApiErrorState";
import {
  formatValue,
  formatUtcToLocal,
  formatDateOnly,
  getAssignedEngineerLabel,
  getStatusColor,
  mapSeverityToDisplay,
  resolveColorFromTheme,
  isSecurityReportAnalysisType,
  stripHtml,
} from "@features/support/utils/support";

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
  error,
  isEngagement = false,
  isServiceRequest = false,
}: CaseDetailsDetailsPanelProps): JSX.Element {
  const theme = useTheme();
  const { projectId = "" } = useParams<{ projectId: string }>();
  const location = useLocation();
  const basePath = location.pathname.includes("/operations/")
    ? "operations"
    : "support";

  const isSecurityReportAnalysis = isSecurityReportAnalysisType(data?.type);
  const overviewTitle = isSecurityReportAnalysis
    ? "Security Report Analysis Overview"
    : isEngagement
      ? "Engagement Overview"
      : isServiceRequest
        ? "Service Request Overview"
        : "Case Overview";
  const overviewIdLabel = isSecurityReportAnalysis
    ? "Security Report Analysis ID"
    : isEngagement
      ? "Engagement ID"
      : isServiceRequest
        ? "Request number"
        : "Case ID";

  const productDisplayName = data?.deployedProduct?.label?.trim?.()?.length
    ? data.deployedProduct.label
    : (data?.product?.label ?? null);
  const relatedChangeRequest = data?.changeRequests?.[0];

  if (isError) {
    return (
      <ApiErrorState
        error={error}
        fallbackMessage="Something went wrong while loading case details."
        containerSx={{ py: 10 }}
      />
    );
  }

  const statusLabel = data?.status?.label ?? null;
  const severityLabel = data?.severity?.label ?? null;
  const statusColorPath = getStatusColor(statusLabel ?? undefined);
  const resolvedStatusColor = resolveColorFromTheme(statusColorPath, theme);

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
      <CaseDetailsCard
        title={overviewTitle}
        icon={<Info size={20} aria-hidden />}
        rightAction={
          isServiceRequest ? (
            <Button
              component={Link}
              to={
                relatedChangeRequest
                  ? `/projects/${projectId}/${basePath}/change-requests/${relatedChangeRequest.id}`
                  : ""
              }
              state={{
                returnTo: location.pathname + location.search,
              }}
              variant="text"
              size="small"
              startIcon={<ExternalLink size={14} />}
              disabled={!relatedChangeRequest}
              sx={{ minHeight: "unset", p: 0, textTransform: "none" }}
            >
              View related change request
            </Button>
          ) : null
        }
      >
        <Box sx={twoColumnGridSx}>
          <Box>
            <Typography {...labelSx}>{overviewIdLabel}</Typography>
            <Typography {...valueSx}>{formatValue(data?.number)}</Typography>
          </Box>
          {data?.internalId ? (
            <Box>
              <Typography {...labelSx}>WSO2 Case ID</Typography>
              <Typography {...valueSx}>
                {formatValue(data.internalId)}
              </Typography>
            </Box>
          ) : null}
          <Box>
            <Typography {...labelSx}>Status</Typography>
            <Stack direction="row" spacing={0.75} alignItems="center">
              <Box
                sx={{
                  width: 8,
                  height: 8,
                  borderRadius: "50%",
                  bgcolor: resolvedStatusColor,
                  flexShrink: 0,
                }}
              />
              <Typography variant="caption" color="text.secondary">
                {formatValue(statusLabel)}
              </Typography>
            </Stack>
          </Box>
          {!isEngagement && !isSecurityReportAnalysis && !isServiceRequest && (
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
          )}
          {(!isServiceRequest || data?.issueType) && (
            <Box>
              <Typography {...labelSx}>Category</Typography>
              <Stack direction="row" alignItems="center" spacing={1}>
                <Tag
                  size={16}
                  color={theme.palette.text.secondary}
                  aria-hidden
                />
                <Typography {...valueSx}>
                  {formatValue(data?.issueType)}
                </Typography>
              </Stack>
            </Box>
          )}
          {isServiceRequest && data?.createdBy ? (
            <Box>
              <Typography {...labelSx}>Created by</Typography>
              <Typography {...valueSx}>
                {formatValue(data.createdBy)}
              </Typography>
            </Box>
          ) : null}
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
          {isServiceRequest && data?.duration ? (
            <Box>
              <Typography {...labelSx}>Duration</Typography>
              <Typography {...valueSx}>{formatValue(data.duration)}</Typography>
            </Box>
          ) : null}
          {/* Related change request action is rendered in the card header (top-right) */}
          {isServiceRequest &&
          (data?.engagementStartDate || data?.engagementEndDate) ? (
            <>
              <Box>
                <Typography {...labelSx}>Engagement start</Typography>
                <Typography {...valueSx}>
                  {formatDateOnly(data?.engagementStartDate ?? null)}
                </Typography>
              </Box>
              <Box>
                <Typography {...labelSx}>Engagement end</Typography>
                <Typography {...valueSx}>
                  {formatDateOnly(data?.engagementEndDate ?? null)}
                </Typography>
              </Box>
            </>
          ) : null}
          {isServiceRequest && data?.assignedTeam ? (
            <Box>
              <Typography {...labelSx}>Assigned team</Typography>
              <Typography {...valueSx}>
                {formatValue(data.assignedTeam)}
              </Typography>
            </Box>
          ) : null}
          {isServiceRequest && (data?.catalog || data?.catalogItem) ? (
            <>
              <Box>
                <Typography {...labelSx}>Catalog</Typography>
                <Typography {...valueSx}>
                  {formatValue(data?.catalog ?? null)}
                </Typography>
              </Box>
              <Box>
                <Typography {...labelSx}>Catalog item</Typography>
                <Typography {...valueSx}>
                  {formatValue(data?.catalogItem ?? null)}
                </Typography>
              </Box>
            </>
          ) : null}
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

      {isServiceRequest ? (
        <CaseDetailsCard
          title="Description"
          icon={<FileText size={20} aria-hidden />}
        >
          {data?.description ? (
            <Box
              component="div"
              sx={{
                typography: "body2",
                color: "text.primary",
                "& p": { mb: 0.5 },
                "& p:last-child": { mb: 0 },
                "& code": {
                  display: "block",
                  p: 1,
                  bgcolor: "action.hover",
                  fontSize: "0.875rem",
                  whiteSpace: "pre-wrap",
                  overflowWrap: "break-word",
                },
              }}
              // biome-ignore lint/security/noDangerouslySetInnerHtml: sanitized with DOMPurify
              dangerouslySetInnerHTML={{
                __html: DOMPurify.sanitize(data.description),
              }}
            />
          ) : (
            <Typography {...valueSx}>Nothing</Typography>
          )}
        </CaseDetailsCard>
      ) : (
        data?.variables &&
        data.variables.length > 0 && (
          <CaseDetailsCard
            title="Request details"
            icon={<FileText size={20} aria-hidden />}
          >
            <Stack spacing={2}>
              {data.variables.map((v, i) => (
                <Box key={`${v.name}-${i}`}>
                  <Typography {...labelSx}>{formatValue(v.name)}</Typography>
                  <Typography {...valueSx} sx={{ whiteSpace: "pre-wrap" }}>
                    {stripHtml(v.value ?? "") || formatValue(v.value)}
                  </Typography>
                </Box>
              ))}
            </Stack>
          </CaseDetailsCard>
        )
      )}

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
                {formatValue(productDisplayName)}
              </Typography>
            </Box>
            {data?.deployedProduct?.version ? (
              <Box>
                <Typography {...labelSx}>Production Version</Typography>
                <Typography {...valueSx}>
                  {data.deployedProduct.version}
                </Typography>
              </Box>
            ) : null}
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
          title={
            isServiceRequest ? "Closed Request Details" : "Closed Case Details"
          }
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
            </>
          )}
        </Box>
      </CaseDetailsCard>
    </Stack>
  );
}
