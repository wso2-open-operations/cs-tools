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

import { Suspense, useState, type ReactNode } from "react";
import { useParams } from "react-router-dom";
import { Button, Divider, Skeleton, Stack, Tab, Tabs, Typography, pxToRem } from "@wso2/oxygen-ui";
import { useQueryClient, useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { cases, type MyOngoingCase } from "@src/services/cases";
import { currentUser } from "@src/services/currentUser";
import { attachments as attachmentsService } from "@src/services/attachments";
import { useUserStore } from "@src/store/user";
import type {
  CaseCause,
  CaseCommentType,
  CaseDetail,
  CaseResolutionCode,
  CaseSeverity,
  CaseState,
  CaseWorkState,
  Comment,
} from "@src/types";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { SeverityChip, StatusChip } from "@components/support/Chips";
import { ErrorState } from "@components/support/ErrorState";
import { TYPE_CONFIG } from "@components/support/config";
import { SectionCard } from "@components/case-detail/SectionCard";
import { CaseActionBar } from "@components/case-detail/CaseActionBar";
import { ResolutionDialog } from "@components/case-detail/ResolutionDialog";
import { ChangeSeverityDialog } from "@components/case-detail/ChangeSeverityDialog";
import { CommentComposer } from "@components/case-detail/CommentComposer";
import { CommentBody } from "@components/case-detail/CommentBody";
import { PauseConflictDialog } from "@components/case-detail/PauseConflictDialog";
import { SlaTab } from "@components/case-detail/SlaTab";
import { AttachmentsTab } from "@components/case-detail/AttachmentsTab";
import { CallRequestsTab } from "@components/case-detail/CallRequestsTab";
import { TimeTrackingTab } from "@components/case-detail/TimeTrackingTab";
import { formatDate } from "@utils/dateTime";
import { Logger } from "@utils/logger";
import { toApiError } from "@utils/ApiError";
import type { PendingAttachment } from "@utils/attachments";

type CaseTabId = "activities" | "details" | "sla" | "attachments" | "time" | "call-requests";

const TAB_DEFS: Array<{ id: CaseTabId; label: string }> = [
  { id: "activities", label: "Activities" },
  { id: "details", label: "Details" },
  { id: "sla", label: "SLAs" },
  { id: "attachments", label: "Attachments" },
  { id: "time", label: "Time tracking" },
  { id: "call-requests", label: "Call requests" },
];

export default function CaseDetailPage() {
  const { id } = useParams<{ id: string }>();

  return (
    <Stack gap={2}>
      <Typography variant="h6">Case Details</Typography>

      <CaseDetailErrorBoundary>
        <Suspense fallback={<CaseDetailSkeleton />}>
          <CaseDetailContent id={id ?? ""} />
        </Suspense>
      </CaseDetailErrorBoundary>
    </Stack>
  );
}

function CaseDetailContent({ id }: { id: string }) {
  const queryClient = useQueryClient();
  const { data: caseDetail } = useSuspenseQuery(cases.get(id));
  const { data: comments } = useSuspenseQuery(cases.comments(id));
  const { data: currentUserId } = useSuspenseQuery(currentUser.id());
  const currentUserEmail = useUserStore((s) => s.user?.email);

  const [isMutating, setIsMutating] = useState(false);
  const [mutationError, setMutationError] = useState<string | null>(null);
  const [resolutionTarget, setResolutionTarget] = useState<"closed" | "solution_proposed" | null>(null);
  const [severityOpen, setSeverityOpen] = useState(false);
  const [isPostingComment, setIsPostingComment] = useState(false);
  const [activeTab, setActiveTab] = useState<CaseTabId>("activities");
  const [pauseConflict, setPauseConflict] = useState<MyOngoingCase[] | null>(null);

  const invalidateCase = (): void => {
    void queryClient.invalidateQueries({ queryKey: ["case", id] });
    void queryClient.invalidateQueries({ queryKey: ["cases"] });
  };

  const handleTransition = (target: CaseState): void => {
    setIsMutating(true);
    setMutationError(null);
    cases
      .patch(id, { state: target })
      .then(invalidateCase)
      .catch(() => setMutationError("Could not update the case. Please try again."))
      .finally(() => setIsMutating(false));
  };

  // The backend allows only one `ongoing` case per engineer — PATCH { workState: "ongoing" }
  // 409s otherwise ("the assigned engineer already has an Ongoing case: <number>"). Check first
  // (mirrors the webapp's findMyOngoingCases) and open the pause-conflict dialog instead of
  // hitting that 409. Shared by both handleAssignAndStart and handleToggleWorkState, same as the
  // webapp's own startWork/resolveOngoingConflict split.
  const attemptGoOngoing = (): Promise<void> => {
    return cases.findMyOngoingCases(id, currentUserId, currentUserEmail?.toLowerCase() ?? null).then((others) => {
      if (others.length > 0) {
        setPauseConflict(others);
        return;
      }
      return cases.patch(id, { workState: "ongoing" }).then(invalidateCase);
    });
  };

  // Claiming an unassigned/someone-else's case needs `assigneeEmail` PATCHed before the state
  // move — the backend only accepts one field per PATCH (see CasePatchPayloadDto), so this is two
  // sequential calls, mirroring the webapp's onAction assign_to_me handling. Then attempts to mark
  // the case ongoing too (matching the webapp's startWork — assigning without also going ongoing
  // would leave comments blocked until a separate "Resume work" tap).
  const handleAssignAndStart = (): void => {
    if (!currentUserEmail) {
      setMutationError("Could not assign the case to you: no signed-in email found.");
      return;
    }
    setIsMutating(true);
    setMutationError(null);
    cases
      .patch(id, { assigneeEmail: currentUserEmail })
      .then(() => cases.patch(id, { state: "work_in_progress" }))
      .then(invalidateCase)
      .then(() => attemptGoOngoing())
      .catch(() => setMutationError("Could not assign the case to you. Please try again."))
      .finally(() => setIsMutating(false));
  };

  // Toggles the work sub-state — the only way to reach `ongoing`, which the comment gate
  // (CommentComposer / utils/caseWorkState.ts) requires for public comments. Only offered to the
  // case's own assignee while `work_in_progress` (see CaseActionBar's canToggleWorkState).
  const handleToggleWorkState = (): void => {
    setIsMutating(true);
    setMutationError(null);
    const next =
      caseDetail.workState === "ongoing"
        ? cases.patch(id, { workState: "paused" }).then(invalidateCase)
        : attemptGoOngoing();
    next
      .catch(() => setMutationError("Could not update the work state. Please try again."))
      .finally(() => setIsMutating(false));
  };

  const handleConfirmPauseConflict = (): void => {
    if (!pauseConflict) return;
    setIsMutating(true);
    setMutationError(null);
    Promise.all(pauseConflict.map((other) => cases.patch(other.id, { workState: "paused" })))
      .then(() => cases.patch(id, { workState: "ongoing" }))
      .then(() => {
        setPauseConflict(null);
        invalidateCase();
      })
      .catch(() => setMutationError("Could not update the work states. Please try again."))
      .finally(() => setIsMutating(false));
  };

  const handleDeclinePauseConflict = (): void => setPauseConflict(null);

  const handleResolutionSubmit = (fields: {
    resolutionCode: CaseResolutionCode;
    cause: CaseCause;
    closeNotes: string;
  }): void => {
    if (!resolutionTarget) return;
    setIsMutating(true);
    setMutationError(null);
    cases
      .patch(id, { state: resolutionTarget, ...fields })
      .then(() => {
        setResolutionTarget(null);
        invalidateCase();
      })
      .catch(() => setMutationError("Could not update the case. Please try again."))
      .finally(() => setIsMutating(false));
  };

  const handleSeveritySubmit = (next: CaseSeverity): void => {
    setIsMutating(true);
    setMutationError(null);
    cases
      .patch(id, { severity: next })
      .then(() => {
        setSeverityOpen(false);
        invalidateCase();
      })
      .catch(() => setMutationError("Could not change the severity. Please try again."))
      .finally(() => setIsMutating(false));
  };

  // Text and inline attachments upload as separate requests (the backend's comment payload has no
  // file field — see CaseCommentCreatePayloadDto), mirroring NewCasePage's create-then-attach
  // pattern. An empty comment with attachments still sends: the case gets the files with no
  // accompanying comment, rather than forcing placeholder text just to satisfy the text field.
  //
  // Resolves false only when the comment text itself fails to post (e.g. the 409 when a public
  // comment is posted outside work_in_progress+ongoing) — the composer keeps what the user typed
  // in that case instead of discarding it. A partial attachment failure still resolves true, since
  // the text (if any) already landed; NewCasePage tolerates the same partial failure on create.
  const handleCommentSubmit = (fields: {
    type: CaseCommentType;
    content: string;
    attachments: PendingAttachment[];
  }): Promise<boolean> => {
    setIsPostingComment(true);
    setMutationError(null);

    const postText = fields.content
      ? cases.postComment(id, { type: fields.type, content: fields.content })
      : Promise.resolve(null);

    return postText
      .then(() =>
        Promise.allSettled(
          fields.attachments.map((attachment) =>
            attachmentsService.create({
              referenceId: id,
              referenceType: "case",
              name: attachment.name,
              type: attachment.type,
              file: attachment.file,
            }),
          ),
        ),
      )
      .then((results) => {
        const failedCount = results.filter((r) => r.status === "rejected").length;
        if (failedCount > 0) {
          Logger.warn(`${failedCount} attachment(s) failed to upload to case ${id}`);
          setMutationError(
            failedCount === fields.attachments.length
              ? "Could not upload the attachment(s). Please try again."
              : `${failedCount} attachment(s) failed to upload.`,
          );
        }
        void queryClient.invalidateQueries({ queryKey: ["case", id, "comments"] });
        return true;
      })
      .catch((error) => {
        setMutationError(toApiError(error, "Could not post the comment. Please try again.").message);
        return false;
      })
      .finally(() => setIsPostingComment(false));
  };

  return (
    <Stack gap={2}>
      <CaseSummarySection
        caseDetail={caseDetail}
        currentUserId={currentUserId}
        isMutating={isMutating}
        mutationError={mutationError}
        onTransition={handleTransition}
        onAssignAndStart={handleAssignAndStart}
        onToggleWorkState={handleToggleWorkState}
        onNeedsResolution={setResolutionTarget}
        onChangeSeverity={() => setSeverityOpen(true)}
      />

      <Tabs value={activeTab} variant="scrollable" onChange={(_, value: CaseTabId) => setActiveTab(value)}>
        {TAB_DEFS.map((tab) => (
          <Tab key={tab.id} value={tab.id} label={tab.label} />
        ))}
      </Tabs>

      {activeTab === "activities" && (
        <CaseCommentsSection
          comments={comments}
          caseState={caseDetail.state}
          workState={caseDetail.workState}
          isPostingComment={isPostingComment}
          onSubmitComment={handleCommentSubmit}
        />
      )}

      {activeTab === "details" && (
        <Stack gap={2}>
          {caseDetail.description && (
            <SectionCard title="Description">
              <Typography variant="body2" color="text.primary" sx={{ whiteSpace: "pre-wrap" }}>
                {caseDetail.description}
              </Typography>
            </SectionCard>
          )}
          <SectionCard title="Overview">
            <CaseMetadataSection caseDetail={caseDetail} />
          </SectionCard>
        </Stack>
      )}

      {activeTab === "sla" && <SlaTab caseId={id} />}

      {activeTab === "attachments" && <AttachmentsTab caseId={id} />}

      {activeTab === "time" && <TimeTrackingTab caseNumber={caseDetail.number} />}

      {activeTab === "call-requests" && <CallRequestsTab caseId={id} />}

      {resolutionTarget && (
        <ResolutionDialog
          kind={resolutionTarget}
          isSubmitting={isMutating}
          onClose={() => setResolutionTarget(null)}
          onSubmit={handleResolutionSubmit}
        />
      )}

      {severityOpen && caseDetail.severity && (
        <ChangeSeverityDialog
          currentSeverity={caseDetail.severity}
          isSubmitting={isMutating}
          onClose={() => setSeverityOpen(false)}
          onSubmit={handleSeveritySubmit}
        />
      )}

      {pauseConflict && (
        <PauseConflictDialog
          otherCases={pauseConflict}
          isSubmitting={isMutating}
          onConfirm={handleConfirmPauseConflict}
          onDecline={handleDeclinePauseConflict}
        />
      )}
    </Stack>
  );
}

function CaseSummarySection({
  caseDetail,
  currentUserId,
  isMutating,
  mutationError,
  onTransition,
  onAssignAndStart,
  onToggleWorkState,
  onNeedsResolution,
  onChangeSeverity,
}: {
  caseDetail: CaseDetail;
  currentUserId: string | null;
  isMutating: boolean;
  mutationError: string | null;
  onTransition: (target: CaseState) => void;
  onAssignAndStart: () => void;
  onToggleWorkState: () => void;
  onNeedsResolution: (target: "closed" | "solution_proposed") => void;
  onChangeSeverity: () => void;
}) {
  const { icon: Icon, color } = TYPE_CONFIG[caseDetail.type ?? "case"] ?? TYPE_CONFIG.case;
  const canChangeSeverity = caseDetail.severity && caseDetail.state !== "closed";
  const canToggleWorkState =
    caseDetail.state === "work_in_progress" && caseDetail.assignedEngineer?.id === currentUserId;

  return (
    <Stack gap={1.5}>
      <Stack gap={0.5}>
        <Stack direction="row" alignItems="center" gap={1}>
          <Icon size={pxToRem(18)} color={color} />
          <Typography variant="subtitle2" color="text.secondary">
            {caseDetail.number}
            {caseDetail.wso2Id ? ` · ${caseDetail.wso2Id}` : ""}
          </Typography>
        </Stack>

        <Typography variant="h6">{caseDetail.subject}</Typography>

        <Stack direction="row" gap={1} flexWrap="wrap" alignItems="center">
          <StatusChip state={caseDetail.state} />
          {/* Only "case"-type items carry a severity; service requests/security reports/etc. don't. */}
          {caseDetail.severity && <SeverityChip severity={caseDetail.severity} />}
        </Stack>
      </Stack>

      {/* Actions live in their own row, visually separated from the identity block above so the
       * primary "what can I do" affordance isn't read as part of the case's own metadata. */}
      {(caseDetail.nextStates.length > 0 || canChangeSeverity || canToggleWorkState) && (
        <Stack direction="row" gap={1} flexWrap="wrap" alignItems="center" sx={{ pt: 0.5 }}>
          <CaseActionBar
            caseDetail={caseDetail}
            currentUserId={currentUserId}
            isPending={isMutating}
            onTransition={onTransition}
            onAssignAndStart={onAssignAndStart}
            onToggleWorkState={onToggleWorkState}
            onNeedsResolution={onNeedsResolution}
          />
          {/* Closed is read-only, same rule the webapp applies to comments/attachments/severity. */}
          {canChangeSeverity && (
            <Button size="small" variant="outlined" disabled={isMutating} onClick={onChangeSeverity}>
              Change severity
            </Button>
          )}
        </Stack>
      )}

      {mutationError && (
        <Typography variant="caption" color="error.main">
          {mutationError}
        </Typography>
      )}
    </Stack>
  );
}

function CaseMetadataSection({ caseDetail }: { caseDetail: CaseDetail }) {
  return (
    <Stack gap={1}>
      <DetailRow label="Project" value={caseDetail.project?.name} />
      <DetailRow label="Deployment" value={caseDetail.deployment?.name} />
      <DetailRow label="Product" value={caseDetail.product?.name} />
      <DetailRow label="Account" value={caseDetail.account?.name} />
      <DetailRow label="Assigned Engineer" value={caseDetail.assignedEngineer?.name} />
      <DetailRow label="Created By" value={caseDetail.createdBy?.displayName || caseDetail.createdBy?.email} />
      <DetailRow label="Created On" value={formatDate(caseDetail.createdOn)} />
      <DetailRow label="Updated On" value={formatDate(caseDetail.updatedOn)} />
      {caseDetail.closedOn && <DetailRow label="Closed On" value={formatDate(caseDetail.closedOn)} />}
    </Stack>
  );
}

function DetailRow({ label, value }: { label: string; value?: string | null }) {
  if (!value) return null;

  return (
    <Stack direction="row" justifyContent="space-between" gap={2}>
      <Typography variant="body2" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body2" color="text.primary" textAlign="right">
        {value}
      </Typography>
    </Stack>
  );
}

function CaseCommentsSection({
  comments,
  caseState,
  workState,
  isPostingComment,
  onSubmitComment,
}: {
  comments: Comment[];
  caseState: CaseState;
  workState: CaseWorkState;
  isPostingComment: boolean;
  onSubmitComment: (fields: {
    type: CaseCommentType;
    content: string;
    attachments: PendingAttachment[];
  }) => Promise<boolean>;
}) {
  return (
    <Stack gap={1.5}>
      <CommentComposer
        caseState={caseState}
        workState={workState}
        isSubmitting={isPostingComment}
        onSubmit={onSubmitComment}
      />

      {comments.length > 0 && <Divider />}

      {comments.length === 0 && (
        <Typography variant="body2" color="text.secondary">
          No comments yet.
        </Typography>
      )}

      {comments.map((comment) => (
        <Stack key={comment.id} gap={0.5} sx={{ p: 1.5, border: "1px solid", borderColor: "divider", borderRadius: 1 }}>
          <Stack direction="row" justifyContent="space-between" gap={2}>
            <Typography variant="subtitle2" color="text.secondary" noWrap sx={{ minWidth: 0 }}>
              {comment.createdBy}
            </Typography>
            <Typography variant="subtitle2" color="text.secondary" noWrap sx={{ flexShrink: 0 }}>
              {formatDate(comment.createdOn)}
            </Typography>
          </Stack>
          <CommentBody content={comment.content} />
        </Stack>
      ))}
    </Stack>
  );
}

function CaseDetailSkeleton() {
  return (
    <Stack gap={2}>
      <Skeleton variant="text" width="40%" height={24} />
      <Skeleton variant="text" width="80%" height={32} />
      <Skeleton variant="rounded" height={100} />
      <Skeleton variant="rounded" height={150} />
    </Stack>
  );
}

function CaseDetailErrorBoundary({ children }: { children: ReactNode }) {
  const { reset } = useQueryErrorResetBoundary();

  return (
    <ErrorBoundary
      fallback={(_error, resetErrorBoundary) => (
        <ErrorState
          onRetry={() => {
            reset();
            resetErrorBoundary();
          }}
        />
      )}
    >
      {children}
    </ErrorBoundary>
  );
}
