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

import { useParams, useNavigate, useLocation } from "react-router";
import { type JSX, useMemo, useState } from "react";
import DOMPurify from "dompurify";
import {
  Box,
  Button,
  Stack,
  Typography,
  Paper,
  Chip,
  Divider,
  alpha,
  colors,
} from "@wso2/oxygen-ui";
import {
  ArrowLeft,
  TriangleAlert,
  CircleCheckBig,
  Circle,
  Server,
  Package,
  FileText,
  Users,
  RotateCcw,
  Shield,
  Download,
  ExternalLink,
  CalendarClock,
  FileCheck,
  X,
} from "@wso2/oxygen-ui-icons-react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import Error500Page from "@components/error/Error500Page";
import useGetChangeRequestDetails from "@features/operations/api/useGetChangeRequestDetails";
import { usePatchChangeRequest } from "@features/operations/api/usePatchChangeRequest";
import ScheduledMaintenanceWindowCard from "@features/operations/components/change-requests/ScheduledMaintenanceWindowCard";
import ProposeNewImplementationTimeModal from "@features/operations/components/change-requests/ProposeNewImplementationTimeModal";
import ChangeRequestDetailsLoadingSkeleton from "@features/operations/components/change-requests/ChangeRequestDetailsLoadingSkeleton";
import {
  buildChangeRequestWorkflowStages,
  generateChangeRequestDetailsPdf,
  getChangeRequestDecisionMode,
} from "@features/operations/utils/changeRequests";
import { ChangeRequestDecisionMode } from "@features/operations/types/changeRequests";
import { formatDateTime } from "@features/support/utils/support";
import {
  formatImpactLabel,
  getChangeRequestImpactColorShades,
  getChangeRequestStateColorShades,
  getChangeRequestStateIcon,
} from "@features/operations/utils/changeRequestUi";

/**
 * ChangeRequestDetailsPage component to display detailed information about a change request.
 *
 * @returns {JSX.Element} The rendered Change Request Details page.
 */
export default function ChangeRequestDetailsPage(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const { projectId, changeRequestId } = useParams<{
    projectId: string;
    changeRequestId: string;
  }>();
  const basePath = location.pathname.includes("/operations/")
    ? "operations"
    : "support";
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;

  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();
  const [proposeTimeOpen, setProposeTimeOpen] = useState(false);

  const {
    data: changeRequest,
    isLoading,
    error,
    isFetching,
  } = useGetChangeRequestDetails(changeRequestId || "");
  const patchChangeRequest = usePatchChangeRequest(changeRequestId || "");

  const { workflowStages, currentStateIndex } = useMemo(
    () => buildChangeRequestWorkflowStages(changeRequest),
    [changeRequest],
  );
  const decisionMode = getChangeRequestDecisionMode(changeRequest);
  const canShowApprovalActions =
    decisionMode !== ChangeRequestDecisionMode.NONE;
  const canShowProposeNewTime =
    decisionMode === ChangeRequestDecisionMode.CUSTOMER_APPROVAL;

  const impactColor = getChangeRequestImpactColorShades(
    changeRequest?.impact?.label,
  );
  const statusColor = getChangeRequestStateColorShades(changeRequest?.state);

  const handleApproveChange = () => {
    if (!changeRequest || decisionMode === ChangeRequestDecisionMode.NONE)
      return;
    patchChangeRequest.mutate(
      decisionMode === ChangeRequestDecisionMode.CUSTOMER_APPROVAL
        ? { isCustomerApproved: true }
        : { isCustomerReviewed: true },
      {
        onSuccess: () => {
          showSuccess("Change request approved successfully.");
        },
        onError: (err) => {
          showError(err?.message ?? "Failed to approve change request.");
        },
      },
    );
  };

  const handleRejectChange = () => {
    if (!changeRequest || decisionMode === ChangeRequestDecisionMode.NONE)
      return;
    patchChangeRequest.mutate(
      decisionMode === ChangeRequestDecisionMode.CUSTOMER_APPROVAL
        ? { isCustomerApproved: false }
        : { isCustomerReviewed: false },
      {
        onSuccess: () => {
          showSuccess("Change request rejected successfully.");
        },
        onError: (err) => {
          showError(err?.message ?? "Failed to reject change request.");
        },
      },
    );
  };

  // Loading state with skeleton (or if fetching/no data yet)
  if (isLoading || (isFetching && !changeRequest)) {
    return <ChangeRequestDetailsLoadingSkeleton />;
  }

  // Error state - only show error if we have an actual error and not loading
  if (error && !isLoading && !isFetching) {
    return (
      <Stack spacing={3}>
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={() =>
            returnTo
              ? navigate(returnTo)
              : navigate(`/projects/${projectId}/${basePath}/change-requests`)
          }
          sx={{ alignSelf: "flex-start" }}
          variant="text"
        >
          Back to Change Requests
        </Button>
        <Paper variant="outlined" sx={{ p: 6, textAlign: "center" }}>
          <Error500Page style={{ width: 200, height: "auto" }} />
          <Typography variant="h6" color="text.primary" sx={{ mt: 3, mb: 1 }}>
            Error Loading Change Request
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {error?.message || "Could not load change request details"}
          </Typography>
        </Paper>
      </Stack>
    );
  }

  if (!changeRequest) {
    return <ChangeRequestDetailsLoadingSkeleton />;
  }

  // Render state icon from stable id/label resolution
  const renderStateIcon = () => {
    const IconComponent = getChangeRequestStateIcon(changeRequest.state);
    return <IconComponent size={12} />;
  };

  const renderHtmlContent = (
    html: string | null | undefined,
    fallback: string,
  ): JSX.Element => {
    if (!html || html.trim() === "") {
      return (
        <Typography variant="body2" color="text.secondary">
          {fallback}
        </Typography>
      );
    }
    return (
      <Box
        component="div"
        sx={{
          typography: "body2",
          color: "text.secondary",
          "& p": { mb: 0.5 },
          "& p:last-child": { mb: 0 },
        }}
        // biome-ignore lint/security/noDangerouslySetInnerHtml: sanitized backend HTML content
        dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(html) }}
      />
    );
  };

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 2,
        minHeight: "100vh",
      }}
    >
      {/* Fixed Header: Back Button + Export */}
      <Box
        sx={{
          flexShrink: 0,
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          gap: 2,
          flexWrap: "wrap",
        }}
      >
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={() =>
            returnTo
              ? navigate(returnTo)
              : navigate(`/projects/${projectId}/${basePath}/change-requests`)
          }
          sx={{ alignSelf: "flex-start" }}
          variant="text"
        >
          Back to Change Requests
        </Button>
        <Button
          variant="contained"
          size="small"
          startIcon={<Download size={18} />}
          color="warning"
          onClick={() => {
            try {
              generateChangeRequestDetailsPdf(changeRequest, []);
            } catch (error) {
              const message =
                error instanceof Error
                  ? error.message
                  : "Failed to generate PDF";
              showError(message);
            }
          }}
        >
          Download Change Request PDF
        </Button>
      </Box>

      {/* Fixed Header: Component A (Database Changes section) */}
      <Box sx={{ flexShrink: 0 }}>
        <Paper variant="outlined" sx={{ p: 4 }}>
          <Box
            sx={{
              display: "flex",
              flexDirection: "column",
              gap: 1.5,
            }}
          >
            <Box sx={{ width: "100%" }}>
              <Box
                sx={{
                  display: "flex",
                  alignItems: "flex-start",
                  justifyContent: "space-between",
                  gap: 1,
                  mb: 1,
                  flexWrap: "wrap",
                }}
              >
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 1,
                    flexWrap: "wrap",
                  }}
                >
                  <Typography variant="h5" color="text.primary">
                    {changeRequest.title || "Not Available"}
                  </Typography>
                  {changeRequest.hasServiceOutage && (
                    <Chip
                      label="Service Outage"
                      size="small"
                      sx={{
                        bgcolor: alpha(colors.red[500], 0.1),
                        color: colors.red[800],
                        borderColor: alpha(colors.red[500], 0.2),
                        border: "1px solid",
                      }}
                    />
                  )}
                </Box>
                <Box
                  sx={{
                    display: "flex",
                    gap: 1,
                    flexWrap: "wrap",
                    justifyContent: "flex-end",
                  }}
                >
                  {changeRequest.impact?.label &&
                    typeof changeRequest.impact.label === "string" && (
                      <Chip
                        label={formatImpactLabel(changeRequest.impact.label)}
                        size="small"
                        sx={{
                          bgcolor: impactColor.bg,
                          color: impactColor.text,
                          borderColor: impactColor.border,
                          border: "1px solid",
                        }}
                      />
                    )}
                  {changeRequest.state?.label &&
                    typeof changeRequest.state.label === "string" && (
                      <Chip
                        icon={renderStateIcon()}
                        label={String(changeRequest.state.label)}
                        size="small"
                        sx={{
                          bgcolor: statusColor.bg,
                          color: statusColor.text,
                          borderColor: statusColor.border,
                          border: "1px solid",
                          "& .MuiChip-icon": {
                            color: statusColor.text,
                          },
                        }}
                      />
                    )}
                </Box>
              </Box>
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  gap: 2,
                  flexWrap: "wrap",
                  width: "100%",
                }}
              >
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 1.5,
                    fontSize: "0.875rem",
                    color: "text.secondary",
                    flexWrap: "wrap",
                  }}
                >
                  <Typography
                    variant="body2"
                    fontWeight={600}
                    color="text.primary"
                  >
                    {changeRequest.number}
                  </Typography>
                  <Typography variant="body2" color="text.disabled">
                    |
                  </Typography>
                  {changeRequest.createdOn && (
                    <>
                      <Typography variant="body2" color="text.secondary">
                        {`Created: ${formatDateTime(changeRequest.createdOn)}`}
                      </Typography>
                      <Typography
                        variant="body2"
                        color="text.disabled"
                        sx={{ ml: 1 }}
                      >
                        |
                      </Typography>
                    </>
                  )}
                  <Button
                    variant="text"
                    size="small"
                    startIcon={<ExternalLink size={14} />}
                    onClick={() => {
                      if (changeRequest.case?.id) {
                        navigate(
                          `/projects/${projectId}/support/cases/${changeRequest.case.id}`,
                        );
                      }
                    }}
                    disabled={!changeRequest.case?.id}
                    sx={{ minHeight: "unset", p: 0 }}
                  >
                    Service Request:{" "}
                    {changeRequest.case?.number || "Not Available"}
                  </Button>
                </Box>
                {canShowApprovalActions && (
                  <Stack direction="row" spacing={1} useFlexGap flexWrap="wrap">
                    {canShowProposeNewTime && (
                      <Button
                        size="small"
                        variant="outlined"
                        startIcon={<CalendarClock size={14} aria-hidden />}
                        onClick={() => setProposeTimeOpen(true)}
                        disabled={patchChangeRequest.isPending}
                        sx={{
                          height: 32,
                          color: colors.blue[700],
                          borderColor: colors.blue[300],
                          "&:hover": {
                            bgcolor: alpha(colors.blue[500], 0.08),
                            borderColor: colors.blue[400],
                          },
                        }}
                      >
                        Propose New Time
                      </Button>
                    )}
                    <Button
                      size="small"
                      variant="outlined"
                      startIcon={<FileCheck size={14} aria-hidden />}
                      onClick={handleApproveChange}
                      disabled={patchChangeRequest.isPending}
                      sx={{
                        height: 32,
                        color: colors.green[800],
                        borderColor: colors.green[300],
                        "&:hover": {
                          bgcolor: alpha(colors.green[500], 0.08),
                          borderColor: colors.green[400],
                        },
                      }}
                    >
                      Approve
                    </Button>
                    <Button
                      size="small"
                      variant="outlined"
                      startIcon={<X size={14} aria-hidden />}
                      onClick={handleRejectChange}
                      disabled={patchChangeRequest.isPending}
                      sx={{
                        height: 32,
                        color: colors.red[800],
                        borderColor: colors.red[300],
                        "&:hover": {
                          bgcolor: alpha(colors.red[500], 0.08),
                          borderColor: colors.red[400],
                        },
                      }}
                    >
                      Reject
                    </Button>
                  </Stack>
                )}
              </Box>
            </Box>
          </Box>
        </Paper>
      </Box>

      {/* Scrollable 2-Column Layout */}
      <Box
        sx={{
          display: "flex",
          flexDirection: { xs: "column", md: "row" },
          gap: 2,
          flex: 1,
          overflow: { xs: "visible", md: "hidden" },
        }}
      >
        {/* Left Column - Scrollable Content */}
        <Box
          sx={{
            flex: 1,
            overflow: { xs: "visible", md: "auto" },
            display: "flex",
            flexDirection: "column",
            gap: 2,
          }}
        >
          {/* Scheduled Maintenance Window Card */}
          <ScheduledMaintenanceWindowCard changeRequest={changeRequest} />

          {/* Deployment & Component Card */}
          <Paper variant="outlined">
            <Box sx={{ px: 3, pt: 3 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Server size={20} color={colors.grey[600]} />
                <Typography variant="h6">Deployment & Component</Typography>
              </Box>
            </Box>

            <Box sx={{ px: 3, py: 3 }}>
              <Box
                sx={{
                  display: "grid",
                  gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
                  gap: 3,
                }}
              >
                <Box>
                  <Typography
                    variant="body2"
                    color="text.secondary"
                    sx={{ mb: 1 }}
                  >
                    Deployment
                  </Typography>
                  <Typography variant="body2">
                    {typeof changeRequest.deployment?.label === "string"
                      ? changeRequest.deployment.label
                      : "Not Available"}
                  </Typography>
                </Box>
                <Box>
                  <Typography
                    variant="body2"
                    color="text.secondary"
                    sx={{ mb: 1 }}
                  >
                    Component
                  </Typography>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                    <Package size={16} color={colors.grey[400]} />
                    <Typography variant="body2">
                      {typeof changeRequest.deployedProduct?.label === "string"
                        ? changeRequest.deployedProduct.label
                        : "Not Available"}
                    </Typography>
                  </Box>
                </Box>
              </Box>
            </Box>
          </Paper>

          {/* Change Description Card */}
          <Paper variant="outlined">
            <Box sx={{ px: 3, pt: 3 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <FileText size={20} color={colors.grey[600]} />
                <Typography variant="h6">Change Description</Typography>
              </Box>
            </Box>

            <Box sx={{ px: 3, py: 3 }}>
              <Stack spacing={3}>
                <Box>
                  {renderHtmlContent(
                    changeRequest.description,
                    "No description available",
                  )}
                </Box>
              </Stack>
            </Box>
          </Paper>

          {/* Impact Analysis Card */}
          <Paper variant="outlined">
            <Box sx={{ px: 3, pt: 3 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <TriangleAlert size={20} color={colors.grey[600]} />
                <Typography variant="h6">Impact Analysis</Typography>
              </Box>
            </Box>

            <Box sx={{ px: 3, py: 3 }}>
              <Stack spacing={3}>
                <Box>
                  {renderHtmlContent(
                    changeRequest.impactDescription,
                    "No impact description available",
                  )}
                </Box>
              </Stack>
            </Box>
          </Paper>

          <Paper variant="outlined">
            <Box sx={{ px: 3, pt: 3 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <TriangleAlert size={20} color={colors.grey[600]} />
                <Typography variant="h6">Service Outage Details</Typography>
              </Box>
            </Box>

            <Box sx={{ px: 3, py: 3 }}>
              {renderHtmlContent(
                changeRequest.serviceOutage,
                "No service outage details available.",
              )}
            </Box>
          </Paper>

          {/* Communication Plan Card */}
          <Paper variant="outlined">
            <Box sx={{ px: 3, pt: 3 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Users size={20} color={colors.grey[600]} />
                <Typography variant="h6">Communication Plan</Typography>
              </Box>
            </Box>

            <Box sx={{ px: 3, py: 3 }}>
              {renderHtmlContent(
                changeRequest.communicationPlan,
                "No communication plan available",
              )}
            </Box>
          </Paper>

          {/* Rollback Plan Card */}
          <Paper variant="outlined">
            <Box sx={{ px: 3, pt: 3 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <RotateCcw size={20} color={colors.grey[600]} />
                <Typography variant="h6">Rollback Plan</Typography>
              </Box>
            </Box>

            <Box sx={{ px: 3, py: 3 }}>
              {renderHtmlContent(
                changeRequest.rollbackPlan,
                "No rollback plan available",
              )}
            </Box>
          </Paper>

          {/* Test Plan Card */}
          <Paper variant="outlined">
            <Box sx={{ px: 3, pt: 3 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Shield size={20} color={colors.grey[600]} />
                <Typography variant="h6">Test Plan</Typography>
              </Box>
            </Box>

            <Box sx={{ px: 3, py: 3 }}>
              {renderHtmlContent(
                changeRequest.testPlan,
                "No test plan available",
              )}
            </Box>
          </Paper>

          {/* Approval Information Card */}
          <Paper variant="outlined">
            <Box sx={{ px: 3, pt: 3, pb: 3 }}>
              <Typography variant="h6" sx={{ mb: 2 }}>
                Approval Information
              </Typography>

              <Stack spacing={1.5}>
                <Box sx={{ display: "flex", justifyContent: "space-between" }}>
                  <Typography variant="body2" color="text.secondary">
                    Created By
                  </Typography>
                  <Typography variant="body2">
                    {changeRequest.createdBy || "Not Available"}
                  </Typography>
                </Box>
                <Box sx={{ display: "flex", justifyContent: "space-between" }}>
                  <Typography variant="body2" color="text.secondary">
                    Created Date
                  </Typography>
                  <Typography variant="body2">
                    {changeRequest.createdOn
                      ? formatDateTime(changeRequest.createdOn)
                      : "Not Available"}
                  </Typography>
                </Box>

                <Divider />

                <Box
                  sx={{
                    display: "flex",
                    justifyContent: "space-between",
                  }}
                >
                  <Typography variant="body2" color="text.secondary">
                    Approved By
                  </Typography>
                  <Typography variant="body2">
                    {changeRequest.approvedBy?.label || "Not available"}
                  </Typography>
                </Box>
                <Box
                  sx={{
                    display: "flex",
                    justifyContent: "space-between",
                  }}
                >
                  <Typography variant="body2" color="text.secondary">
                    Approved Date
                  </Typography>
                  <Typography variant="body2">
                    {changeRequest.approvedOn || "Not available"}
                  </Typography>
                </Box>
              </Stack>
            </Box>
          </Paper>
        </Box>

        {/* Right Column - Workflow (Fixed Width, Scrollable) */}
        <Box
          sx={{
            width: { xs: "100%", md: 400 },
            flexShrink: 0,
            overflow: { xs: "visible", md: "auto" },
          }}
        >
          <Paper variant="outlined">
            <Box sx={{ px: 3, pt: 3 }}>
              <Box
                sx={{ display: "flex", alignItems: "center", gap: 1, mb: 0.5 }}
              >
                <Typography variant="h6">Change Request Workflow</Typography>
              </Box>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                Track the progress of this change request through each stage
              </Typography>
            </Box>

            <Box sx={{ px: 3, pb: 3 }}>
              <Stack spacing={0}>
                {workflowStages.map((stage, index) => (
                  <Box key={index} sx={{ position: "relative" }}>
                    <Box sx={{ display: "flex", gap: 2 }}>
                      <Box
                        sx={{
                          display: "flex",
                          flexDirection: "column",
                          alignItems: "center",
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
                            border: "2px solid",
                            bgcolor: stage.disabled
                              ? alpha(colors.grey[500], 0.05)
                              : stage.completed
                                ? alpha(colors.green[500], 0.1)
                                : stage.current
                                  ? alpha(colors.blue[500], 0.1)
                                  : alpha(colors.grey[500], 0.1),
                            borderColor: stage.disabled
                              ? colors.grey[200]
                              : stage.completed
                                ? colors.green[500]
                                : stage.current
                                  ? colors.blue[500]
                                  : colors.grey[300],
                            opacity: stage.disabled ? 0.5 : 1,
                          }}
                        >
                          {stage.completed ? (
                            <CircleCheckBig
                              size={20}
                              color={colors.green[600]}
                            />
                          ) : (
                            <Circle
                              size={20}
                              color={
                                stage.current
                                  ? colors.blue[600]
                                  : colors.grey[400]
                              }
                              fill={stage.current ? colors.blue[600] : "none"}
                            />
                          )}
                        </Box>
                        {index < workflowStages.length - 1 && (
                          <Box
                            sx={{
                              width: 2,
                              height: 64,
                              mt: 0.5,
                              bgcolor:
                                currentStateIndex > index
                                  ? colors.green[300]
                                  : colors.grey[200],
                              opacity: 1,
                            }}
                          />
                        )}
                      </Box>
                      <Box
                        sx={{
                          flex: 1,
                          pb: index < workflowStages.length - 1 ? 2 : 0,
                        }}
                      >
                        <Box
                          sx={{
                            display: "flex",
                            justifyContent: "space-between",
                            alignItems: "flex-start",
                          }}
                        >
                          <Box>
                            <Box
                              sx={{
                                display: "flex",
                                alignItems: "center",
                                gap: 1,
                                mb: 0.5,
                              }}
                            >
                              <Typography
                                variant="body2"
                                fontWeight={stage.current ? 600 : 500}
                                color={
                                  stage.disabled
                                    ? "text.disabled"
                                    : stage.current
                                      ? colors.blue[900]
                                      : "text.primary"
                                }
                                sx={{ opacity: stage.disabled ? 0.5 : 1 }}
                              >
                                {stage.name}
                              </Typography>
                              {stage.current && !stage.disabled && (
                                <Chip
                                  label="Current"
                                  size="small"
                                  sx={{
                                    bgcolor: alpha(colors.blue[500], 0.1),
                                    color: colors.blue[800],
                                    borderColor: alpha(colors.blue[500], 0.2),
                                    border: "1px solid",
                                    height: 20,
                                    fontSize: "0.7rem",
                                  }}
                                />
                              )}
                            </Box>
                            <Typography
                              variant="caption"
                              color={
                                stage.disabled
                                  ? "text.disabled"
                                  : stage.completed || stage.current
                                    ? "text.primary"
                                    : "text.disabled"
                              }
                              sx={{ opacity: stage.disabled ? 0.5 : 1 }}
                            >
                              {stage.description}
                            </Typography>
                          </Box>
                        </Box>
                      </Box>
                    </Box>
                  </Box>
                ))}
              </Stack>
            </Box>
          </Paper>
        </Box>
      </Box>

      <ProposeNewImplementationTimeModal
        open={proposeTimeOpen}
        onClose={() => setProposeTimeOpen(false)}
        changeRequest={changeRequest}
      />
    </Box>
  );
}
