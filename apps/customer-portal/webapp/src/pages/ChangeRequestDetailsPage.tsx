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

import { useParams, useNavigate } from "react-router";
import { type JSX, useMemo } from "react";
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
  Skeleton,
} from "@wso2/oxygen-ui";
import {
  ArrowLeft,
  TriangleAlert,
  ChevronRight,
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
} from "@wso2/oxygen-ui-icons-react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import ErrorStateIcon from "@components/common/error-state/ErrorStateIcon";
import useGetChangeRequestDetails from "@api/useGetChangeRequestDetails";
import ScheduledMaintenanceWindowCard from "@components/support/change-requests/ScheduledMaintenanceWindowCard";
import { generateChangeRequestDetailsPdf } from "@utils/changeRequestDetailsPdf";
import {
  formatImpactLabel,
  getChangeRequestStateIcon,
  getChangeRequestImpactColorShades,
  getChangeRequestStateColorShades,
  ChangeRequestStates,
  type ChangeRequestState,
} from "@constants/supportConstants";

/**
 * State order for change request workflow
 */
const STATE_ORDER = [
  ChangeRequestStates.NEW,
  ChangeRequestStates.ASSESS,
  ChangeRequestStates.AUTHORIZE,
  ChangeRequestStates.CUSTOMER_APPROVAL,
  ChangeRequestStates.SCHEDULED,
  ChangeRequestStates.IMPLEMENT,
  ChangeRequestStates.REVIEW,
  ChangeRequestStates.CUSTOMER_REVIEW,
  ChangeRequestStates.ROLLBACK,
  ChangeRequestStates.CLOSED,
  ChangeRequestStates.CANCELED,
];

/**
 * Strip HTML tags from a string
 */
function stripHtmlTags(html: string | null | undefined): string {
  if (!html) return "";
  return html.replace(/<[^>]*>/g, "").trim();
}

/**
 * ChangeRequestDetailsPage component to display detailed information about a change request.
 *
 * @returns {JSX.Element} The rendered Change Request Details page.
 */
export default function ChangeRequestDetailsPage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId, changeRequestId } = useParams<{
    projectId: string;
    changeRequestId: string;
  }>();

  const { showError } = useErrorBanner();

  const {
    data: changeRequest,
    isLoading,
    error,
    isFetching,
  } = useGetChangeRequestDetails(changeRequestId || "");

  // Workflow stages with dynamic state
  const { workflowStages, currentStateIndex } = useMemo(() => {
    if (!changeRequest) return { workflowStages: [], currentStateIndex: -1 };

    const currentState =
      (changeRequest.state?.label as ChangeRequestState) ||
      ChangeRequestStates.NEW;
    const { hasCustomerApproved, hasCustomerReviewed } = changeRequest;

    const currentIndex = STATE_ORDER.indexOf(currentState);

    const stages = [
      {
        name: ChangeRequestStates.NEW,
        description: "Change request created",
        completed: currentIndex > 0,
        current: currentState === ChangeRequestStates.NEW,
        disabled: false,
      },
      {
        name: ChangeRequestStates.ASSESS,
        description: "Technical assessment completed",
        completed: currentIndex > 1,
        current: currentState === ChangeRequestStates.ASSESS,
        disabled: false,
      },
      {
        name: ChangeRequestStates.AUTHORIZE,
        description: "Internal authorization obtained",
        completed: currentIndex > 2,
        current: currentState === ChangeRequestStates.AUTHORIZE,
        disabled: false,
      },
      {
        name: ChangeRequestStates.CUSTOMER_APPROVAL,
        description: "Customer approval received",
        completed: currentIndex > 3 && hasCustomerApproved,
        current: currentState === ChangeRequestStates.CUSTOMER_APPROVAL,
        disabled:
          (currentState === ChangeRequestStates.IMPLEMENT ||
            currentState === ChangeRequestStates.REVIEW) &&
          !hasCustomerApproved,
      },
      {
        name: ChangeRequestStates.SCHEDULED,
        description: "Maintenance window scheduled",
        completed: currentIndex > 4,
        current: currentState === ChangeRequestStates.SCHEDULED,
        disabled: false,
      },
      {
        name: ChangeRequestStates.IMPLEMENT,
        description: "Change implementation",
        completed: currentIndex > 5,
        current: currentState === ChangeRequestStates.IMPLEMENT,
        disabled: false,
      },
      {
        name: ChangeRequestStates.REVIEW,
        description: "Internal review",
        completed: currentIndex > 6,
        current: currentState === ChangeRequestStates.REVIEW,
        disabled: false,
      },
      {
        name: ChangeRequestStates.CUSTOMER_REVIEW,
        description: "Customer validation",
        completed: currentIndex > 7 && hasCustomerReviewed,
        current: currentState === ChangeRequestStates.CUSTOMER_REVIEW,
        disabled:
          (currentState === ChangeRequestStates.ROLLBACK ||
            currentState === ChangeRequestStates.CLOSED ||
            currentState === ChangeRequestStates.CANCELED) &&
          !hasCustomerReviewed,
      },
      {
        name: ChangeRequestStates.ROLLBACK,
        description: "Change rollback if needed",
        completed: false,
        current: currentState === ChangeRequestStates.ROLLBACK,
        disabled:
          currentState === ChangeRequestStates.CLOSED ||
          currentState === ChangeRequestStates.CANCELED,
      },
      {
        name: ChangeRequestStates.CLOSED,
        description: "Change request completed",
        completed: false,
        current: currentState === ChangeRequestStates.CLOSED,
        disabled:
          currentState === ChangeRequestStates.CANCELED ||
          currentState === ChangeRequestStates.ROLLBACK,
      },
      {
        name: ChangeRequestStates.CANCELED,
        description: "Change request canceled",
        completed: false,
        current: currentState === ChangeRequestStates.CANCELED,
        disabled:
          currentState === ChangeRequestStates.CLOSED ||
          currentState === ChangeRequestStates.ROLLBACK,
      },
    ];

    return { workflowStages: stages, currentStateIndex: currentIndex };
  }, [changeRequest]);

  const impactColor = getChangeRequestImpactColorShades(
    changeRequest?.impact?.label,
  );
  const statusColor = getChangeRequestStateColorShades(
    changeRequest?.state?.label,
  );

  // Render state icon based on state label
  const renderStateIcon = () => {
    const IconComponent = getChangeRequestStateIcon(
      changeRequest?.state?.label,
    );
    return <IconComponent size={12} />;
  };

  // Loading state with skeleton (or if fetching/no data yet)
  if (isLoading || (isFetching && !changeRequest)) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {/* Header Skeleton */}
        <Paper variant="outlined" sx={{ p: 4 }}>
          <Box
            sx={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "flex-start",
            }}
          >
            <Box sx={{ flex: 1 }}>
              <Skeleton variant="text" width="60%" height={40} sx={{ mb: 1 }} />
              <Stack
                direction="row"
                spacing={1.5}
                sx={{ alignItems: "center" }}
              >
                <Skeleton variant="text" width={100} />
                <Skeleton variant="text" width={150} />
                <Skeleton variant="text" width={200} />
              </Stack>
            </Box>
            <Stack direction="row" spacing={1}>
              <Skeleton variant="rounded" width={100} height={24} />
              <Skeleton variant="rounded" width={100} height={24} />
            </Stack>
          </Box>
        </Paper>

        {/* Workflow Skeleton */}
        <Paper variant="outlined" sx={{ p: 3 }}>
          <Box sx={{ mb: 2 }}>
            <Skeleton variant="text" width={250} height={32} />
            <Skeleton variant="text" width="60%" height={20} />
          </Box>
          <Stack spacing={0}>
            {[1, 2, 3, 4, 5].map((i) => (
              <Box key={i} sx={{ display: "flex", gap: 2 }}>
                <Box
                  sx={{
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "center",
                  }}
                >
                  <Skeleton variant="circular" width={40} height={40} />
                  {i < 5 && (
                    <Box sx={{ width: 2, height: 64, mt: 0.5 }}>
                      <Skeleton variant="rectangular" width={2} height={64} />
                    </Box>
                  )}
                </Box>
                <Box sx={{ flex: 1, pb: i < 5 ? 2 : 0 }}>
                  <Skeleton variant="text" width="30%" height={20} />
                  <Skeleton variant="text" width="60%" height={16} />
                </Box>
              </Box>
            ))}
          </Stack>
        </Paper>

        {/* Content Cards Skeleton */}
        {[1, 2, 3, 4].map((i) => (
          <Paper key={i} variant="outlined" sx={{ p: 3 }}>
            <Skeleton variant="text" width={200} height={32} sx={{ mb: 2 }} />
            <Skeleton variant="text" width="100%" />
            <Skeleton variant="text" width="90%" />
            <Skeleton variant="text" width="95%" />
          </Paper>
        ))}
      </Box>
    );
  }

  // Error state - only show error if we have an actual error and not loading
  if (error && !isLoading && !isFetching) {
    return (
      <Stack spacing={3}>
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate(`/${projectId}/support/change-requests`)}
          sx={{ alignSelf: "flex-start" }}
          variant="text"
        >
          Back to Change Requests
        </Button>
        <Paper variant="outlined" sx={{ p: 6, textAlign: "center" }}>
          <ErrorStateIcon style={{ width: 200, height: "auto" }} />
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

  // Loading state or no data yet
  if (!changeRequest) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {/* Header Skeleton */}
        <Paper variant="outlined" sx={{ p: 4 }}>
          <Box
            sx={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "flex-start",
            }}
          >
            <Box sx={{ flex: 1 }}>
              <Skeleton variant="text" width="60%" height={40} sx={{ mb: 1 }} />
              <Stack
                direction="row"
                spacing={1.5}
                sx={{ alignItems: "center" }}
              >
                <Skeleton variant="text" width={100} />
                <Skeleton variant="text" width={150} />
                <Skeleton variant="text" width={200} />
              </Stack>
            </Box>
            <Stack direction="row" spacing={1}>
              <Skeleton variant="rounded" width={100} height={24} />
              <Skeleton variant="rounded" width={100} height={24} />
            </Stack>
          </Box>
        </Paper>

        {/* Workflow Skeleton */}
        <Paper variant="outlined" sx={{ p: 3 }}>
          <Box sx={{ mb: 2 }}>
            <Skeleton variant="text" width={250} height={32} />
            <Skeleton variant="text" width="60%" height={20} />
          </Box>
          <Stack spacing={0}>
            {[1, 2, 3, 4, 5].map((i) => (
              <Box key={i} sx={{ display: "flex", gap: 2 }}>
                <Box
                  sx={{
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "center",
                  }}
                >
                  <Skeleton variant="circular" width={40} height={40} />
                  {i < 5 && (
                    <Box sx={{ width: 2, height: 64, mt: 0.5 }}>
                      <Skeleton variant="rectangular" width={2} height={64} />
                    </Box>
                  )}
                </Box>
                <Box sx={{ flex: 1, pb: i < 5 ? 2 : 0 }}>
                  <Skeleton variant="text" width="30%" height={20} />
                  <Skeleton variant="text" width="60%" height={16} />
                </Box>
              </Box>
            ))}
          </Stack>
        </Paper>

        {/* Content Cards Skeleton */}
        {[1, 2, 3, 4].map((i) => (
          <Paper key={i} variant="outlined" sx={{ p: 3 }}>
            <Skeleton variant="text" width={200} height={32} sx={{ mb: 2 }} />
            <Skeleton variant="text" width="100%" />
            <Skeleton variant="text" width="90%" />
            <Skeleton variant="text" width="95%" />
          </Paper>
        ))}
      </Box>
    );
  }

  // Compute stripped values for proper fallback rendering
  const descriptionText = stripHtmlTags(changeRequest.description);
  const impactText = stripHtmlTags(changeRequest.impactDescription);
  const serviceOutageText = stripHtmlTags(changeRequest.serviceOutage);
  const communicationPlanText = stripHtmlTags(changeRequest.communicationPlan);
  const rollbackPlanText = stripHtmlTags(changeRequest.rollbackPlan);
  const testPlanText = stripHtmlTags(changeRequest.testPlan);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {/* Back Button */}
      <Button
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(`/${projectId}/support/change-requests`)}
        sx={{ alignSelf: "flex-start" }}
        variant="text"
      >
        Back to Change Requests
      </Button>

      {/* Header Section */}
      <Paper variant="outlined" sx={{ p: 4 }}>
        <Box
          sx={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "flex-start",
          }}
        >
          <Box>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 1 }}>
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
                alignItems: "center",
                gap: 1.5,
                fontSize: "0.875rem",
                color: "text.secondary",
              }}
            >
              <Typography variant="body2" fontWeight={600} color="text.primary">
                {changeRequest.number}
              </Typography>
              <Typography variant="body2" color="text.disabled">
                |
              </Typography>
              <Typography variant="body2">
                Service Request: {changeRequest.case?.number || "Not Available"}
              </Typography>
              <Typography variant="body2" color="text.disabled">
                |
              </Typography>
              <Typography variant="body2">
                Created {changeRequest.createdOn}
              </Typography>
            </Box>
          </Box>

          <Box sx={{ display: "flex", gap: 1 }}>
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
      </Paper>

      {/* Content Section */}
      {/* Workflow Card */}
      <Paper variant="outlined">
        <Box sx={{ px: 3, pt: 3 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 0.5 }}>
            <ChevronRight size={20} />
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
                        <CircleCheckBig size={20} color={colors.green[600]} />
                      ) : (
                        <Circle
                          size={20}
                          color={
                            stage.current ? colors.blue[600] : colors.grey[400]
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
                              label="Current Stage"
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
          <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 3 }}>
            <Box>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                Deployment
              </Typography>
              <Typography variant="body2">
                {typeof changeRequest.deployment?.label === "string"
                  ? changeRequest.deployment.label
                  : "Not Available"}
              </Typography>
            </Box>
            <Box>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
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
              <Typography
                variant="body2"
                color="text.primary"
                fontWeight={500}
                sx={{ mb: 1 }}
              >
                Change Description
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {descriptionText || "No description available"}
              </Typography>
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
              <Typography
                variant="body2"
                color="text.primary"
                fontWeight={500}
                sx={{ mb: 1 }}
              >
                Impact Description
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {impactText || "No impact description available"}
              </Typography>
            </Box>

            {changeRequest.hasServiceOutage && (
              <>
                <Divider />

                <Box>
                  <Typography
                    variant="body2"
                    color="text.primary"
                    fontWeight={500}
                    sx={{ mb: 2 }}
                  >
                    Service Outage Details
                  </Typography>
                  <Paper
                    sx={{
                      bgcolor: alpha(colors.red[500], 0.05),
                      border: 1,
                      borderColor: alpha(colors.red[500], 0.2),
                      p: 2,
                    }}
                  >
                    <Box sx={{ display: "flex", gap: 1.5 }}>
                      <TriangleAlert
                        size={20}
                        color={colors.red[600]}
                        style={{ marginTop: 2 }}
                      />
                      <Typography variant="body2" color="text.primary">
                        {serviceOutageText || "Service outage details not available"}
                      </Typography>
                    </Box>
                  </Paper>
                </Box>
              </>
            )}
          </Stack>
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
          <Typography variant="body2" color="text.secondary">
            {communicationPlanText || "No communication plan available"}
          </Typography>
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
          <Typography variant="body2" color="text.secondary">
            {rollbackPlanText || "No rollback plan available"}
          </Typography>
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
          <Typography variant="body2" color="text.secondary">
            {testPlanText || "No test plan available"}
          </Typography>
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
              <Typography variant="body2">{changeRequest.createdOn}</Typography>
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

      {/* Action Buttons */}
      <Box sx={{ display: "flex", gap: 2 }}>
        <Button
          variant="contained"
          startIcon={<Download size={18} />}
          sx={{ flex: 1 }}
          onClick={() => {
            try {
              generateChangeRequestDetailsPdf(changeRequest, []);
            } catch (error) {
              const message = error instanceof Error ? error.message : "Failed to generate PDF";
              showError(message);
              console.error("PDF generation error:", error);
            }
          }}
        >
          Download Change Request PDF
        </Button>
        <Button
          variant="contained"
          startIcon={<ExternalLink size={18} />}
          sx={{ flex: 1 }}
          onClick={() => {
            if (changeRequest.case?.id) {
              navigate(`/${projectId}/support/cases/${changeRequest.case.id}`);
            }
          }}
          disabled={!changeRequest.case?.id}
        >
          View Related Service Request
        </Button>
      </Box>
    </Box>
  );
}
