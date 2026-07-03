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

import {
  Box,
  Button,
  Card,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  Skeleton,
  Tab,
  Tabs,
  Typography,
} from "@wso2/oxygen-ui";
import {
  Activity,
  ArrowLeft,
  Clock,
  Layers,
  ListChecks,
  MessageSquarePlus,
  Paperclip,
  Phone,
  TriangleAlert,
  X,
} from "@wso2/oxygen-ui-icons-react";
import { useCallback, useEffect, useMemo, useState, type JSX } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { useGetCsmCaseDetail } from "@features/csm-cases/api/useGetCsmCaseDetail";
import {
  usePatchCsmCase,
  usePatchCsmCaseById,
} from "@features/csm-cases/api/usePatchCsmCase";
import {
  useFindMyOngoingCases,
  type MyOngoingCase,
} from "@features/csm-cases/api/useFindMyOngoingCases";
import type { BeCaseState } from "@api/backend/types";
import { beStateFromUi } from "@api/backend/mappers";
import { BackendApiError } from "@api/backend/client";
import {
  useGetCsmCaseComments,
  usePostCsmCaseComment,
} from "@features/csm-cases/api/useCsmCaseComments";
import { useGetCsmConversationMessages } from "@features/csm-cases/api/useCsmConversationMessages";
import {
  useGetCsmCaseAttachments,
  usePostCsmCaseAttachment,
  useDownloadCsmCaseAttachment,
  useDeleteCsmCaseAttachment,
} from "@features/csm-cases/api/useCsmCaseAttachments";
import CsmCaseCommentInput from "@features/csm-cases/components/CsmCaseCommentInput";
import CaseActionBar from "@features/csm-cases/components/CaseActionBar";
import AssignEngineerDialog from "@features/csm-cases/components/AssignEngineerDialog";
import { CreateGithubIssueDialog } from "@features/csm-cases/components/CreateGithubIssueDialog";
import { usePostCaseGithubIssue } from "@features/csm-cases/api/useCsmCaseGithubIssue";
import CaseActivitiesFeed from "@features/csm-cases/components/CaseActivitiesFeed";
import CaseMetaBand from "@features/csm-cases/components/CaseMetaBand";
import {
  AttachmentsWidget,
  AuditTimelineWidget,
  CustomerContextWidget,
  LinkedItemsWidget,
  ProductContextWidget,
  SlaTimelineWidget,
  WatchersWidget,
} from "@features/csm-cases/components/CaseDetailWidgets";
import { CallRequestsWidget } from "@features/csm-cases/components/CallRequestsWidget";
import CaseTimeCardsPanel from "@features/csm-timecards/components/CaseTimeCardsPanel";
import LogTimeCardDialog from "@features/csm-timecards/components/LogTimeCardDialog";
import { usePostTimeCard } from "@features/csm-timecards/api/useTimeCards";
import { caseIdLabel } from "@features/csm-cases/utils/caseIdentity";
import {
  publicCommentGateReason,
  WORK_STATE_LABEL,
} from "@features/csm-cases/utils/caseWorkState";
import { useRecordRecentView } from "@features/csm-recent/hooks/useRecentViews";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import {
  SLA_CLOCK_LABEL,
  formatTimeToBreach,
} from "@features/csm-dashboard/utils/abtDashboard";
import RelativeTime from "@components/RelativeTime";
import SeverityChip from "@components/SeverityChip";
import StateChip from "@components/StateChip";
import { CASE_TYPE_LABEL } from "@features/csm-cases/utils/caseType";
import type {
  CaseAttachment,
  CaseLifecycleAction,
} from "@features/csm-cases/types/csmCases";
import type { CaseState } from "@features/csm-dashboard/types/abtDashboard";

function MetaCell({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Box>{children}</Box>
    </Box>
  );
}

const LIFECYCLE_TOAST: Record<CaseLifecycleAction, string> = {
  start_work: "Started work on this case.",
  assign_to_me: "Assigned to you.",
  propose_solution: "Solution proposed to the customer.",
  request_info: "Requested additional info from the customer.",
  wait_on_wso2: "Marked as waiting on internal WSO2 dependency.",
  resume_work: "Resumed work on this case.",
  close: "Case closed.",
  close_no_response: "Closed (no response received).",
  transition: "Case updated.",
};

type FeedbackSeverity = "success" | "info" | "warning" | "error";

interface Feedback {
  message: string;
  severity: FeedbackSeverity;
  /** Sticky feedback (state transitions) stays until dismissed; transient
   * feedback (copy link, downloads) auto-dismisses. */
  sticky: boolean;
}

// Lifecycle transitions carry semantic weight — closure/resolution reads as
// success, "waiting" states as a caution. Benign/neutral moves stay info.
const LIFECYCLE_SEVERITY: Record<CaseLifecycleAction, FeedbackSeverity> = {
  start_work: "info",
  assign_to_me: "info",
  propose_solution: "success",
  request_info: "warning",
  wait_on_wso2: "warning",
  resume_work: "info",
  close: "success",
  close_no_response: "success",
  transition: "info",
};

// Lifecycle actions that map to a `PATCH /cases/{id}` state transition.
// `assign_to_me` is intentionally absent — the backend has no assignee field
// yet, so it can't be persisted and stays a local-only acknowledgement.
const LIFECYCLE_TARGET_STATE: Partial<
  Record<CaseLifecycleAction, BeCaseState>
> = {
  start_work: "work_in_progress",
  resume_work: "work_in_progress",
  propose_solution: "solution_proposed",
  request_info: "awaiting_info",
  wait_on_wso2: "waiting_on_wso2",
  close: "closed",
  close_no_response: "closed",
};

const FEEDBACK_PALETTE: Record<
  FeedbackSeverity,
  { bg: string; border: string; fg: string }
> = {
  success: { bg: "success.50", border: "success.main", fg: "success.main" },
  info: { bg: "info.50", border: "info.main", fg: "info.main" },
  warning: { bg: "warning.50", border: "warning.main", fg: "warning.main" },
  error: { bg: "error.50", border: "error.main", fg: "error.main" },
};

const SECONDARY_TOAST: Record<string, string> = {
  reassign_engineer: "Reassign engineer dialog (mock).",
  reassign_group: "Reassign group dialog (mock).",
  escalate: "Escalation form (mock).",
  change_severity: "Severity change request (mock).",
  hold_auto_close: "Hold auto-closure dialog (mock).",
  create_incident: "Create incident from case (mock).",
  link_case: "Link related case picker (mock).",
  link_incident: "Link to incident picker (mock).",
  manage_watchers: "Add/remove watcher dialog (mock).",
  create_task: "Create task dialog (mock).",
  request_call: "Request a call dialog (mock).",
  log_time: "Log time dialog (mock).",
  copy_link: "Case link copied to clipboard.",
};

type CaseTabId =
  | "activities"
  | "details"
  | "sla"
  | "attachments"
  | "time"
  | "call-requests";

/**
 * Walk the parent chain to find the nearest vertically-scrollable element.
 * Falls back to the document scrolling element if none is found.
 */
function findVerticalScrollAncestor(el: HTMLElement): HTMLElement {
  let cur: HTMLElement | null = el.parentElement;
  while (cur && cur !== document.body) {
    const style = window.getComputedStyle(cur);
    const overflowY = style.overflowY;
    if (
      (overflowY === "auto" || overflowY === "scroll") &&
      cur.scrollHeight > cur.clientHeight
    ) {
      return cur;
    }
    cur = cur.parentElement;
  }
  return (document.scrollingElement as HTMLElement | null) ?? document.documentElement;
}

const TAB_DEFS: Array<{
  id: CaseTabId;
  label: string;
  icon: JSX.Element;
  disabled?: boolean;
}> = [
  { id: "activities", label: "Activities", icon: <Activity size={16} /> },
  { id: "details", label: "Details", icon: <ListChecks size={16} /> },
  // SLA tab is parked until its backend clock data is wired; disabled for now.
  { id: "sla", label: "SLA", icon: <Clock size={16} />, disabled: true },
  { id: "attachments", label: "Attachments", icon: <Paperclip size={16} /> },
  { id: "time", label: "Time tracking", icon: <Layers size={16} /> },
  { id: "call-requests", label: "Call requests", icon: <Phone size={16} /> },
];

export default function CsmCaseDetailPage(): JSX.Element {
  const { caseId } = useParams<{ caseId: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const isEngagementRoute = location.pathname.startsWith("/engagements/");
  const backPath = isEngagementRoute ? "/engagements" : "/cases";
  const backLabel = isEngagementRoute ? "Back to engagements" : "Back to cases";
  const detailPath = isEngagementRoute ? `/engagements/${caseId}` : `/cases/${caseId}`;
  const { data, isLoading, isError } = useGetCsmCaseDetail(caseId);
  const {
    data: comments,
    isLoading: isCommentsLoading,
    isError: isCommentsError,
  } = useGetCsmCaseComments(caseId);
  // The chat transcript the case was spawned from, when linked. Loaded lazily
  // off the case's conversation id and merged into the comment stream below so
  // it renders as the earliest activity entries — mirrors the customer portal.
  // Disabled (no fetch) when the case has no linked conversation, so
  // isChatLoading/isChatError stay false for chat-less cases.
  const {
    data: chatMessages,
    isLoading: isChatLoading,
    isError: isChatError,
  } = useGetCsmConversationMessages(data?.conversationId);
  const postComment = usePostCsmCaseComment();
  const {
    data: attachments,
    isLoading: isAttachmentsLoading,
    isError: isAttachmentsError,
    refetch: refetchAttachments,
  } = useGetCsmCaseAttachments(caseId);
  const postAttachment = usePostCsmCaseAttachment();
  const downloadAttachment = useDownloadCsmCaseAttachment();
  const deleteAttachment = useDeleteCsmCaseAttachment();
  const patchCase = usePatchCsmCase(caseId);
  const patchCaseById = usePatchCsmCaseById();
  const findMyOngoingCases = useFindMyOngoingCases();
  const recordView = useRecordRecentView();
  const claims = useIdTokenClaims();
  // Display name for comments authored in this session, resolved from the
  // signed-in user's ID token. Falls back to the email local part so a token
  // without name claims still attributes the comment to the right person.
  const engineerName =
    claims?.name ||
    [claims?.given_name, claims?.family_name].filter(Boolean).join(" ").trim() ||
    claims?.email?.split("@")[0] ||
    "Unknown engineer";
  const { showError } = useErrorBanner();
  const [feedback, setFeedback] = useState<Feedback | null>(null);
  const [activeTab, setActiveTab] = useState<CaseTabId>("activities");
  const [metaCollapsed, setMetaCollapsed] = useState(false);
  const [composerOpen, setComposerOpen] = useState(false);
  const [assignOpen, setAssignOpen] = useState(false);
  const [logTimeOpen, setLogTimeOpen] = useState(false);
  const [githubIssueOpen, setGithubIssueOpen] = useState(false);
  // Inline error shown inside the Git-issue dialog (e.g. the SN routing 422 /
  // state 409). Cleared when the dialog opens or a submit is retried.
  const [githubIssueError, setGithubIssueError] = useState<string | null>(null);
  const postGithubIssue = usePostCaseGithubIssue();
  const postTimeCard = usePostTimeCard();
  // Attachment pending delete confirmation (drives the confirm dialog).
  const [pendingDelete, setPendingDelete] = useState<CaseAttachment | null>(
    null,
  );
  // When starting work would leave the engineer with more than one ongoing case,
  // hold the other ongoing case(s) here to drive the confirm dialog.
  const [pauseConflict, setPauseConflict] = useState<MyOngoingCase[] | null>(
    null,
  );
  // Email of the signed-in engineer, for the "Assign to me" shortcut.
  const currentUserEmail = claims?.email ?? undefined;

  // Twitter-style permalinks: when the URL has a fragment matching an entry id,
  // jump to the Activities tab, scroll the entry into view vertically, and
  // flash it. The browser's default hash-anchor `scrollIntoView` also drags
  // ancestors horizontally if any of them is wider than the viewport (e.g.
  // when a comment contains a wide `<pre>` block). We zero `scrollLeft` on
  // every ancestor to undo that horizontal shift while keeping vertical
  // scroll in place.
  useEffect(() => {
    const hash = location.hash?.replace(/^#/, "");
    if (!hash) return;
    // Permalink: a URL fragment forces the Activities tab. Effect-driven so it
    // also fires when the hash changes while already on the page.
    // eslint-disable-next-line react-hooks/set-state-in-effect -- syncs active tab to URL hash
    setActiveTab("activities");
    // Track every timer (outer + both nested resets) so the cleanup can cancel
    // all of them on unmount / hash change — the inner resets were previously
    // leaked.
    const timers: ReturnType<typeof setTimeout>[] = [];
    const timer = setTimeout(() => {
      const target = document.getElementById(hash);
      if (!target) return;

      // Undo any horizontal scroll the browser introduced on ancestors.
      let cur: HTMLElement | null = target.parentElement;
      while (cur && cur !== document.body) {
        if (cur.scrollLeft !== 0) cur.scrollLeft = 0;
        cur = cur.parentElement;
      }
      if (document.documentElement.scrollLeft !== 0) {
        document.documentElement.scrollLeft = 0;
      }
      if (document.body.scrollLeft !== 0) document.body.scrollLeft = 0;

      const container = findVerticalScrollAncestor(target);
      const containerTop = container === document.documentElement
        ? 0
        : container.getBoundingClientRect().top;
      const targetTop = target.getBoundingClientRect().top;
      const offset = 96;
      const delta = targetTop - containerTop - offset;
      container.scrollTo({
        top: container.scrollTop + delta,
        behavior: "smooth",
      });

      const prevTransition = target.style.transition;
      const prevBg = target.style.backgroundColor;
      target.style.transition = "background-color 200ms ease-out";
      target.style.backgroundColor = "rgba(255, 213, 79, 0.35)";
      const reset = setTimeout(() => {
        target.style.backgroundColor = prevBg;
        timers.push(
          setTimeout(() => {
            target.style.transition = prevTransition;
          }, 350),
        );
      }, 1500);
      timers.push(reset);
    }, 250);
    timers.push(timer);
    return () => timers.forEach(clearTimeout);
  }, [location.hash, data, comments]);

  useEffect(() => {
    // State-transition feedback is sticky (persists until dismissed) so it
    // isn't missed; transient confirmations auto-dismiss.
    if (!feedback || feedback.sticky) return;
    const t = setTimeout(() => setFeedback(null), 2500);
    return () => clearTimeout(t);
  }, [feedback]);

  useEffect(() => {
    if (!data) return;
    recordView({
      kind: "case",
      id: data.id,
      // Lead with the human case id(s) (WSO2 id / CS number, never the UUID) as
      // the recent/pinned label — what engineers and customers reference. Falls
      // back to the subject alone when a case has no human id yet.
      title: caseIdLabel(data)
        ? `${caseIdLabel(data)} · ${data.subject}`
        : data.subject,
      subtitle: `${data.customer} · ${data.projectName}`,
      href: detailPath,
    });
  }, [data, recordView]);

  const onAction = useCallback(
    (
      action: CaseLifecycleAction | { secondary: string },
      // Target state supplied by the action bar, taken straight from the case's
      // backend `nextStates`. It is authoritative for the PATCH: the action name
      // (e.g. `resume_work`) maps to different states depending on the source
      // state, so we never re-derive the target from the action alone.
      nextState?: CaseState,
    ) => {
      if (typeof action === "string") {
        const targetState = nextState
          ? beStateFromUi(nextState)
          : LIFECYCLE_TARGET_STATE[action];

        // Starting work: enforce the single-active-case rule.
        // 1) look up the engineer's other ongoing cases (abort on failure — we
        //    must not transition without knowing), 2) move this case to
        // work_in_progress, 3) if none ongoing → mark this one ongoing; if some
        // exist → ask before pausing them (handled in onConfirmStartWork).
        if (targetState === "work_in_progress" && data) {
          const caseId = data.id;
          void (async () => {
            let others: MyOngoingCase[];
            try {
              others = await findMyOngoingCases(caseId);
            } catch (err) {
              // Don't proceed blind: marking this ongoing without knowing the
              // other active cases would break the single-active-case rule.
              showError(
                "Couldn't check your other active cases. Please try again.",
                err,
              );
              return;
            }
            try {
              await patchCase.mutateAsync({ state: "work_in_progress" });
            } catch (err) {
              showError(
                "Could not move the case to Work in progress. Please try again.",
                err,
              );
              return;
            }
            if (others.length === 0) {
              // No competing case → make this the active (ongoing) one.
              try {
                await patchCase.mutateAsync({ workState: "ongoing" });
              } catch (err) {
                showError(
                  "Moved to Work in progress, but could not mark it ongoing.",
                  err,
                );
                return;
              }
              setFeedback({
                message: LIFECYCLE_TOAST[action],
                severity: LIFECYCLE_SEVERITY[action],
                sticky: true,
              });
              return;
            }
            // Others are ongoing → confirm before pausing them.
            setPauseConflict(others);
          })();
          return;
        }

        if (targetState) {
          // Real state transition via PATCH /cases/{id}; the detail + list
          // queries refetch on success so the new state shows.
          patchCase.mutate(
            { state: targetState },
            {
              onSuccess: () =>
                setFeedback({
                  message: LIFECYCLE_TOAST[action],
                  severity: LIFECYCLE_SEVERITY[action],
                  sticky: true,
                }),
              onError: (err) =>
                showError(
                  "Could not update the case. Please try again.",
                  err,
                ),
            },
          );
          return;
        }
        // No backend state change (e.g. assign_to_me — no assignee field yet):
        // local acknowledgement only.
        setFeedback({
          message: LIFECYCLE_TOAST[action],
          severity: LIFECYCLE_SEVERITY[action],
          sticky: true,
        });
        return;
      }

      // Assign / reassign opens the engineer picker; the PATCH happens in
      // onAssign once an engineer is chosen.
      if (action.secondary === "reassign_engineer") {
        setAssignOpen(true);
        return;
      }

      // Pause / resume the work sub-state via PATCH { workState }. Only for an
      // in-progress case assigned to the current user. Pausing is a direct
      // single-field patch. Resuming sets this case `ongoing`, so it runs the
      // same single-active-case conflict check as starting work — otherwise an
      // engineer with another ongoing case would end up with two.
      if (action.secondary === "toggle_work_state") {
        if (!data || data.state !== "work_in_progress") return;
        // Anything other than `ongoing` (paused OR a null work-state) resumes.
        const resuming = data.workState !== "ongoing";

        if (!resuming) {
          patchCase.mutate(
            { workState: "paused" },
            {
              onSuccess: () =>
                setFeedback({
                  message:
                    "Work paused — customer replies are disabled until you resume.",
                  severity: "warning",
                  sticky: true,
                }),
              onError: (err) =>
                showError(
                  "Could not update the work state. Please try again.",
                  err,
                ),
            },
          );
          return;
        }

        // Resuming → ongoing: check for other active cases first (abort on
        // failure), then either mark ongoing or prompt to pause the others.
        const caseId = data.id;
        void (async () => {
          let others: MyOngoingCase[];
          try {
            others = await findMyOngoingCases(caseId);
          } catch (err) {
            showError(
              "Couldn't check your other active cases. Please try again.",
              err,
            );
            return;
          }
          if (others.length === 0) {
            try {
              await patchCase.mutateAsync({ workState: "ongoing" });
            } catch (err) {
              showError(
                "Could not resume work on this case. Please try again.",
                err,
              );
              return;
            }
            setFeedback({
              message: "Resumed work on this case.",
              severity: "info",
              sticky: true,
            });
            return;
          }
          setPauseConflict(others);
        })();
        return;
      }

      // Copy-link is async: only confirm success once the clipboard write
      // actually resolves, otherwise a failure shows both a false "copied"
      // toast and an error.
      if (action.secondary === "copy_link") {
        if (data && navigator.clipboard) {
          navigator.clipboard
            .writeText(`${window.location.origin}${detailPath}`)
            .then(() =>
              setFeedback({
                message: SECONDARY_TOAST.copy_link,
                severity: "success",
                sticky: false,
              }),
            )
            .catch(() => showError("Could not copy link."));
        } else {
          showError("Could not copy link.");
        }
        return;
      }

      if (action.secondary === "log_time") {
        // CAMG-006: closed cases are read-only — no new time entries.
        if (data?.state === "closed") {
          showError("This case is closed — time tracking is read-only.");
          return;
        }
        setLogTimeOpen(true);
        return;
      }

      // ISSU-020: file an internal GitHub issue from the case. The SN side
      // gates on the case state (only certain states may file) and resolves
      // the target repo from the product; we don't pre-gate here — open the
      // dialog and surface any backend rejection inline.
      if (action.secondary === "raise_git_issue") {
        setGithubIssueError(null);
        setGithubIssueOpen(true);
        return;
      }

      setFeedback({
        message: SECONDARY_TOAST[action.secondary] ?? `Action: ${action.secondary}`,
        severity: "info",
        sticky: false,
      });
    },
    [data, showError, patchCase, findMyOngoingCases, detailPath],
  );

  // Confirm pausing the engineer's other ongoing case(s) and making this case
  // the active one. By the time the dialog opens this case is already
  // work_in_progress (whether via start-work or resume); here we pause each
  // other case then mark this one ongoing.
  const onConfirmStartWork = useCallback(async () => {
    const others = pauseConflict;
    if (!others || !data) return;
    setPauseConflict(null);
    try {
      for (const o of others) {
        await patchCaseById(o.id, { workState: "paused" });
      }
      await patchCase.mutateAsync({ workState: "ongoing" });
      setFeedback({
        message:
          others.length === 1
            ? `Paused ${others[0].label} and made this your active case.`
            : `Paused ${others.length} other ongoing cases and made this your active case.`,
        severity: "info",
        sticky: true,
      });
    } catch (err) {
      showError("Could not update the work states. Please try again.", err);
    }
  }, [pauseConflict, data, patchCaseById, patchCase, showError]);

  // Decline: keep the move to work_in_progress (already applied) but leave this
  // case not-ongoing and the other case(s) ongoing.
  const onDeclineStartWork = useCallback(() => {
    setPauseConflict(null);
    setFeedback({
      message:
        "Moved to Work in progress. Your other case stays your active one.",
      severity: "info",
      sticky: true,
    });
  }, []);

  // Assign the case to the chosen engineer via PATCH { assigneeEmail }. The
  // detail query is invalidated by the hook, so the assignee display refreshes
  // on success. (ServiceNow-source only; the BE rejects it for PG cases.)
  const onAssign = useCallback(
    (email: string) => {
      patchCase.mutate(
        { assigneeEmail: email },
        {
          onSuccess: () => {
            setAssignOpen(false);
            setFeedback({
              message: "Case reassigned.",
              severity: "success",
              sticky: true,
            });
          },
          onError: (err) => showError("Could not reassign the case.", err),
        },
      );
    },
    [patchCase, showError],
  );

  const attachmentList = useMemo(() => attachments ?? [], [attachments]);

  // Case comments + the linked chat transcript, as one list for the activity
  // feed. Memoised so the feed's own sort doesn't rerun on every render.
  const mergedComments = useMemo(
    () => [...(comments ?? []), ...(chatMessages ?? [])],
    [comments, chatMessages],
  );

  const onUploadAttachment = useCallback(
    (file: File) => {
      if (!caseId) return;
      postAttachment.mutate(
        { caseId, file, uploadedBy: engineerName },
        {
          onSuccess: () =>
            setFeedback({
              message: `Uploaded ${file.name}.`,
              severity: "success",
              sticky: false,
            }),
          // Failures surface inline on the widget via postAttachment.error.
        },
      );
    },
    [caseId, engineerName, postAttachment],
  );

  const onDownloadAttachment = useCallback(
    (attachment: CaseAttachment) => {
      void downloadAttachment(attachment).catch((err) =>
        showError(`Could not download ${attachment.filename}.`, err),
      );
    },
    [downloadAttachment, showError],
  );

  const onDownloadAllAttachments = useCallback(() => {
    // No bulk endpoint; fetch each sequentially (not a parallel burst) and save.
    void (async () => {
      for (const a of attachmentList) {
        try {
          await downloadAttachment(a);
        } catch (err) {
          showError(`Could not download ${a.filename}.`, err);
        }
      }
    })();
  }, [attachmentList, downloadAttachment, showError]);

  const onConfirmDeleteAttachment = useCallback(() => {
    if (!caseId || !pendingDelete) return;
    const target = pendingDelete;
    deleteAttachment.mutate(
      { caseId, attachmentId: target.id },
      {
        onSuccess: () => {
          setPendingDelete(null);
          setFeedback({
            message: `Deleted ${target.filename}.`,
            severity: "success",
            sticky: false,
          });
        },
        onError: (err) => {
          setPendingDelete(null);
          showError(`Could not delete ${target.filename}.`, err);
        },
      },
    );
  }, [caseId, pendingDelete, deleteAttachment, showError]);

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rectangular" height={32} width={240} />
        <Skeleton variant="rectangular" height={200} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate(backPath)}
          sx={{ alignSelf: "flex-start" }}
        >
          {backLabel}
        </Button>
        <Typography variant="body1" color="error">
          Could not load case {caseId}.
        </Typography>
      </Box>
    );
  }

  if (!data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate(backPath)}
          sx={{ alignSelf: "flex-start" }}
        >
          {backLabel}
        </Button>
        <Typography variant="h5">Case not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No case found with id <code>{caseId}</code>.
        </Typography>
      </Box>
    );
  }

  const c = data;
  const isClosed = c.state === "closed";
  // The backend rejects a customer-visible comment unless the case is
  // work_in_progress + ongoing. Internal work notes are allowed in any state,
  // so this only disables the public-reply path in the composer — never work
  // notes. Mirrors the BFF comment guard so the engineer sees a clear reason
  // instead of a generic error.
  const publicReplyGateReason = publicCommentGateReason(c.state, c.workState);
  const breached = c.minutesToBreach < 0;
  // The backend carries no SLA timing yet (LIVE detail leaves slaClocks empty
  // and minutesToBreach at 0). Without this guard the header SLA chip reads
  // `0 <= 60` and paints every open case orange ("Ack · 0m left"), which is
  // both wrong and the main source of orange on the screen. Only show SLA
  // affordances when we actually have clock data (mock, and LIVE once wired).
  const hasSlaData = c.slaClocks.length > 0;
  // The case description is already returned by `comments/search` as the
  // opening comment, so the stream renders it directly — no synthetic entry is
  // injected (that duplicated the first comment). The linked chat transcript
  // (if any) is appended; the feed sorts chronologically, so the chat — being
  // oldest — sinks below the case comments in the default newest-first view.
  const safeComments = mergedComments;

  // SLA breach is a live condition, not a dismissible notice: surface it in a
  // persistent banner under the header so it stays visible on every tab, not
  // just the SLA tab.
  const breachedClocks = c.slaClocks.filter((k) => k.state === "breached");
  const showBreachBanner = breached || breachedClocks.length > 0;
  const breachMessage =
    breachedClocks.length > 0
      ? `SLA breached — ${breachedClocks
          .map(
            (k) =>
              `${SLA_CLOCK_LABEL[k.clockType]} ${formatTimeToBreach(
                k.minutesToBreach,
              )}`,
          )
          .join(", ")}`
      : `SLA breached — ${SLA_CLOCK_LABEL[c.slaClockType]} ${formatTimeToBreach(
          c.minutesToBreach,
        )}`;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(backPath)}
        sx={{ alignSelf: "flex-start" }}
      >
        {backLabel}
      </Button>

      <Box
        sx={{
          display: "flex",
          gap: 2,
          alignItems: "flex-start",
          flexWrap: { xs: "wrap", md: "nowrap" },
          justifyContent: "space-between",
        }}
      >
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            gap: 1,
            flex: 1,
            minWidth: 0,
          }}
        >
          {/* Case identity: project-scoped WSO2 id + case number, joined by a
              slash and given prominence so it reads as the case's headline. */}
          <Typography
            variant="h6"
            sx={{
              fontFamily: "monospace",
              fontWeight: 700,
              letterSpacing: 0.2,
              lineHeight: 1.2,
            }}
          >
            {c.wso2CaseId && (
              <Box component="span" sx={{ color: "text.secondary" }}>
                {c.wso2CaseId}
              </Box>
            )}
            {c.wso2CaseId && c.caseNumber && (
              <Box component="span" sx={{ color: "text.disabled", mx: 0.25 }}>
                /
              </Box>
            )}
            {c.caseNumber && (
              <Box component="span" sx={{ color: "text.primary" }}>
                {c.caseNumber}
              </Box>
            )}
            {!c.wso2CaseId && !c.caseNumber && (
              <Box component="span" sx={{ color: "text.disabled" }}>
                —
              </Box>
            )}
          </Typography>

          {/* Status group (severity, lifecycle state, SLA) kept visually
              distinct from the free-form tags by a divider, so the current
              state doesn't get lost among the tag chips. */}
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
            {c.caseType && CASE_TYPE_LABEL[c.caseType] && (
              <Chip
                size="small"
                variant="outlined"
                label={CASE_TYPE_LABEL[c.caseType]}
                sx={{ fontWeight: 600 }}
              />
            )}
            <SeverityChip severity={c.severity} withLabel />
            <StateChip state={c.state} />
            {c.state === "work_in_progress" && c.workState && (
              <Chip
                size="small"
                variant="outlined"
                color={c.workState === "paused" ? "warning" : "default"}
                label={WORK_STATE_LABEL[c.workState]}
                sx={{ fontWeight: 600 }}
              />
            )}
            {!isClosed && hasSlaData && (
              <Chip
                size="small"
                variant="outlined"
                color={breached ? "error" : c.minutesToBreach <= 60 ? "warning" : "default"}
                label={`${SLA_CLOCK_LABEL[c.slaClockType]} · ${formatTimeToBreach(c.minutesToBreach)}`}
              />
            )}
            {c.tags.length > 0 && (
              <Divider orientation="vertical" flexItem sx={{ mx: 0.5, my: 0.25 }} />
            )}
            {c.tags.map((t) => (
              <Chip
                key={t.id}
                size="small"
                variant="outlined"
                color={t.color ?? "default"}
                label={t.label}
              />
            ))}
          </Box>
          <Typography variant="h5">{c.subject}</Typography>
        </Box>
        <Box sx={{ flexShrink: 0, alignSelf: { xs: "stretch", md: "flex-start" } }}>
          <CaseActionBar caseDetail={c} onAction={onAction} />
        </Box>
      </Box>

      <CaseMetaBand
        detail={c}
        collapsed={metaCollapsed}
        onToggleCollapsed={() => setMetaCollapsed((v) => !v)}
      />

      {showBreachBanner && (
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1,
            p: 1.25,
            borderRadius: 1,
            backgroundColor: "error.50",
            border: 1,
            borderColor: "error.main",
            color: "error.main",
          }}
        >
          <TriangleAlert size={18} />
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            {breachMessage}
          </Typography>
        </Box>
      )}

      {feedback && (
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1,
            p: 1,
            pl: 1.5,
            borderRadius: 1,
            backgroundColor: FEEDBACK_PALETTE[feedback.severity].bg,
            border: 1,
            borderColor: FEEDBACK_PALETTE[feedback.severity].border,
            color: FEEDBACK_PALETTE[feedback.severity].fg,
          }}
        >
          <Typography variant="body2" sx={{ flex: 1 }}>
            {feedback.message}
          </Typography>
          <IconButton
            size="small"
            onClick={() => setFeedback(null)}
            aria-label="Dismiss"
            sx={{ color: "inherit" }}
          >
            <X size={16} />
          </IconButton>
        </Box>
      )}

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={activeTab}
          onChange={(_, v) => setActiveTab(v as CaseTabId)}
          variant="scrollable"
          scrollButtons="auto"
        >
          {TAB_DEFS.map((t) => {
            // Counts shown only where the tab IS the list (unambiguous).
            const count =
              t.id === "attachments"
                ? attachmentList.length
                : t.id === "time"
                  ? c.timeLogs.length
                  : undefined;
            return (
              <Tab
                key={t.id}
                value={t.id}
                disabled={t.disabled}
                icon={t.icon}
                iconPosition="start"
                label={count ? `${t.label} (${count})` : t.label}
                sx={{ minHeight: 44, textTransform: "none" }}
              />
            );
          })}
        </Tabs>
      </Box>

      {activeTab === "activities" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          {/* Comments are latest-in-top (WSO2 convention), so the composer
              belongs at the top. It stays collapsed behind a button to keep
              the thread the focal point until the engineer chooses to reply.
              The composer is always available (an internal work note can be
              added in any state); the public-reply path is gated inside it via
              `publicReplyGateReason` when the case isn't in-progress/ongoing. */}
          {composerOpen ? (
            <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                }}
              >
                <Typography variant="subtitle2">Reply</Typography>
                <Button
                  size="small"
                  variant="text"
                  color="inherit"
                  onClick={() => setComposerOpen(false)}
                >
                  Cancel
                </Button>
              </Box>
              <CsmCaseCommentInput
                disabled={!caseId}
                publicCommentDisabledReason={publicReplyGateReason}
                autoFocus
                onSubmit={async (bodyHtml, internal, commentAttachments) => {
                  if (!caseId) return;
                  // Post the comment only when there's text; an attachment-only
                  // send skips the comment endpoint and just uploads the files.
                  const hasText =
                    bodyHtml
                      .replace(/<[^>]*>/g, "")
                      .replace(/&nbsp;/g, " ")
                      .trim().length > 0;
                  if (hasText) {
                    await postComment.mutateAsync({
                      caseId,
                      bodyHtml,
                      authorName: engineerName,
                      internal,
                    });
                  }
                  // Attachments are case-level (no comment linkage on the BE);
                  // upload each sequentially so a failure surfaces clearly.
                  for (const { file, name } of commentAttachments) {
                    await postAttachment.mutateAsync({
                      caseId,
                      file,
                      name,
                      uploadedBy: engineerName,
                    });
                  }
                  // Collapse only on success; on error the input keeps its
                  // draft + files and surfaces the failure.
                  setComposerOpen(false);
                }}
              />
            </Card>
          ) : (
            // Full-width collapsed composer: a faux input bar that fills the
            // row and reads as "click to write a reply" rather than a lone
            // button floating in empty space.
            <Button
              fullWidth
              variant="outlined"
              color="inherit"
              startIcon={<MessageSquarePlus size={18} />}
              onClick={() => setComposerOpen(true)}
              sx={{
                justifyContent: "flex-start",
                textTransform: "none",
                gap: 0.5,
                py: 1.5,
                px: 2,
                color: "text.secondary",
                borderColor: "divider",
                borderStyle: "dashed",
                "&:hover": {
                  borderColor: "primary.main",
                  borderStyle: "solid",
                  backgroundColor: "action.hover",
                },
              }}
            >
              {publicReplyGateReason
                ? "Add an internal work note…"
                : "Compose a reply to the customer…"}
            </Button>
          )}

          <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
              <Typography variant="subtitle2">Activity timeline</Typography>
              {!isCommentsLoading && (
                <Chip
                  size="small"
                  variant="outlined"
                  label={`${safeComments.length + c.audit.length + attachmentList.length} entries`}
                />
              )}
            </Box>

            {isCommentsLoading || isChatLoading ? (
              // Wait for both the comments and the linked chat transcript so the
              // transcript doesn't pop into an already-rendered timeline.
              // isChatLoading is false for chat-less cases (query disabled).
              <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                {[0, 1, 2].map((i) => (
                  <Skeleton key={i} variant="rectangular" height={56} />
                ))}
              </Box>
            ) : (
              // A comments/chat failure shouldn't blank the timeline — the
              // description, audit, and attachments loaded fine. Show them with
              // an inline notice.
              <>
                {isCommentsError && (
                  <Typography variant="body2" color="error">
                    Could not load comments. Showing the rest of the activity —
                    reload to try again.
                  </Typography>
                )}
                {isChatError && (
                  <Typography variant="body2" color="error">
                    Could not load the chat conversation. Showing the rest of the
                    activity — reload to try again.
                  </Typography>
                )}
                <CaseActivitiesFeed
                  comments={safeComments}
                  audit={c.audit}
                  attachments={attachmentList}
                  onDownloadAttachment={onDownloadAttachment}
                />
              </>
            )}
          </Card>
        </Box>
      )}

      {activeTab === "details" && (
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              md: "repeat(2, minmax(0, 1fr))",
            },
            alignItems: "start",
          }}
        >
          <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Typography variant="subtitle2">Identifiers &amp; timestamps</Typography>
            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: "1fr 1fr",
                gap: 2,
              }}
            >
              <MetaCell label="Case number">
                <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
                  {c.caseNumber ?? "—"}
                </Typography>
              </MetaCell>
              <MetaCell label="WSO2 case ID">
                <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
                  {c.wso2CaseId ?? "—"}
                </Typography>
              </MetaCell>
              <MetaCell label="Created">
                <Typography variant="body2">
                  <RelativeTime iso={c.createdAt} />
                </Typography>
              </MetaCell>
              <MetaCell label="Last update">
                <Typography variant="body2">
                  <RelativeTime iso={c.updatedAt} />
                </Typography>
              </MetaCell>
              <MetaCell label="Assignment group (ABT)">
                <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
                  {c.assignmentGroup}
                </Typography>
              </MetaCell>
            </Box>
          </Card>
          <CustomerContextWidget ctx={c.customerContext} />
          <ProductContextWidget ctx={c.productContext} />
          <WatchersWidget
            watchers={c.watchers}
            onAdd={() => onAction({ secondary: "manage_watchers" })}
          />
          <LinkedItemsWidget
            items={c.linkedItems}
            onLink={() => onAction({ secondary: "link_case" })}
          />
        </Box>
      )}

      {activeTab === "sla" && (
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              md: "repeat(2, minmax(0, 1fr))",
            },
          }}
        >
          <SlaTimelineWidget clocks={c.slaClocks} />
          <AuditTimelineWidget entries={c.audit} />
        </Box>
      )}

      {activeTab === "attachments" && (
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: "1fr" }}>
          <AttachmentsWidget
            attachments={attachmentList}
            loading={isAttachmentsLoading}
            error={isAttachmentsError}
            onRetry={() => void refetchAttachments()}
            uploading={postAttachment.isPending}
            uploadError={
              postAttachment.isError
                ? (postAttachment.error?.message ??
                  "Could not upload the attachment.")
                : null
            }
            onUpload={onUploadAttachment}
            onDownloadAll={onDownloadAllAttachments}
            onDownload={onDownloadAttachment}
            onDelete={setPendingDelete}
            deletingId={deleteAttachment.isPending ? pendingDelete?.id : null}
          />
        </Box>
      )}

      {activeTab === "time" && (
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: "1fr" }}>
          <CaseTimeCardsPanel
            caseId={c.id}
            projectId={c.projectId}
            onLogTime={() => setLogTimeOpen(true)}
            readOnly={isClosed}
          />
        </Box>
      )}

      {activeTab === "call-requests" && caseId && (
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: "1fr" }}>
          <CallRequestsWidget caseId={caseId} severity={c.severity} />
        </Box>
      )}

      {assignOpen && (
        <AssignEngineerDialog
          currentAssignee={c.assignee}
          currentUserEmail={currentUserEmail}
          isAssigning={patchCase.isPending}
          onClose={() => setAssignOpen(false)}
          onAssign={onAssign}
        />
      )}

      {logTimeOpen && (
        <LogTimeCardDialog
          caseId={c.id}
          caseNumber={c.caseNumber ?? c.id}
          projectId={c.projectId}
          projectName={c.projectName}
          isSubmitting={postTimeCard.isPending}
          onClose={() => setLogTimeOpen(false)}
          onSubmit={(input) =>
            postTimeCard.mutate(input, {
              onSuccess: () => {
                setLogTimeOpen(false);
                setActiveTab("time");
                setFeedback({
                  message: "Time card submitted for review.",
                  severity: "success",
                  sticky: false,
                });
              },
              onError: (err) => {
                // Surface the backend's own message on 4xx (e.g. an invalid
                // approver or hour value) instead of a generic string, same
                // as the decide-flow in CsmTimeCardsPage.tsx.
                const msg =
                  err instanceof BackendApiError && err.status < 500 && err.message
                    ? err.message
                    : "Could not log time.";
                showError(msg, err);
              },
            })
          }
        />
      )}

      {githubIssueOpen && (
        <CreateGithubIssueDialog
          open={githubIssueOpen}
          submitting={postGithubIssue.isPending}
          error={githubIssueError}
          defaultUpdateLevel={c.productContext.updateLevel}
          defaultTitle={c.subject}
          defaultDescription={c.description}
          onClose={() => {
            setGithubIssueOpen(false);
            setGithubIssueError(null);
          }}
          onSubmit={(payload) => {
            setGithubIssueError(null);
            postGithubIssue.mutate(
              { caseId: c.id, ...payload },
              {
                onSuccess: (res) => {
                  setGithubIssueOpen(false);
                  // The SN side writes the issue URL into work notes; show that
                  // entry by switching to the activities feed (just refetched).
                  setActiveTab("activities");
                  setFeedback({
                    message: res.issue?.url
                      ? `Internal Git issue created: ${res.issue.url}`
                      : "Internal Git issue created.",
                    severity: "success",
                    sticky: true,
                  });
                },
                onError: (err) => {
                  // Surface the backend's own message on 4xx (invalid state,
                  // product not routable) inline in the dialog; fall back to a
                  // generic banner for 5xx.
                  if (
                    err instanceof BackendApiError &&
                    err.status < 500 &&
                    err.message
                  ) {
                    setGithubIssueError(err.message);
                  } else {
                    showError("Could not create the Git issue.", err);
                  }
                },
              },
            );
          }}
        />
      )}

      <Dialog
        open={!!pendingDelete}
        onClose={() => {
          if (!deleteAttachment.isPending) setPendingDelete(null);
        }}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>Delete attachment?</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            Permanently delete{" "}
            <strong>{pendingDelete?.filename}</strong>? This can't be undone.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button
            color="inherit"
            onClick={() => setPendingDelete(null)}
            disabled={deleteAttachment.isPending}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            color="error"
            onClick={onConfirmDeleteAttachment}
            disabled={deleteAttachment.isPending}
          >
            {deleteAttachment.isPending ? "Deleting…" : "Delete"}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={!!pauseConflict}
        onClose={onDeclineStartWork}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>
          {pauseConflict && pauseConflict.length > 1
            ? "Pause your other active cases?"
            : "Pause your other active case?"}
        </DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            You're already working on{" "}
            <strong>
              {pauseConflict?.map((o) => o.label).join(", ")}
            </strong>
            . Pause {pauseConflict && pauseConflict.length > 1 ? "them" : "it"}{" "}
            and make this case your active (ongoing) one?
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button color="inherit" onClick={onDeclineStartWork}>
            No, keep it active
          </Button>
          <Button
            variant="contained"
            onClick={() => void onConfirmStartWork()}
            disabled={patchCase.isPending}
          >
            Pause and start this
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
