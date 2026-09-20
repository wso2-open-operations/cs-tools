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

import type { CaseDetailsActionRowProps } from "@features/support/types/supportComponents";
import {
  Box,
  Button,
  CircularProgress,
  Stack,
  Tooltip,
  alpha,
  useTheme,
  type Theme,
} from "@wso2/oxygen-ui";
import CaseStateConfirmDialog from "@features/support/components/case-details/dialogs/CaseStateConfirmDialog";
import RejectSolutionDialog from "@features/support/components/case-details/dialogs/RejectSolutionDialog";
import EscalateCaseModal from "../escalation/EscalateCaseModal";
import DeescalateCaseModal from "../escalation/DeescalateCaseModal";
import CaseFeedbackModal from "../feedback/CaseFeedbackModal";
import { type JSX, useState } from "react";
import {
  CASE_STATUS_ACTIONS,
  CommentType,
  ESCALATION_LEAD_REQUIRED_FROM_LEVEL,
  ESCALATION_MAX_LEVEL_ID,
  ESCALATION_NEXT_LEVEL,
  type CaseStatusPaletteIntent,
} from "@features/support/constants/supportConstants";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { usePatchCase } from "@features/support/api/usePatchCase";
import { usePostComment } from "@features/support/api/usePostComment";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import {
  ACTION_TO_CASE_STATE_LABEL,
  getAvailableCaseActions,
  isWithinOpenRelatedCaseWindow,
  toPresentContinuousActionLabel,
  toPresentTenseActionLabel,
} from "@features/support/utils/support";
import { escapeHtml } from "@features/support/utils/richTextEditor";
import { TriangleAlert } from "@wso2/oxygen-ui-icons-react";

const ACTION_BUTTON_ICON_SIZE = 12;
const DEESCALATE_PERMISSION_TOOLTIP =
  "Only customer admins, project leads, or (at escalation level 1-3) the user who created the escalation can de-escalate this case.";
const REJECT_SOLUTION_LABEL = "Reject Solution";

function getActionButtonSx(
  theme: Theme,
  intent: CaseStatusPaletteIntent,
): Record<string, unknown> {
  const light = theme.palette[intent].light;
  return {
    borderColor: light,
    bgcolor: alpha(light, 0.1),
    color: light,
    fontSize: "0.7rem",
    minHeight: 0,
    py: 0.5,
    px: 1,
    "&:hover": {
      borderColor: theme.palette[intent].main,
      bgcolor: alpha(light, 0.2),
    },
    textTransform: "none",
  };
}

function getStateKeyForAction(
  actionLabel: string,
  caseStates?: { id: string; label: string }[],
): number | undefined {
  if (!caseStates?.length) return undefined;
  const stateLabel = ACTION_TO_CASE_STATE_LABEL[actionLabel];
  if (!stateLabel) return undefined;
  const entry = caseStates.find(
    (s) => s.label.toLowerCase() === stateLabel.toLowerCase(),
  );
  if (!entry?.id) return undefined;
  const num = Number(entry.id);
  return Number.isNaN(num) ? undefined : num;
}

export default function CaseDetailsActionRow({
  assignedEngineer,
  engineerInitials,
  statusLabel,
  closedOn,
  onOpenRelatedCase,
  projectId = "",
  caseId = "",
  isLoading = false,
  escalationLevelId,
  onEscalateSuccess,
  isCurrentUserLead,
  isEscalated,
  canDeescalate,
  onDeescalateSuccess,
}: CaseDetailsActionRowProps): JSX.Element {
  void assignedEngineer;
  void engineerInitials;
  void isLoading;
  const theme = useTheme();
  const { data: filterMetadata } = useGetProjectFilters(projectId);
  const caseStates = filterMetadata?.caseStates;

  const { showSuccess } = useSuccessBanner();
  const { showError } = useErrorBanner();

  const patchCase = usePatchCase(projectId, caseId);
  const postComment = usePostComment();
  const [pendingActionLabel, setPendingActionLabel] = useState<string | null>(null);
  const [confirmAction, setConfirmAction] = useState<{
    label: string;
    stateKey: number;
  } | null>(null);
  const [escalateModalOpen, setEscalateModalOpen] = useState(false);
  const [deescalateModalOpen, setDeescalateModalOpen] = useState(false);
  const [feedbackModalOpen, setFeedbackModalOpen] = useState(false);
  const [rejectDialogOpen, setRejectDialogOpen] = useState(false);
  const [isRejectSubmitting, setIsRejectSubmitting] = useState(false);

  const resolvedEscalationLevelId = escalationLevelId != null ? String(escalationLevelId) : null;
  const escalationLevelInfo = resolvedEscalationLevelId != null ? ESCALATION_NEXT_LEVEL[resolvedEscalationLevelId ?? "0"] : null;
  const needsLead = resolvedEscalationLevelId != null && ESCALATION_LEAD_REQUIRED_FROM_LEVEL.has(resolvedEscalationLevelId);
  const showEscalateButton =
    resolvedEscalationLevelId != null &&
    statusLabel !== "Closed" &&
    resolvedEscalationLevelId !== ESCALATION_MAX_LEVEL_ID &&
    !!escalationLevelInfo &&
    (!needsLead || isCurrentUserLead === true);
  const showDeescalateButton = isEscalated === true;
  const canDeescalateCase = canDeescalate === true;

  // Reject Solution requires a customer comment before the case flips back to
  // "Waiting On WSO2", so support engineers see why it bounced instead of a
  // silent state change. Comments and state changes are entirely separate
  // endpoints (POST /cases/:id/comments vs. PATCH /cases/:id with stateKey —
  // the PATCH endpoint accepts exactly one of stateKey/watchList and has no
  // comment field at all), so this must be two sequential calls: comment
  // first, then state — never the reverse, so a comment failure can never be
  // masked by an already-applied state flip.
  const handleRejectSolutionConfirm = (reason: string): void => {
    const stateKey = getStateKeyForAction(REJECT_SOLUTION_LABEL, caseStates);
    if (!caseId || stateKey == null) return;

    // Comment content is rendered as sanitized HTML, so the reason must be
    // escaped before splicing it in (preserves the customer's exact text
    // instead of it being parsed/stripped as markup) and the line break needs
    // an actual <br> — a plain "\n" would not render.
    const commentBody = reason
      ? `Proposed solution was rejected with following feedback:<br>${escapeHtml(reason)}`
      : "Proposed solution was rejected. No additional feedback was provided.";

    setIsRejectSubmitting(true);
    setPendingActionLabel(REJECT_SOLUTION_LABEL);
    postComment.mutate(
      { caseId, body: { content: commentBody, type: CommentType.COMMENT } },
      {
        onSuccess: () => {
          // Comment landed — proceed to flip state. A failure from here on is not
          // a data-loss risk (a separate reopen automation is expected to catch
          // it), so the dialog closes either way once the comment is in.
          patchCase.mutate(
            { stateKey },
            {
              onSuccess: () => {
                showSuccess("Solution rejected and feedback submitted.");
              },
              onError: (err) => {
                showError(
                  err?.message ??
                    "Feedback was submitted, but updating the case status failed.",
                );
              },
              onSettled: () => {
                setIsRejectSubmitting(false);
                setPendingActionLabel(null);
                setRejectDialogOpen(false);
              },
            },
          );
        },
        onError: (err) => {
          // Never attempt the state PATCH if the comment failed — the case must
          // stay in Solution Proposed rather than silently flip with no context.
          showError(err?.message ?? "Failed to submit feedback. Please try again.");
          setIsRejectSubmitting(false);
          setPendingActionLabel(null);
          // Keep the dialog open so the customer doesn't lose their typed reason.
        },
      },
    );
  };

  const availableActions = getAvailableCaseActions(statusLabel).filter(
    (label) => {
      if (label === "Open Related Case") {
        if (!onOpenRelatedCase) return false;
        if (!isWithinOpenRelatedCaseWindow(closedOn)) return false;
      }
      return true;
    },
  );

  return (
    <Stack
      direction="row"
      spacing={1}
      alignItems="center"
      flexWrap="wrap"
      sx={{
        justifyContent: "flex-end",
      }}
    >
      {CASE_STATUS_ACTIONS.filter((action) =>
        availableActions.includes(action.label),
      ).map(({ label, Icon, paletteIntent }) => {
        const stateKey = getStateKeyForAction(label, caseStates);
        const isOpenRelatedCase = label === "Open Related Case";
        const isRejectSolution = label === REJECT_SOLUTION_LABEL;
        const canPatch = !isOpenRelatedCase && stateKey != null && !!caseId;
        const isThisPending = isOpenRelatedCase
          ? false
          : isRejectSolution
            ? isRejectSubmitting
            : patchCase.isPending && pendingActionLabel === label;
        const isAnyActionPending = patchCase.isPending || isRejectSubmitting;

        return (
          <Button
            key={label}
            variant="outlined"
            size="small"
            startIcon={
              isThisPending ? (
                <CircularProgress
                  size={ACTION_BUTTON_ICON_SIZE}
                  color="inherit"
                  sx={{ display: "block" }}
                />
              ) : (
                <Icon size={ACTION_BUTTON_ICON_SIZE} />
              )
            }
            disabled={!isOpenRelatedCase && (isAnyActionPending || !canPatch)}
            onClick={
              isOpenRelatedCase
                ? onOpenRelatedCase
                : canPatch
                  ? () =>
                      isRejectSolution
                        ? setRejectDialogOpen(true)
                        : setConfirmAction({ label, stateKey: stateKey! })
                  : undefined
            }
            sx={getActionButtonSx(theme, paletteIntent) as Record<string, unknown>}
          >
            {isThisPending
              ? toPresentContinuousActionLabel(label)
              : toPresentTenseActionLabel(label)}
          </Button>
        );
      })}
      {showEscalateButton && (
        <Button
          variant="outlined"
          size="small"
          startIcon={<TriangleAlert size={ACTION_BUTTON_ICON_SIZE} />}
          onClick={() => setEscalateModalOpen(true)}
          sx={{
            borderColor: theme.palette.warning.light,
            bgcolor: alpha(theme.palette.warning.light, 0.1),
            color: theme.palette.warning.light,
            fontSize: "0.7rem",
            minHeight: 0,
            py: 0.5,
            px: 1,
            "&:hover": {
              borderColor: theme.palette.warning.main,
              bgcolor: alpha(theme.palette.warning.light, 0.2),
            },
            textTransform: "none",
          }}
        >
          Escalate Case
        </Button>
      )}
      {showDeescalateButton && (
        <Tooltip
          title={canDeescalateCase ? "" : DEESCALATE_PERMISSION_TOOLTIP}
          describeChild
        >
          <Box component="span">
            <Button
              variant="outlined"
              size="small"
              disabled={!canDeescalateCase}
              startIcon={<TriangleAlert size={ACTION_BUTTON_ICON_SIZE} />}
              onClick={() => setDeescalateModalOpen(true)}
              sx={getActionButtonSx(theme, "success") as Record<string, unknown>}
            >
              De-escalate Case
            </Button>
          </Box>
        </Tooltip>
      )}
      <CaseStateConfirmDialog
        open={!!confirmAction}
        actionLabel={confirmAction ? toPresentTenseActionLabel(confirmAction.label) : ""}
        isPending={patchCase.isPending}
        onClose={() => setConfirmAction(null)}
        onConfirm={() => {
          if (!confirmAction) return;
          const { label, stateKey } = confirmAction;
          setPendingActionLabel(label);
          patchCase.mutate(
            { stateKey },
            {
              onSuccess: () => {
                showSuccess("State updated successfully.");
                // Prompt for feedback when accepting a solution or closing the case.
                if ((label === "Accept Solution" || label === "Closed") && caseId) {
                  setFeedbackModalOpen(true);
                }
              },
              onError: (err) => {
                showError(
                  err?.message ?? "Failed to update case status. Please try again.",
                );
              },
              onSettled: () => {
                setPendingActionLabel(null);
                setConfirmAction(null);
              },
            },
          );
        }}
      />
      <RejectSolutionDialog
        open={rejectDialogOpen}
        isPending={isRejectSubmitting}
        onClose={() => setRejectDialogOpen(false)}
        onConfirm={handleRejectSolutionConfirm}
      />
      {showEscalateButton && (
        <EscalateCaseModal
          open={escalateModalOpen}
          caseId={caseId}
          escalationLevelId={resolvedEscalationLevelId}
          escalationLevelLabel={`EL${resolvedEscalationLevelId}`}
          onClose={() => setEscalateModalOpen(false)}
          onSuccess={() => {
            showSuccess("Case escalated successfully.");
            onEscalateSuccess?.();
          }}
          onError={(msg) => showError(msg)}
        />
      )}
      {showDeescalateButton && (
        <DeescalateCaseModal
          open={deescalateModalOpen}
          caseId={caseId}
          onClose={() => setDeescalateModalOpen(false)}
          onSuccess={() => {
            showSuccess("Case de-escalated successfully.");
            onDeescalateSuccess?.();
          }}
          onError={(msg) => showError(msg)}
        />
      )}
      {feedbackModalOpen && (
        <CaseFeedbackModal
          open
          caseId={caseId}
          onClose={() => setFeedbackModalOpen(false)}
          onSubmitted={() => showSuccess("Thanks for your feedback.")}
          onError={(msg) => showError(msg)}
        />
      )}
    </Stack>
  );
}
