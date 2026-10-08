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

import { useNavigate, useLocation } from "react-router";
import useNormalizedIdParam from "@hooks/useNormalizedIdParam";
import { type JSX, useCallback, useEffect, useMemo, useRef, useState } from "react";
import DOMPurify from "dompurify";
import { DESCRIPTION_PURIFY_CONFIG } from "@utils/common";
import { useDarkMode } from "@utils/useDarkMode";
import {
  Alert,
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
import ApiErrorState from "@components/error/ApiErrorState";
import { isNotFoundError } from "@utils/ApiError";
import useGetChangeRequestDetails from "@features/operations/api/useGetChangeRequestDetails";
import { usePatchChangeRequest } from "@features/operations/api/usePatchChangeRequest";
import ScheduledMaintenanceWindowCard from "@features/operations/components/change-requests/ScheduledMaintenanceWindowCard";
import ProposeNewImplementationTimeModal from "@features/operations/components/change-requests/ProposeNewImplementationTimeModal";
import ChangeRequestRejectConfirmDialog from "@features/operations/components/change-requests/ChangeRequestRejectConfirmDialog";
import ChangeRequestDetailsLoadingSkeleton from "@features/operations/components/change-requests/ChangeRequestDetailsLoadingSkeleton";
import {
  CHANGE_REQUEST_NOT_FOUND_MESSAGE,
  buildChangeRequestWorkflowStages,
  describeChangeRequestActionError,
  generateChangeRequestDetailsPdf,
  getAnsweredWindow,
  getCustomerDecisionLabels,
  getCustomerDecisionMessages,
  getProposalNote,
  isAwaitingInternalReview,
  resolveCustomerDecisionMode,
} from "@features/operations/utils/changeRequests";
import { getChangeRequestWindow } from "@features/operations/utils/changeRequestSchedule";
import { formatDateTime } from "@features/support/utils/support";
import {
  formatImpactLabel,
  getChangeRequestImpactColorShades,
} from "@features/operations/utils/changeRequestUi";
import {
  ChangeRequestDecisionMode,
  type ProposeNewTimeAvailability,
} from "@features/operations/types/changeRequests";

/**
 * Where focus goes once the customer is done with an answer dialog or an answer
 * attempt has ended (see `requestFocus`): the page heading, which is always there,
 * or the answer button that was used, while it can still be used.
 */
type FocusAfterAnswer = "heading" | "trigger";

/**
 * ChangeRequestDetailsPage component to display detailed information about a change request.
 *
 * @returns {JSX.Element} The rendered Change Request Details page.
 */
export default function ChangeRequestDetailsPage(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const projectId = useNormalizedIdParam("projectId");
  const changeRequestId = useNormalizedIdParam("changeRequestId");
  const basePath = location.pathname.includes("/operations/")
    ? "operations"
    : "support";
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;

  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();
  const [proposeTimeOpen, setProposeTimeOpen] = useState(false);
  const [rejectConfirmOpen, setRejectConfirmOpen] = useState(false);
  const answerInFlightRef = useRef(false);
  // Where focus goes whenever an answer attempt or one of the two dialogs ends,
  // by whatever route -- given, refused for good, failed, or just closed. The
  // buttons that had focus are gone, switched off or about to go by then (a
  // refused answer re-reads the change request, and an answered one no longer
  // offers any), so a keyboard or screen reader user would otherwise land on the
  // document body and start again from the top. The page heading is always
  // there; the answer button used last is where a customer who can still answer
  // belongs (after a proposal that is the Propose New Time button: the change
  // request stays in Customer Approval with its buttons). The outcome itself is
  // announced by the banner (an alert).
  const headingRef = useRef<HTMLElement | null>(null);
  const answerTriggerRef = useRef<HTMLElement | null>(null);
  const [pendingFocus, setPendingFocus] = useState<FocusAfterAnswer | null>(null);
  const requestFocus = useCallback((target: FocusAfterAnswer) => {
    // The heading is the stronger claim: an answer that is over leaves no button
    // to return to, whatever else closed at the same time.
    setPendingFocus((current) => (current === "heading" ? current : target));
  }, []);
  // The app's own dark-mode signal (<html data-color-scheme>), which theme.palette.mode does not follow.
  const isDark = useDarkMode();

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
  const decisionMode = resolveCustomerDecisionMode(changeRequest);
  const canShowApprovalActions = decisionMode !== ChangeRequestDecisionMode.NONE;
  const canShowProposeNewTime = decisionMode === ChangeRequestDecisionMode.CUSTOMER_APPROVAL;
  // A held change refuses a proposed time (but not an answer): offer the button
  // switched off, with the reason beside it, rather than let a customer type a
  // whole window and be refused.
  const proposeBlockedByHold = canShowProposeNewTime && changeRequest?.isOnHold === true;
  // A proposal moves the start and keeps the planned length, so a change request
  // with no window (no start, no end, or an end that is not after the start) has
  // nothing to move: the button is switched off with the reason beside it.
  const proposeBlockedByNoWindow =
    canShowProposeNewTime &&
    !proposeBlockedByHold &&
    changeRequest != null &&
    getChangeRequestWindow(changeRequest).durationMs == null;
  const proposeNoteId = proposeBlockedByHold
    ? "cr-propose-hold-note"
    : proposeBlockedByNoWindow
      ? "cr-propose-nowindow-note"
      : undefined;
  // What the reject confirmation may say about proposing a different time: only
  // what is true of the button beside it. A button that is switched off for want
  // of a window to move is not pointed at, and the hold is the one reason that is
  // worth saying (it passes).
  const proposeNewTime: ProposeNewTimeAvailability = !canShowProposeNewTime
    ? "unavailable"
    : proposeBlockedByHold
      ? "on_hold"
      : proposeBlockedByNoWindow
        ? "unavailable"
        : "available";
  const proposeDialogOpen = proposeTimeOpen && canShowProposeNewTime;
  const rejectDialogOpen = rejectConfirmOpen && canShowApprovalActions;
  // What the customer is told about a proposed time that waits for WSO2 or was not accepted.
  const proposalNote = getProposalNote(changeRequest, canShowApprovalActions);
  const { approve: approveLabel, reject: rejectLabel } =
    getCustomerDecisionLabels(decisionMode);

  // Outlined answer buttons. Lighter shades in dark mode and darker ones in light:
  // one fixed shade for both read 3.6 to 4.3 : 1 on the dark page (AA text needs 4.5).
  const answerButtonSx = (hue: "blue" | "green" | "red") => ({
    height: 32,
    color: isDark ? colors[hue][300] : colors[hue][800],
    borderColor: isDark ? colors[hue][400] : colors[hue][300],
    "&:hover": {
      bgcolor: alpha(colors[hue][500], isDark ? 0.16 : 0.08),
      borderColor: isDark ? colors[hue][300] : colors[hue][400],
    },
  });

  // Runs once the dialogs have closed (so it is the last to move focus, after the
  // dialog's own return of focus) and no answer is in flight (the answer buttons
  // are switched off while one is, and cannot take focus).
  useEffect(() => {
    if (
      !pendingFocus ||
      proposeDialogOpen ||
      rejectDialogOpen ||
      patchChangeRequest.isPending
    ) {
      return;
    }
    setPendingFocus(null);
    const trigger = answerTriggerRef.current;
    const triggerUsable =
      pendingFocus === "trigger" &&
      trigger?.isConnected === true &&
      !(trigger as HTMLButtonElement).disabled;
    (triggerUsable ? trigger : headingRef.current)?.focus();
  }, [pendingFocus, proposeDialogOpen, rejectDialogOpen, patchChangeRequest.isPending]);

  // The error state replaces the whole page. A refused answer re-reads the change
  // request, and that read can fail (the contact was deregistered meanwhile, the
  // change request is gone: a 404): the heading or the button that had focus is then
  // unmounted and focus would drop to the document body. So the error state takes it,
  // but only when focus has nowhere else to be -- never from a control the customer
  // has moved to since.
  const errorStateShown = !!error && !isLoading && !isFetching;
  const errorStateRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!errorStateShown) return;
    const active = document.activeElement;
    if (active && active !== document.body && active.isConnected) return;
    errorStateRef.current?.focus();
  }, [errorStateShown]);

  const impactColor = getChangeRequestImpactColorShades(
    changeRequest?.impact?.label,
  );

  /**
   * Sends the customer's answer. The patch hook refetches the change request
   * before this resolves, so the page shows the new state (and no buttons) as
   * soon as the message appears. Awaited rather than passed as `mutate`
   * callbacks, which would be dropped if the refetch hid this page's buttons
   * and unmounted whatever called us.
   */
  const submitAnswer = async (approved: boolean) => {
    if (!changeRequest || answerInFlightRef.current) return;
    const mode = decisionMode;
    if (mode === ChangeRequestDecisionMode.NONE) return;
    const messages = getCustomerDecisionMessages(mode, approved);
    answerInFlightRef.current = true;
    try {
      // The answer names the schedule on screen: if it moved while the page was
      // open (a re-schedule was approved behind it), it is refused, not given
      // for a time the customer never saw.
      const shown = getAnsweredWindow(changeRequest);
      await patchChangeRequest.mutateAsync(
        mode === ChangeRequestDecisionMode.CUSTOMER_REVIEW
          ? { isCustomerReviewed: approved, ...shown }
          : { isCustomerApproved: approved, ...shown },
      );
      showSuccess(messages.success);
      requestFocus("heading");
    } catch (err) {
      const { message, terminal } = describeChangeRequestActionError(err, messages.failure);
      showError(message);
      // A refusal that ends the question (answered already, moved on, not yours to
      // answer) leaves no answer to give: the heading. Any other failure leaves
      // the buttons as they were: back to the one that was used.
      requestFocus(terminal ? "heading" : "trigger");
    } finally {
      answerInFlightRef.current = false;
      setRejectConfirmOpen(false);
    }
  };

  // Loading state with skeleton (or if fetching/no data yet)
  if (isLoading || (isFetching && !changeRequest)) {
    return (
      <Stack spacing={2}>
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
        <ChangeRequestDetailsLoadingSkeleton />
      </Stack>
    );
  }

  // Error state - only show error if we have an actual error and not loading
  if (errorStateShown) {
    const errorMessage = isNotFoundError(error)
      ? CHANGE_REQUEST_NOT_FOUND_MESSAGE
      : "Could not load change request details.";
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
        {/* The focus target of the error state (see `errorStateShown`): named after the message so a screen reader says what happened. */}
        <Box
          ref={errorStateRef}
          tabIndex={-1}
          role="group"
          aria-label={errorMessage}
          sx={{ outline: "none" }}
        >
          <ApiErrorState error={error} fallbackMessage={errorMessage} />
        </Box>
      </Stack>
    );
  }

  if (!changeRequest) {
    return <ChangeRequestDetailsLoadingSkeleton />;
  }

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
        dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(html, DESCRIPTION_PURIFY_CONFIG) }}
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
          variant="outlined"
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
                  <Typography
                    variant="h5"
                    color="text.primary"
                    ref={headingRef}
                    tabIndex={-1}
                    sx={{ outline: "none" }}
                  >
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
                    {changeRequest.case?.internalId
                      ? `${changeRequest.case.internalId} | ${changeRequest.case?.number || "Not Available"}`
                      : (changeRequest.case?.number || "Not Available")}
                  </Button>
                </Box>
                {canShowApprovalActions && (
                  <Box
                    sx={{
                      display: "flex",
                      flexDirection: "column",
                      alignItems: { xs: "flex-start", sm: "flex-end" },
                      gap: 0.75,
                    }}
                  >
                    {decisionMode === ChangeRequestDecisionMode.CUSTOMER_REVIEW && (
                      <Typography
                        id="cr-answer-prompt"
                        variant="body2"
                        color="text.secondary"
                      >
                        This change has been implemented. Was it successful?
                      </Typography>
                    )}
                    <Stack
                      direction="row"
                      spacing={1}
                      useFlexGap
                      flexWrap="wrap"
                      role="group"
                      aria-labelledby={
                        decisionMode === ChangeRequestDecisionMode.CUSTOMER_REVIEW
                          ? "cr-answer-prompt"
                          : undefined
                      }
                      aria-label={
                        decisionMode === ChangeRequestDecisionMode.CUSTOMER_REVIEW
                          ? undefined
                          : "Answer this change request"
                      }
                    >
                      {canShowProposeNewTime && (
                        <Button
                          size="small"
                          variant="outlined"
                          startIcon={<CalendarClock size={14} aria-hidden />}
                          onClick={(event) => {
                            answerTriggerRef.current = event.currentTarget;
                            setProposeTimeOpen(true);
                          }}
                          disabled={
                            patchChangeRequest.isPending ||
                            proposeBlockedByHold ||
                            proposeBlockedByNoWindow
                          }
                          aria-describedby={proposeNoteId}
                          sx={answerButtonSx("blue")}
                        >
                          Propose New Time
                        </Button>
                      )}
                      <Button
                        size="small"
                        variant="outlined"
                        startIcon={<FileCheck size={14} aria-hidden />}
                        onClick={(event) => {
                          answerTriggerRef.current = event.currentTarget;
                          void submitAnswer(true);
                        }}
                        disabled={patchChangeRequest.isPending}
                        sx={answerButtonSx("green")}
                      >
                        {approveLabel}
                      </Button>
                      <Button
                        size="small"
                        variant="outlined"
                        startIcon={<X size={14} aria-hidden />}
                        onClick={(event) => {
                          answerTriggerRef.current = event.currentTarget;
                          setRejectConfirmOpen(true);
                        }}
                        disabled={patchChangeRequest.isPending}
                        sx={answerButtonSx("red")}
                      >
                        {rejectLabel}
                      </Button>
                    </Stack>
                    {proposeBlockedByHold && (
                      <Typography
                        id="cr-propose-hold-note"
                        variant="caption"
                        color="text.secondary"
                        role="note"
                        sx={{ maxWidth: 360, textAlign: { sm: "right" } }}
                      >
                        WSO2 has this change request on hold, so a new time cannot
                        be proposed right now. You can still approve or reject it.
                      </Typography>
                    )}
                    {proposeBlockedByNoWindow && (
                      <Typography
                        id="cr-propose-nowindow-note"
                        variant="caption"
                        color="text.secondary"
                        role="note"
                        sx={{ maxWidth: 360, textAlign: { sm: "right" } }}
                      >
                        This change request has no planned time yet, so a new
                        time cannot be proposed for it. You can still approve or
                        reject it.
                      </Typography>
                    )}
                  </Box>
                )}
              </Box>
            </Box>
          </Box>
        </Paper>
      </Box>

      {proposalNote && (
        <Alert
          severity="info"
          role="status"
          id={
            proposalNote.kind === "waiting"
              ? "cr-proposal-waiting-note"
              : "cr-proposal-not-accepted-note"
          }
        >
          {proposalNote.text}
        </Alert>
      )}

      {/* Only a proposal made before a proposed time waited in Customer Approval
          can leave a change request here: it goes back through WSO2's internal
          approval and the customer is then asked again. */}
      {isAwaitingInternalReview(changeRequest) && (
        <Alert severity="info" role="status" id="cr-internal-review-note">
          WSO2 is reviewing this change request internally. You will be asked to
          approve the schedule once it is confirmed.
        </Alert>
      )}

      {/* Scrollable 2-Column Layout */}
      <Box
        sx={{
          display: "flex",
          flexDirection: { xs: "column", md: "row" },
          gap: 3,
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
            pr: { xs: 0, md: 0.5 },
          }}
        >
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
            pl: { xs: 0, md: 0.5 },
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
                                <Box
                                  sx={{
                                    display: "flex",
                                    alignItems: "center",
                                    gap: 0.5,
                                  }}
                                >
                                  <Box
                                    sx={{
                                      width: 6,
                                      height: 6,
                                      bgcolor: colors.blue[600],
                                      borderRadius: "50%",
                                      flexShrink: 0,
                                    }}
                                  />
                                  <Typography
                                    variant="caption"
                                    sx={{
                                      color: colors.blue[800],
                                      fontSize: "0.7rem",
                                    }}
                                  >
                                    Current
                                  </Typography>
                                </Box>
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
        open={proposeDialogOpen}
        onClose={() => {
          setProposeTimeOpen(false);
          requestFocus("trigger");
        }}
        // A proposal leaves the change request in Customer Approval with its
        // buttons, so focus goes back to the one that opened the dialog (the
        // heading only when that button is gone: see the focus effect).
        onProposed={() => requestFocus("trigger")}
        onRefused={() => requestFocus("heading")}
        changeRequest={changeRequest}
      />
      <ChangeRequestRejectConfirmDialog
        open={rejectDialogOpen}
        mode={decisionMode}
        proposeNewTime={proposeNewTime}
        isPending={patchChangeRequest.isPending}
        onClose={() => {
          setRejectConfirmOpen(false);
          requestFocus("trigger");
        }}
        onConfirm={() => void submitAnswer(false)}
      />
    </Box>
  );
}
