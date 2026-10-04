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

import { Suspense, useRef, useState, type ReactNode } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Card, Divider, Grid, Skeleton, Stack, Tab, Tabs, Typography, pxToRem } from "@wso2/oxygen-ui";
import {
  Building2,
  CheckCircle,
  Clock,
  Folder,
  Package,
  PenLine,
  RefreshCw,
  Rocket,
  UserCog,
  type LucideIcon,
} from "@wso2/oxygen-ui-icons-react";
import { useQueryClient, useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { cases, parseOngoingConflictCaseNumber, type MyOngoingCase } from "@src/services/cases";
import { currentUser } from "@src/services/currentUser";
import { attachments as attachmentsService } from "@src/services/attachments";
import { timecards } from "@src/services/timecards";
import { useUserStore } from "@src/store/user";
import { useCaseActivityStream } from "@utils/useCaseActivityStream";
import type {
  CaseCause,
  CaseCommentType,
  CaseDetail,
  CaseResolutionCode,
  CaseSeverity,
  CaseState,
  CaseWorkState,
  Comment,
  CreateTimeCardInput,
} from "@src/types";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { CopyIconButton } from "@components/common/CopyIconButton";
import { SeverityChip, StatusChip } from "@components/support/Chips";
import { ErrorState } from "@components/support/ErrorState";
import { TYPE_CONFIG } from "@components/support/config";
import { CaseActionBar } from "@components/case-detail/CaseActionBar";
import { CaseMoreMenu } from "@components/case-detail/CaseMoreMenu";
import { ChangeSeverityDialog } from "@components/case-detail/ChangeSeverityDialog";
import { LinkCaseDialog, type CaseLinkType } from "@components/case-detail/LinkCaseDialog";
import { LinkedItemsTab } from "@components/case-detail/LinkedItemsTab";
import { LogTimeCardDialog } from "@components/case-detail/LogTimeCardDialog";
import { ResolutionDialog } from "@components/case-detail/ResolutionDialog";
import { CommentComposer } from "@components/case-detail/CommentComposer";
import { CaseActivitiesTab } from "@components/case-detail/CaseActivityFeed";
import { PauseConflictDialog } from "@components/case-detail/PauseConflictDialog";
import { SlaTab } from "@components/case-detail/SlaTab";
import { AttachmentsTab } from "@components/case-detail/AttachmentsTab";
import { CallRequestsTab } from "@components/case-detail/CallRequestsTab";
import { TimeTrackingTab } from "@components/case-detail/TimeTrackingTab";
import type { CreateServiceRequestFromCaseNavState } from "@pages/NewServiceRequestPage";
import { formatDate } from "@utils/dateTime";
import { Logger } from "@utils/logger";
import { toApiError } from "@utils/ApiError";
import type { PendingAttachment } from "@utils/attachments";

type CaseTabId = "activities" | "details" | "related" | "sla" | "attachments" | "time" | "call-requests";

const TAB_DEFS: Array<{ id: CaseTabId; label: string }> = [
  { id: "activities", label: "Activities" },
  { id: "details", label: "Details" },
  { id: "related", label: "Linked Items" },
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
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data: caseDetail } = useSuspenseQuery(cases.get(id));
  const { data: comments } = useSuspenseQuery(cases.comments(id));
  const { data: currentUserId } = useSuspenseQuery(currentUser.id());
  const currentUserEmail = useUserStore((s) => s.user?.email);
  // Live updates: invalidates the case's comments/activities queries
  // whenever another viewer adds a comment or the case's status changes, so
  // this page doesn't rely solely on their own staleTime/a manual refresh.
  useCaseActivityStream(id);

  // Announcements are cases of type "announcement" — read-only, and the
  // SLA/time-tracking/call-request tabs don't apply. Mirrors the webapp's
  // CsmCaseDetailPage, minus its route-based check: this page is always
  // reached via /cases/:id regardless of case type, so the loaded case's own
  // type is the only signal needed.
  const isAnnouncement = caseDetail.type === "announcement";

  const [isMutating, setIsMutating] = useState(false);
  const [mutationError, setMutationError] = useState<string | null>(null);
  const [resolutionTarget, setResolutionTarget] = useState<"closed" | "solution_proposed" | null>(null);
  const [isPostingComment, setIsPostingComment] = useState(false);
  const [activeTab, setActiveTab] = useState<CaseTabId>("activities");
  const [pauseConflict, setPauseConflict] = useState<MyOngoingCase[] | null>(null);
  const [severityDialogOpen, setSeverityDialogOpen] = useState(false);
  const [logTimeOpen, setLogTimeOpen] = useState(false);
  const [isLoggingTime, setIsLoggingTime] = useState(false);
  const [logTimeError, setLogTimeError] = useState<string | null>(null);
  const [linkCaseOpen, setLinkCaseOpen] = useState(false);
  const [isLinkingCase, setIsLinkingCase] = useState(false);

  const ANNOUNCEMENT_HIDDEN_TABS: CaseTabId[] = ["related", "sla", "time", "call-requests"];
  const visibleTabDefs = isAnnouncement
    ? TAB_DEFS.filter((tab) => !ANNOUNCEMENT_HIDDEN_TABS.includes(tab.id))
    : TAB_DEFS;
  // If a case turns out to be an announcement (only knowable once caseDetail
  // loads) while a tab hidden for announcements is active, fall back to
  // Activities rather than rendering a tab no longer in the visible list.
  const effectiveTab = visibleTabDefs.some((tab) => tab.id === activeTab) ? activeTab : "activities";

  const invalidateCase = (): void => {
    void queryClient.invalidateQueries({ queryKey: ["case", id] });
    void queryClient.invalidateQueries({ queryKey: ["cases"] });
    // Home's "Assigned to me"/composition widgets key off a disjoint ["dashboard", ...] prefix —
    // without this, a state/assignment/workState change made here (e.g. closing or picking up a
    // case) would leave those widgets showing stale data until their own staleTime lapsed.
    void queryClient.invalidateQueries({ queryKey: ["dashboard"] });
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
  // 409s otherwise ("[SERVICENOW_ERROR] ...already has an Ongoing case: <number>"). Shared by
  // handleAssignAndStart and handleToggleWorkState, same as the webapp's own
  // startWork/resolveOngoingConflict split — with one important difference from the first version
  // of this: the conflict check runs BEFORE any PATCH at all, and nothing is applied — not the
  // assignee change, not the state transition, not the workState — until either the check comes
  // back clear or the user explicitly confirms pausing the other case. Cancel must leave the case
  // completely untouched, not "assigned and moved to work_in_progress but not ongoing".
  //
  // pendingOngoingAction holds the full sequence to run once a detected conflict is resolved
  // (paused). It's a ref, not state, since it's never rendered — only read/written imperatively
  // around the dialog's confirm/decline.
  const pendingOngoingAction = useRef<(() => Promise<void>) | null>(null);

  // Pure check — no mutation. Best-effort: this backend errors on some filter combinations (see
  // the note in services/timecards.ts), so a failed search returns [] rather than blocking the
  // caller — the reactive 409 catch in runOngoingAction is the backstop if this misses a conflict.
  const checkOngoingConflict = async (): Promise<MyOngoingCase[]> => {
    try {
      const others = await cases.findMyOngoingCases(id, currentUserId, currentUserEmail?.toLowerCase() ?? null);
      Logger.info("[ongoing-conflict] proactive search result", { others });
      return others;
    } catch (searchError) {
      Logger.warn("[ongoing-conflict] proactive search failed; relying on the PATCH 409 instead", searchError);
      return [];
    }
  };

  // Runs `action` (a PATCH sequence whose last step marks this case ongoing). If ServiceNow's own
  // 409 rejects that last step anyway (the proactive check missed a conflict), ServiceNow's
  // message already names the real conflicting case by number — resolve it and open the same
  // pause dialog with `action` stored, so confirming re-runs the whole thing rather than
  // dead-ending on a raw error the user can't act on.
  const runOngoingAction = async (action: () => Promise<void>): Promise<void> => {
    try {
      await action();
    } catch (error) {
      const conflictNumber = parseOngoingConflictCaseNumber(toApiError(error, "").message);
      if (!conflictNumber) throw error;
      const conflictCase = await cases.findCaseByNumber(conflictNumber);
      pendingOngoingAction.current = action;
      setPauseConflict([conflictCase]);
    }
  };

  // Checks for a conflict first; only calls runOngoingAction (i.e. only touches the case at all)
  // once the check is clear. If a conflict is already known, `action` is stashed for the dialog's
  // confirm instead of running anything yet.
  const goOngoingWithConflictGuard = (action: () => Promise<void>): Promise<void> =>
    checkOngoingConflict().then((others) => {
      if (others.length > 0) {
        pendingOngoingAction.current = action;
        setPauseConflict(others);
        return;
      }
      return runOngoingAction(action);
    });

  // Any move into work_in_progress — whether the case is already assigned to the caller
  // ("Start progress") or not ("Assign to me") — mirrors the webapp's onAction: `targetState ===
  // "work_in_progress"` always routes through startWork() unconditionally, never a bare state
  // PATCH, because starting is also the point where the case should go `ongoing`. Claiming an
  // unassigned/someone else's case needs `assigneeEmail` PATCHed first — the backend only accepts
  // one field per PATCH (see CasePatchPayloadDto) — so that step is skipped when already the
  // caller's. The whole sequence is gated behind the ongoing-conflict check (see above) so nothing
  // runs — not even the assign — until it's actually safe to proceed.
  const handleAssignAndStart = (): void => {
    const alreadyMine = caseDetail.assignedEngineer?.id === currentUserId;
    if (!alreadyMine && !currentUserEmail) {
      setMutationError("Could not assign the case to you: no signed-in email found.");
      return;
    }
    // The backend has no single atomic "assign + start + go ongoing" operation (one field per
    // PATCH — see CasePatchPayloadDto), so this is three sequential PATCHes. The proactive
    // conflict check (goOngoingWithConflictGuard) keeps ANY of them from running until it's
    // clear — but a later step can still fail (the reactive 409 in runOngoingAction only fires on
    // the last step, and any step can fail on a network or backend error), by which point the
    // earlier PATCHes have already landed. Undo exactly the steps that succeeded, newest first,
    // before re-throwing, so a failure or the pause-conflict dialog's Cancel leaves the case
    // untouched, matching the invariant documented on pendingOngoingAction above. Each undo is
    // guarded on its own: one failing must neither skip the next nor replace the original error.
    // (A typed, server-side conflict endpoint would make this client-side compensation
    // unnecessary; until then it is best-effort.)
    const originalAssigneeEmail = caseDetail.assignedEngineer?.email ?? null;
    const originalState = caseDetail.state;
    const action = async (): Promise<void> => {
      const undo: { label: string; run: () => Promise<unknown> }[] = [];
      try {
        if (!alreadyMine) {
          await cases.patch(id, { assigneeEmail: currentUserEmail as string });
          // A null email restores an originally-unassigned case to unassigned.
          undo.push({ label: "assignee", run: () => cases.patch(id, { assigneeEmail: originalAssigneeEmail }) });
        }
        await cases.patch(id, { state: "work_in_progress" });
        undo.push({ label: "state", run: () => cases.patch(id, { state: originalState }) });
        invalidateCase();
        await cases.patch(id, { workState: "ongoing" });
      } catch (error) {
        for (const step of undo.reverse()) {
          try {
            await step.run();
          } catch (undoError) {
            Logger.warn(`[start-work] could not restore the case ${step.label}`, undoError);
          }
        }
        invalidateCase();
        throw error;
      }
      invalidateCase();
    };
    setIsMutating(true);
    setMutationError(null);
    goOngoingWithConflictGuard(action)
      .catch((error) => setMutationError(toApiError(error, "Could not start work on the case.").message))
      .finally(() => setIsMutating(false));
  };

  const handleConfirmPauseConflict = (): void => {
    if (!pauseConflict) return;
    // A null id means that case's UUID couldn't be resolved (the search that would find it has
    // been unreliable) — it can't be auto-paused. Running the pending action afterward would just
    // hit the same 409 again since the real conflict is still active, so block the confirm
    // entirely and tell the user to pause it themselves instead of silently failing.
    const unresolved = pauseConflict.filter((c) => !c.id);
    if (unresolved.length > 0) {
      setMutationError(
        `Couldn't automatically look up ${unresolved.map((c) => c.label).join(", ")}. Please open it from Support, ` +
          `pause the work there, then try again.`,
      );
      return;
    }
    const action = pendingOngoingAction.current;
    if (!action) {
      setPauseConflict(null);
      return;
    }
    setIsMutating(true);
    setMutationError(null);
    // Pause each other ongoing case, then run whatever was pending (mark ongoing, or the full
    // assign+start+ongoing sequence) — same sequential order as the webapp's onConfirmStartWork.
    Promise.all(pauseConflict.map((other) => cases.patch(other.id as string, { workState: "paused" })))
      .then(() => action())
      .then(() => {
        setPauseConflict(null);
        pendingOngoingAction.current = null;
      })
      .catch((error) => setMutationError(toApiError(error, "Could not update the work states.").message))
      .finally(() => setIsMutating(false));
  };

  // Cancel: nothing was ever applied (see goOngoingWithConflictGuard) — just close the dialog and
  // drop the pending action. The case is left exactly as it was before the action was tapped.
  const handleDeclinePauseConflict = (): void => {
    setPauseConflict(null);
    pendingOngoingAction.current = null;
  };

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
        setSeverityDialogOpen(false);
        invalidateCase();
      })
      .catch(() => setMutationError("Could not change the severity. Please try again."))
      .finally(() => setIsMutating(false));
  };

  const handleLogTimeSubmit = (input: CreateTimeCardInput): void => {
    setIsLoggingTime(true);
    setLogTimeError(null);
    timecards
      .create(input)
      .then(() => {
        setLogTimeOpen(false);
        // Root key — one invalidate refreshes every time-cards view (My sheets, All, Approvals,
        // and this case's own list), same as TimeTrackingTab's own log-time flow.
        void queryClient.invalidateQueries({ queryKey: ["timecards"] });
      })
      .catch(() => setLogTimeError("Could not log time. Please try again."))
      .finally(() => setIsLoggingTime(false));
  };

  const handleLinkCase = (targetCaseId: string, linkType: CaseLinkType): void => {
    setIsLinkingCase(true);
    setMutationError(null);
    cases
      .patch(id, linkType === "parent" ? { parentId: targetCaseId } : { relatedCaseId: targetCaseId })
      .then(() => {
        setLinkCaseOpen(false);
        invalidateCase();
      })
      .catch(() => setMutationError("Could not link the case. Please try again."))
      .finally(() => setIsLinkingCase(false));
  };

  // Pre-fills the create-service-request form and links the new SR back to this case in one step
  // (see CreateServiceRequestFromCaseNavState) — no separate create-then-link round trip.
  const handleCreateServiceRequest = (): void => {
    const navState: CreateServiceRequestFromCaseNavState = {
      projectId: caseDetail.project.id,
      relatedCaseId: caseDetail.id,
      relatedCaseNumber: caseDetail.number,
      deploymentId: caseDetail.deployment?.id,
      deployedProductId: caseDetail.deployedProduct?.id,
    };
    navigate("/operations/service-requests/new", { state: navState });
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
        const allAttachmentsFailed = fields.attachments.length > 0 && failedCount === fields.attachments.length;
        if (failedCount > 0) {
          Logger.warn(`${failedCount} attachment(s) failed to upload to case ${id}`);
          setMutationError(
            allAttachmentsFailed
              ? "Could not upload the attachment(s). Please try again."
              : `${failedCount} attachment(s) failed to upload.`,
          );
        }
        void queryClient.invalidateQueries({ queryKey: ["case", id, "comments"] });
        // A comment (like a state transition, see invalidateCase above) bumps the case's own
        // updatedOn server-side — without this, neither this page's own "Updated On" nor the
        // cases list's sort order pick up the change until something else happens to invalidate
        // them.
        invalidateCase();
        // An attachment-only submission (no text) where every upload failed posted nothing at
        // all — resolve false so the composer keeps the picked files for retry instead of
        // clearing them after showing the error. A submission with text still succeeds here
        // since the text itself already landed.
        return !(allAttachmentsFailed && !fields.content);
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
        isAnnouncement={isAnnouncement}
        isMutating={isMutating}
        mutationError={mutationError}
        onTransition={handleTransition}
        onAssignAndStart={handleAssignAndStart}
        onNeedsResolution={setResolutionTarget}
        onOpenChangeSeverity={() => setSeverityDialogOpen(true)}
        onOpenLogTime={() => setLogTimeOpen(true)}
      />

      <Tabs value={effectiveTab} variant="scrollable" onChange={(_, value: CaseTabId) => setActiveTab(value)}>
        {visibleTabDefs.map((tab) => (
          <Tab key={tab.id} value={tab.id} label={tab.label} />
        ))}
      </Tabs>

      {effectiveTab === "activities" && (
        <CaseCommentsSection
          caseId={id}
          comments={comments}
          caseState={caseDetail.state}
          workState={caseDetail.workState}
          isPostingComment={isPostingComment}
          isAnnouncement={isAnnouncement}
          onSubmitComment={handleCommentSubmit}
        />
      )}

      {effectiveTab === "details" && (
        <Stack gap={2}>
          <CaseMetadataSection caseDetail={caseDetail} />
        </Stack>
      )}

      {effectiveTab === "related" && (
        <LinkedItemsTab
          caseId={id}
          caseDetail={caseDetail}
          isClosed={caseDetail.state === "closed"}
          onLinkCase={() => setLinkCaseOpen(true)}
          onCreateServiceRequest={handleCreateServiceRequest}
        />
      )}

      {effectiveTab === "sla" && <SlaTab caseId={id} />}

      {effectiveTab === "attachments" && <AttachmentsTab caseId={id} />}

      {effectiveTab === "time" && (
        <TimeTrackingTab
          caseId={id}
          caseNumber={caseDetail.number}
          caseSeverity={caseDetail.severity}
          projectId={caseDetail.project?.id ?? ""}
          projectName={caseDetail.project?.name ?? ""}
        />
      )}

      {activeTab === "call-requests" && <CallRequestsTab caseId={id} severity={caseDetail.severity} />}

      {resolutionTarget && (
        <ResolutionDialog
          kind={resolutionTarget}
          isSubmitting={isMutating}
          onClose={() => setResolutionTarget(null)}
          onSubmit={handleResolutionSubmit}
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
      {severityDialogOpen && caseDetail.severity && (
        <ChangeSeverityDialog
          currentSeverity={caseDetail.severity}
          isSubmitting={isMutating}
          onClose={() => setSeverityDialogOpen(false)}
          onSubmit={handleSeveritySubmit}
        />
      )}
      {logTimeOpen && (
        <LogTimeCardDialog
          caseId={id}
          caseNumber={caseDetail.number}
          caseSeverity={caseDetail.severity}
          projectId={caseDetail.project?.id ?? ""}
          projectName={caseDetail.project?.name ?? ""}
          isSubmitting={isLoggingTime}
          error={logTimeError}
          onClose={() => {
            if (!isLoggingTime) {
              setLogTimeOpen(false);
              setLogTimeError(null);
            }
          }}
          onSubmit={handleLogTimeSubmit}
        />
      )}
      {linkCaseOpen && (
        <LinkCaseDialog
          currentCaseId={id}
          isLinking={isLinkingCase}
          onClose={() => setLinkCaseOpen(false)}
          onLink={handleLinkCase}
        />
      )}
    </Stack>
  );
}

function CaseSummarySection({
  caseDetail,
  currentUserId,
  isAnnouncement,
  isMutating,
  mutationError,
  onTransition,
  onAssignAndStart,
  onNeedsResolution,
  onOpenChangeSeverity,
  onOpenLogTime,
}: {
  caseDetail: CaseDetail;
  currentUserId: string | null;
  isAnnouncement: boolean;
  isMutating: boolean;
  mutationError: string | null;
  onTransition: (target: CaseState) => void;
  onAssignAndStart: () => void;
  onNeedsResolution: (target: "closed" | "solution_proposed") => void;
  onOpenChangeSeverity: () => void;
  onOpenLogTime: () => void;
}) {
  const { icon: Icon, color } = TYPE_CONFIG[caseDetail.type ?? "case"] ?? TYPE_CONFIG.case;

  return (
    <Stack gap={1.5}>
      <Stack gap={0.5}>
        <Stack direction="row" alignItems="center" gap={0.5}>
          <Icon size={pxToRem(18)} color={color} />
          <Typography variant="subtitle2" color="text.secondary">
            {/* wso2Id then number, "|"-separated — matches the customer-portal microapp's
                OverlineSlot/ItemCard convention (ids={[internalId, number]}). Copy button below
                copies only the number (not wso2Id) — that's the value Support's search box
                actually matches on, so copying it lets you paste straight into a case search. */}
            {caseDetail.wso2Id && (
              <>
                {caseDetail.wso2Id}
                <span style={{ opacity: 0.5, margin: "0 4px" }}>|</span>
              </>
            )}
            {caseDetail.number}
          </Typography>
          <CopyIconButton value={caseDetail.number} aria-label="Copy case ID" />
        </Stack>

        <Typography variant="h6">{caseDetail.subject}</Typography>

        {/* Announcements are read-only and carry no meaningful state/severity of their own. */}
        {!isAnnouncement && (
          <Stack direction="row" gap={1} flexWrap="wrap" alignItems="center">
            <StatusChip state={caseDetail.state} />
            {/* Only "case"-type items carry a severity; service requests/security reports/etc. don't. */}
            {caseDetail.severity && <SeverityChip severity={caseDetail.severity} />}
          </Stack>
        )}
      </Stack>

      {/* Actions live in their own row, visually separated from the identity block above so the
       * primary "what can I do" affordance isn't read as part of the case's own metadata. The
       * More menu always renders alongside a lifecycle button (mirrors the webapp's action bar) —
       * it has case-independent items (e.g. Log time) that apply regardless of `nextStates`. */}
      {!isAnnouncement && (
        <Stack
          direction="row"
          gap={1}
          flexWrap="nowrap"
          alignItems="center"
          sx={{ pt: 0.5, overflowX: "auto", scrollbarWidth: "none", "&::-webkit-scrollbar": { display: "none" } }}
        >
          <CaseActionBar
            caseDetail={caseDetail}
            currentUserId={currentUserId}
            isPending={isMutating}
            onTransition={onTransition}
            onAssignAndStart={onAssignAndStart}
            onNeedsResolution={onNeedsResolution}
          />
          <CaseMoreMenu
            caseDetail={caseDetail}
            currentUserId={currentUserId}
            onChangeSeverity={onOpenChangeSeverity}
            onLogTime={onOpenLogTime}
          />
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

interface MetaField {
  icon: LucideIcon;
  label: string;
  value?: string | null;
}

function CaseMetadataSection({ caseDetail }: { caseDetail: CaseDetail }) {
  // Related fields share a row (2-up); each group collapses to one full-width row if its
  // other field has no value (e.g. Account missing leaves Project alone on its row).
  // Project and Account each get their own full-width row rather than being paired.
  const groups: MetaField[][] = [
    [{ icon: Folder, label: "Project", value: caseDetail.project?.name }],
    [{ icon: Building2, label: "Account", value: caseDetail.account?.name }],
    [
      { icon: Rocket, label: "Deployment", value: caseDetail.deployment?.name },
      { icon: Package, label: "Product", value: caseDetail.product?.name },
    ],
    [
      { icon: UserCog, label: "Assigned Engineer", value: caseDetail.assignedEngineer?.name },
      {
        icon: PenLine,
        label: "Created By",
        // Falls back to "Unknown" rather than leaving this undefined — this section's groups are
        // filtered to only fields with a truthy value, so an undefined here would silently drop
        // the whole "Created By" row instead of showing that the creator is unknown.
        value: caseDetail.createdBy?.name || caseDetail.createdBy?.email || "Unknown",
      },
    ],
    [
      { icon: Clock, label: "Created On", value: formatDate(caseDetail.createdOn) },
      { icon: RefreshCw, label: "Updated On", value: formatDate(caseDetail.updatedOn) },
    ],
    ...(caseDetail.closedOn
      ? [[{ icon: CheckCircle, label: "Closed On", value: formatDate(caseDetail.closedOn) }]]
      : []),
  ]
    .map((group) => group.filter((field) => field.value))
    .filter((group) => group.length > 0);

  return (
    <Card sx={{ p: 1.75 }}>
      <Stack gap={1.5}>
        {groups.map((group, index) => (
          <Grid container spacing={2} key={index}>
            {group.map((field) => (
              <Grid key={field.label} size={group.length === 2 ? 6 : 12}>
                <MetaFieldItem field={field} />
              </Grid>
            ))}
          </Grid>
        ))}
      </Stack>
    </Card>
  );
}

// Backend-supplied display names (e.g. the assigned engineer's ServiceNow name) sometimes end in
// a trailing marker after a space, like "Hesara Perera (Intern) Ⓦ" — a normal space there lets the
// browser wrap the marker onto its own line, splitting it from the word it qualifies. Swap the
// last space for a non-breaking one so those two trailing tokens always wrap (or don't) together.
function keepLastWordJoined(value: string): string {
  const lastSpaceIndex = value.lastIndexOf(" ");
  const NON_BREAKING_SPACE = "\u00A0";
  return lastSpaceIndex === -1
    ? value
    : `${value.slice(0, lastSpaceIndex)}${NON_BREAKING_SPACE}${value.slice(lastSpaceIndex + 1)}`;
}

function MetaFieldItem({ field }: { field: MetaField }) {
  const Icon = field.icon;
  return (
    <Stack direction="row" alignItems="flex-start" gap={1}>
      {/* Icon color prop is a raw CSS color, not a MUI theme-path string (see TYPE_CONFIG's
       * usage elsewhere in this file) — passing "text.secondary" directly breaks the SVG's
       * stroke color. Set color on the wrapping Stack instead and let the icon's default
       * currentColor pick it up. */}
      <Stack
        alignItems="center"
        justifyContent="center"
        sx={{
          width: 28,
          height: 28,
          flexShrink: 0,
          borderRadius: 1.5,
          bgcolor: "action.hover",
          color: "text.secondary",
        }}
      >
        <Icon size={pxToRem(15)} />
      </Stack>
      <Stack sx={{ minWidth: 0, flex: 1 }} gap={0.125}>
        <Typography variant="caption" color="text.secondary">
          {field.label}
        </Typography>
        <Typography variant="caption" fontWeight={600} sx={{ overflowWrap: "anywhere" }}>
          {field.value ? keepLastWordJoined(field.value) : field.value}
        </Typography>
      </Stack>
    </Stack>
  );
}

function CaseCommentsSection({
  caseId,
  comments,
  caseState,
  workState,
  isPostingComment,
  isAnnouncement,
  onSubmitComment,
}: {
  caseId: string;
  comments: Comment[];
  caseState: CaseState;
  workState: CaseWorkState;
  isPostingComment: boolean;
  isAnnouncement: boolean;
  onSubmitComment: (fields: {
    type: CaseCommentType;
    content: string;
    attachments: PendingAttachment[];
  }) => Promise<boolean>;
}) {
  return (
    <Stack gap={1.5}>
      {/* Announcements are read-only — no commenting. */}
      {!isAnnouncement && (
        <>
          <CommentComposer
            caseState={caseState}
            workState={workState}
            isSubmitting={isPostingComment}
            onSubmit={onSubmitComment}
          />

          <Divider />
        </>
      )}

      <CaseActivitiesTab caseId={caseId} comments={comments} />
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
