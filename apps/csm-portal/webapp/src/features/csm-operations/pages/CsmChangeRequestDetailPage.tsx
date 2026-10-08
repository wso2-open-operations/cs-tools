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
  Alert,
  Box,
  Button,
  Card,
  Chip,
  Skeleton,
  Tab,
  Tabs,
  Typography,
} from "@wso2/oxygen-ui";
import {
  ArrowLeft,
  Check,
  Clock,
  ClipboardCheck,
  CopyPlus,
  FileText,
  MessageSquare,
  MessageSquarePlus,
  Paperclip,
  Pencil,
  X,
} from "@wso2/oxygen-ui-icons-react";
import {
  type JSX,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useLocation } from "react-router";
import { ApiQueryKeys } from "@constants/apiConstants";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import { BackendApiError } from "@api/backend/client";
import ExportPdfButton from "@components/ExportPdfButton";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useEngineerDisplayName } from "@hooks/useEngineerDisplayName";
import { useRecordRecentView } from "@features/csm-recent/hooks/useRecentViews";
import { useGetChangeRequest } from "@features/csm-operations/api/useGetChangeRequest";
import { useGetChangeRequestApprovals } from "@features/csm-operations/api/useGetChangeRequestApprovals";
import { usePatchChangeRequest } from "@features/csm-operations/api/usePatchChangeRequest";
import {
  useGetCsmChangeRequestComments,
  usePostCsmChangeRequestComment,
} from "@features/csm-operations/api/useCsmChangeRequestComments";
import {
  useDeleteComment,
  usePatchComment,
} from "@features/csm-cases/api/useCsmCaseComments";
import ChangeRequestAcceptProposedTimeDialog from "@features/csm-operations/components/ChangeRequestAcceptProposedTimeDialog";
import ChangeRequestActionBar from "@features/csm-operations/components/ChangeRequestActionBar";
import ChangeRequestApprovals from "@features/csm-operations/components/ChangeRequestApprovals";
import ChangeRequestLifecycleStepper from "@features/csm-operations/components/ChangeRequestLifecycleStepper";
import ChangeRequestProposedTimeBanner from "@features/csm-operations/components/ChangeRequestProposedTimeBanner";
import ChangeRequestRescheduleDialog from "@features/csm-operations/components/ChangeRequestRescheduleDialog";
import ChangeRequestTransitionReasonDialog from "@features/csm-operations/components/ChangeRequestTransitionReasonDialog";
import EditChangeRequestDialog from "@features/csm-operations/components/EditChangeRequestDialog";
import EntityRefLink from "@features/csm-operations/components/EntityRefLink";
import {
  buildCloneChangeRequestNavState,
  changeRequestBlockingReason,
  changeRequestCategoryLabel,
  noCustomerAskedHelper,
  isChangeRequestCreator,
  pendingCustomerReview,
  changeRequestCommentGateReason,
  changeRequestTransitionRequiresReason,
  changeRequestImpactColor,
  changeRequestImpactLabel,
  changeRequestStateColor,
  changeRequestStateLabel,
  CUSTOMER_STEP_NOT_APPLICABLE,
  customerApprovedDisplay,
  isCustomerStepNotApplicable,
  customerProposalProposer,
  answerSnapshotMoved,
  isStaleAnswerError,
  pendingCustomerProposal,
} from "@features/csm-operations/utils/changeRequests";
import CaseActivitiesFeed from "@features/csm-cases/components/CaseActivitiesFeed";
import CsmCaseCommentInput from "@features/csm-cases/components/CsmCaseCommentInput";
import { AttachmentsWidget } from "@features/csm-cases/components/CaseDetailWidgets";
import {
  useGetCsmCaseAttachments,
  usePostCsmCaseAttachment,
  useDownloadCsmCaseAttachment,
} from "@features/csm-cases/api/useCsmCaseAttachments";
import type {
  BeChangeRequestCustomerProposal,
  BeChangeRequestDetail,
  BeEntityRef,
  BePatchChangeRequestPayload,
} from "@api/backend/types";
import { useNavTransition } from "@hooks/useNavTransition";
import { useNormalizedIdParam } from "@hooks/useNormalizedIdParam";
import { useCaseRouteOverride } from "@context/case-tabs/CaseRouteOverrideContext";
import { useReportCaseTabMeta } from "@features/case-tabs/hooks/useReportCaseTabMeta";
import { useReportCaseTabDraft } from "@features/case-tabs/hooks/useReportCaseTabDraft";
import { useQueryParamTabs } from "@hooks/useSectionTabs";
import { isHttpUrl } from "@utils/isHttpUrl";

const OPERATIONS_CR_PATH = "/operations/change-requests";

/**
 * The backend surfaces real rejection reasons on 4xx (e.g. a state
 * transition rejected by the backing data source); prefer that message over
 * a generic fallback whenever one is available.
 */
function backendErrorMessage(err: unknown, fallback: string): string {
  return err instanceof BackendApiError && err.status < 500 && err.message
    ? err.message
    : fallback;
}

/**
 * The patch that performs a transition into `target`.
 *
 * Every target — including `assess` — goes through the plain `state` field.
 * New -> Assess used to be modeled as a special "approval request" action
 * (`{requestApproval: true}`), but that was backwards relative to the real
 * ServiceNow process (confirmed against the live instance): it's a direct,
 * ungated state change, exactly like every other forward transition in this
 * bar ("Mark implemented", …) — there is no approval gate on this
 * move at all. `requestApproval` is a separate, unrelated bookkeeping flag on
 * the same PATCH endpoint that this action bar no longer has any reason to
 * set.
 */
function buildTransitionPatch(target: string): BePatchChangeRequestPayload {
  return { state: target };
}

/**
 * Fallback message for a failed transition, used only when the backend gave
 * no usable 4xx reason of its own.
 */
function transitionFallbackMessage(target: string): string {
  return `Could not move this change request to ${changeRequestStateLabel(target)}.`;
}

/** A refusal's words as one sentence of the page's own notice: first letter up, a full stop at the end. */
function asSentence(message: string): string {
  const text = message.trim();
  if (!text) return text;
  return `${text.charAt(0).toUpperCase()}${text.slice(1)}${/[.!?]$/.test(text) ? "" : "."}`;
}

function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      dateStyle: "medium",
      timeStyle: "short",
    }) ?? "—"
  );
}

function MetaCell({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
      >
        {label}
      </Typography>
      <Box sx={{ minWidth: 0 }}>{children}</Box>
    </Box>
  );
}

/** WSO2's answer to a time the customer proposed (the previous system's Agree / Disagree), "—" while unanswered. */
function wso2AnswerLabel(raw?: string | null): string {
  const answer = raw?.trim().toLowerCase();
  return answer === "agree" ? "Agree" : answer === "disagree" ? "Disagree" : "—";
}

function RefText({ value }: { value?: BeEntityRef | null }): JSX.Element {
  return <Typography variant="body2">{value?.name || "—"}</Typography>;
}

/** A multi-valued reference (Deployments / Deployment products / Customer Group) as chips, "—" when empty. */
function RefChips({ values }: { values?: BeEntityRef[] | null }): JSX.Element {
  if (!values?.length) return <Typography variant="body2">—</Typography>;
  return (
    <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.75 }}>
      {values.map((v) => (
        <Chip key={v.id} size="small" variant="outlined" label={v.name} />
      ))}
    </Box>
  );
}

/**
 * Yes / No, or "Not applicable" for a customer step an Emergency change does not
 * have (it acts without customer consent: it never reaches a customer state). A
 * step such a change still carries from before that rule reads as it is stored.
 */
function YesNo({ value, notApplicable = false }: { value?: boolean; notApplicable?: boolean }): JSX.Element {
  if (notApplicable) {
    return (
      <Typography variant="body2" color="text.secondary">
        {CUSTOMER_STEP_NOT_APPLICABLE}
      </Typography>
    );
  }
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
      {value ? <Check size={14} /> : <X size={14} />}
      <Typography variant="body2">{value ? "Yes" : "No"}</Typography>
    </Box>
  );
}

/**
 * The "Customer approved" cell: Yes / No, or "Proposed time accepted" for a change that went to Scheduled
 * because WSO2 accepted the time the customer proposed (nothing is stamped as the customer's approval
 * then: a plain "No" would mislead).
 */
function CustomerApprovedValue({ cr, notApplicable }: { cr: BeChangeRequestDetail; notApplicable: boolean }): JSX.Element {
  const display = customerApprovedDisplay(cr);
  if (display === "Proposed time accepted") {
    return (
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
        <Check size={14} />
        <Typography variant="body2">{display}</Typography>
      </Box>
    );
  }
  return <YesNo value={display === "Yes"} notApplicable={notApplicable} />;
}

/**
 * A long-form plan section. The value is ServiceNow rich-text HTML, so it's
 * sanitized and rendered as HTML. Renders nothing when the field is empty or
 * has no visible content.
 */
function PlanSection({ title, html }: { title: string; html?: string | null }): JSX.Element | null {
  if (!html || isBlankHtml(html)) return null;
  const safeHtml = sanitizeRichTextHtml(html);
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
      <Typography variant="subtitle2">{title}</Typography>
      <Box
        sx={{
          color: "text.secondary",
          fontSize: "0.875rem",
          lineHeight: 1.5,
          wordBreak: "break-word",
          // Newly generated comments no longer carry a per-run
          // `white-space: pre-wrap` inline style (digiops-cs#2933) — declared
          // once here instead. Older comments carry their own inline style
          // and are unaffected either way.
          whiteSpace: "pre-wrap",
          "& p": { my: 0.5 },
          "& p:first-of-type": { mt: 0 },
          "& p:last-child": { mb: 0 },
          "& ul, & ol": { my: 0.5, pl: 3 },
          "& a": { color: "primary.main" },
          "& img": { maxWidth: "100%", height: "auto" },
          "& table": { borderCollapse: "collapse", width: "100%" },
          "& th, & td": { border: 1, borderColor: "divider", px: 1, py: 0.5, textAlign: "left" },
        }}
        dangerouslySetInnerHTML={{ __html: safeHtml }}
      />
    </Box>
  );
}

type ChangeRequestTabId = "approval" | "plan" | "comments" | "attachments";

const TAB_DEFS: Array<{
  id: ChangeRequestTabId;
  label: string;
  icon: JSX.Element;
}> = [
  { id: "approval", label: "Approval", icon: <ClipboardCheck size={16} /> },
  { id: "plan", label: "Plan", icon: <FileText size={16} /> },
  { id: "comments", label: "Comments", icon: <MessageSquare size={16} /> },
  { id: "attachments", label: "Attachments", icon: <Paperclip size={16} /> },
];
const CHANGE_REQUEST_TAB_IDS: readonly ChangeRequestTabId[] = TAB_DEFS.map((t) => t.id);

/**
 * Read-only detail for a single change request (`GET /change-requests/{id}`):
 * its references, the change window, approval state, and the implementation /
 * rollback / test / communication plans.
 */
export default function CsmChangeRequestDetailPage(): JSX.Element {
  // Real router hooks — called unconditionally regardless of `routeOverride`
  // below (rules of hooks), but their VALUES are only actually used when
  // this instance isn't part of an open in-app tab. See the identical
  // pattern (and its own longer doc comment) at the top of
  // `CsmCaseDetailPage`, which this mirrors: this page can be mounted
  // several times at once (one per open tab, kept alive in the background —
  // see `CaseTabIsolatedRouter`), while there is only ever one real matched
  // route/location for the app as a whole.
  // UX only — the backend 403s attachment downloads the same regardless of
  // this flag, so hiding the control here is never the enforcement.
  const { canDownloadAttachment } = usePortalAccess();
  const routedId = useNormalizedIdParam("id");
  const routedNavigate = useNavTransition();
  const routedLocationState = useLocation().state;
  const routeOverride = useCaseRouteOverride();
  const id = routeOverride?.caseId ?? routedId;
  const navigate = routeOverride?.navigate ?? routedNavigate;
  // Prefer the list URL the row link captured (if any) so "back" returns to
  // the exact view the engineer came from, falling back to the bare tab path
  // for a bookmarked or directly-linked change request.
  const backState = (routeOverride ? routeOverride.state : routedLocationState) as
    | { from?: string }
    | undefined;
  const backTarget = backState?.from ?? OPERATIONS_CR_PATH;
  const { data, isLoading, isError, refetch } = useGetChangeRequest(id);
  // Same label computation this page's own `recordView` call below uses —
  // The CR number as the short chip label (matching `CsmCaseDetailPage`'s
  // own `caseNumber`-only report); change requests have no separate
  // project-scoped id the way cases do, so the tooltip's `internalId` reuses
  // the same number, with the subject alongside it.
  useReportCaseTabMeta(id, {
    label: data?.number ?? undefined,
    internalId: data?.number ?? undefined,
    subject: data?.subject ?? undefined,
  });
  // Fetched here (not just inside the Approval tab's `ChangeRequestApprovals`)
  // so the header's blocking-reason note has data on first render, even when
  // the engineer lands on a different tab. Both call sites share the same
  // query key, so react-query dedupes this into a single request rather than
  // fetching twice.
  const { data: approvalsData, isFetching: approvalsFetching } = useGetChangeRequestApprovals(id);
  const { showError } = useErrorBanner();
  const { user } = useCurrentUser();
  const patchCr = usePatchChangeRequest();
  const [editOpen, setEditOpen] = useState(false);
  // Kept in the URL (`?tab=`), not local state, so a shared/bookmarked link
  // to a specific tab survives a refresh.
  const { activeTab, setActiveTab } = useQueryParamTabs<ChangeRequestTabId>(
    CHANGE_REQUEST_TAB_IDS,
    "approval",
  );
  const engineerName = useEngineerDisplayName();

  const {
    data: comments,
    isLoading: isCommentsLoading,
    isError: isCommentsError,
  } = useGetCsmChangeRequestComments(id);
  const postComment = usePostCsmChangeRequestComment();
  const patchComment = usePatchComment();
  const deleteComment = useDeleteComment();
  const onEditComment = useCallback(
    (commentId: string, content: string) =>
      patchComment.mutateAsync({
        commentId,
        content,
        invalidateQueryKey: [ApiQueryKeys.CHANGE_REQUEST_COMMENTS, id],
      }),
    [patchComment, id],
  );
  const onDeleteComment = useCallback(
    (commentId: string) =>
      deleteComment.mutateAsync({
        commentId,
        invalidateQueryKey: [ApiQueryKeys.CHANGE_REQUEST_COMMENTS, id],
      }),
    [deleteComment, id],
  );
  const { data: attachments } = useGetCsmCaseAttachments(id, "change_request");
  const postAttachment = usePostCsmCaseAttachment();
  const downloadAttachment = useDownloadCsmCaseAttachment();
  const [composerOpen, setComposerOpen] = useState(false);
  // Reports composerOpen up to the in-app case-tabs layer, purely so closing
  // this change request's tab from the tab strip can confirm first — see
  // CsmCaseDetailPage's identical call, and the hook's own doc comment for
  // what this signal does and doesn't guarantee. Missing here was itself a
  // bug: this tab's `hasDraft` never became true, so its close-confirm
  // never fired for an unsent reply.
  useReportCaseTabDraft(id, composerOpen);
  // Transition awaiting a reason (`rollback`/`canceled`), the inline error for
  // that attempt, and whether its reason comment already landed — the last
  // one so a retry after a failed patch re-sends only the state change
  // instead of duplicating the comment.
  const [reasonTransition, setReasonTransition] = useState<{ target: string } | null>(null);
  const [reasonError, setReasonError] = useState<string | null>(null);
  const [reasonRecorded, setReasonRecorded] = useState(false);
  // Re-schedule / counter-proposal (`{state: "authorize"}` out of Customer Approval,
  // which never moves the state) collects the new planned window first; same
  // shape as the reason dialog above. The change request and the proposal the
  // dialog was opened on are kept as they were when it opened (a snapshot):
  // what the engineer answers is what they were shown, and the page refetching
  // behind the dialog (after a refusal, say) neither moves its pickers nor
  // swaps the proposal under their hands. The backend refuses a moved proposal or
  // window in words, and the dialog shows them. A refusal that names one of the
  // stale-answer codes (see `isStaleAnswerError`) closes the dialog instead, and the
  // page says why (`staleNotice`): the same request would be refused again. The optional
  // reason is recorded only after the change has been updated, so no attempt, retry or
  // reopened dialog can leave a duplicate note.
  const [reschedule, setReschedule] = useState<{
    cr: BeChangeRequestDetail;
    proposal: BeChangeRequestCustomerProposal | null;
  } | null>(null);
  const [rescheduleError, setRescheduleError] = useState<string | null>(null);
  // "Accept proposed time": its confirmation, on the same kind of snapshot.
  const [accept, setAccept] = useState<{
    cr: BeChangeRequestDetail;
    proposal: BeChangeRequestCustomerProposal;
  } | null>(null);
  const [acceptError, setAcceptError] = useState<string | null>(null);
  // Why an answer dialog was closed for the engineer (what it showed is no longer what is stored). It is
  // an alert on the page, and takes focus once the dialog is gone: the button that opened the dialog is
  // often not there any more (the proposal was answered, or it moved), and focus would otherwise drop
  // to the document body.
  const [staleNotice, setStaleNotice] = useState<string | null>(null);
  const staleNoticeRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (staleNotice) staleNoticeRef.current?.focus();
  }, [staleNotice]);

  const attachmentList = useMemo(() => attachments ?? [], [attachments]);

  const recordView = useRecordRecentView();
  useEffect(() => {
    if (!data?.id) return;
    recordView({
      kind: "change_request",
      id: data.id,
      title:
        [data.number, data.subject].filter((s): s is string => !!s?.trim()).join(" · ") ||
        "(no subject)",
      subtitle: data.project?.name,
      href: `/operations/change-requests/${data.id}`,
    });
  }, [data, recordView]);

  const onUploadAttachment = useCallback(
    (file: File) => {
      if (!id) return;
      postAttachment.mutate({
        caseId: id,
        file,
        uploadedBy: engineerName,
        referenceType: "change_request",
      });
    },
    [id, engineerName, postAttachment],
  );

  const onDownloadAttachment = useCallback(
    (attachment: (typeof attachmentList)[number]) => {
      void downloadAttachment(attachment).catch((err) =>
        showError(`Could not download ${attachment.filename}.`, err),
      );
    },
    [downloadAttachment, showError],
  );

  const back = (): void => {
    navigate(backTarget);
  };

  const BackButton = (
    <Button
      variant="text"
      size="small"
      className="csm-print-hide"
      startIcon={<ArrowLeft size={16} />}
      onClick={back}
      sx={{ alignSelf: "flex-start" }}
    >
      Back
    </Button>
  );

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rounded" height={32} width={240} />
        <Skeleton variant="rounded" height={260} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="body1" color="error">
          Could not load change request {id}.
        </Typography>
      </Box>
    );
  }

  if (!data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="h5">Change request not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No change request with id <code>{id}</code>.
        </Typography>
      </Box>
    );
  }

  const cr = data;
  // The customer's proposed time while it waits for WSO2's answer (the backend's own verdict).
  const proposal = pendingCustomerProposal(cr);
  // An Emergency change acts without customer consent: its customer part reads "Not applicable" (a record that
  // shows it went through a customer gate, from before that rule, is shown as it is).
  const approvalNotApplicable = isCustomerStepNotApplicable(cr, "approval", approvalsData?.approvals);
  const reviewNotApplicable = isCustomerStepNotApplicable(cr, "review", approvalsData?.approvals);
  // The creator can't approve/reject any stage (backend-enforced); they can
  // still cancel, which the action bar offers via `legalNextStates` as usual.
  const isCreator = isChangeRequestCreator(cr, user);

  const handleExportChangeRequestPdf = async (): Promise<void> => {
    try {
      const { generateChangeRequestReportPdf } = await import(
        "@features/csm-operations/utils/changeRequestReportPdf"
      );
      generateChangeRequestReportPdf(cr, comments ?? []);
    } catch (err) {
      showError("Could not export this change request as a PDF. Please try again.", err);
    }
  };
  // Only meaningful while the CR is actively moving through approval —
  // closed/canceled/rollback are terminal or off-ramp states where "awaiting
  // approval" no longer describes what's happening.
  const blockingReason =
    cr.state === "closed" || cr.state === "canceled" || cr.state === "rollback"
      ? null
      : changeRequestBlockingReason(approvalsData?.approvals, cr.state, proposal);
  // At a customer gate nobody is being asked to answer when the project has no
  // registered contacts (the backend had no one to assign the stage to), and
  // also when it has some but none has a request waiting: only the requester,
  // contacts no longer active, or a legacy change with no stage at all. The
  // second case needs the approvals, and not while they are being reloaded (a
  // state change refetches them after the detail, so the old rows would read
  // as "nobody is waiting" for a moment). `customerContacts` absent from the
  // payload (another data source) yields null, so nothing is claimed.
  const noCustomerGroupNote = noCustomerAskedHelper(
    cr.state,
    cr.customerContacts,
    approvalsFetching ? undefined : approvalsData?.approvals,
  );
  // The customer's review the change is waiting for, if any, from the same
  // approval stages as the note above. The action bar uses it to show Roll back
  // disabled, with who the review is waiting on, instead of leaving it out (a
  // failed review is the customer's to give in the Customer Portal). `null`
  // until the approvals load.
  const customerReviewPending = pendingCustomerReview(approvalsData?.approvals, cr.state);
  // A transition is in flight whenever either half of a transition that needs
  // a reason (the reason comment, then the patch) or a plain patch is
  // running, so the bar stays disabled across both and a double-click can't
  // fire two transitions.
  const transitionPending = patchCr.isPending || postComment.isPending;

  const openReschedule = (): void => {
    setRescheduleError(null);
    setStaleNotice(null);
    setReschedule({ cr, proposal });
  };

  const openAccept = (): void => {
    // Only a time a customer is recorded as having proposed can be accepted (the banner disables the button otherwise).
    if (!proposal || !customerProposalProposer(proposal)) return;
    setAcceptError(null);
    setStaleNotice(null);
    setAccept({ cr, proposal });
  };

  // After a refused attempt, whether the page now holds something other than what each dialog was opened on:
  // the same request would be refused again, so the dialog holds submit back (see its `stale` prop).
  const rescheduleStale =
    !!reschedule && !!rescheduleError && answerSnapshotMoved(reschedule, { cr, proposal });
  const acceptStale = !!accept && !!acceptError && answerSnapshotMoved(accept, { cr, proposal });

  /**
   * The dialog's own refusal was one of the stale-answer codes: what it showed is no longer what is stored,
   * and the same request would be refused again. Closes both answer dialogs, says why on the page and reads
   * the change request again so the page shows the truth (the patch hook invalidates the detail and the
   * approvals when it settles; this read joins that one rather than starting another).
   */
  const closeAnswerDialogsAsStale = (err: unknown, fallback: string): void => {
    setReschedule(null);
    setRescheduleError(null);
    setAccept(null);
    setAcceptError(null);
    setStaleNotice(`${asSentence(backendErrorMessage(err, fallback))} The page now shows the current state.`);
    void refetch({ cancelRefetch: false });
  };

  /**
   * Apply `target` to this change request. Targets that need a reason (the
   * destructive ones) are diverted into the confirmation dialog first — see `confirmReasonTransition` for the
   * comment-then-patch ordering they then follow.
   */
  const onTransition = (target: string): void => {
    // `authorize` is only offered as Re-schedule (or, with a customer's proposal waiting, as
    // "Propose a different time"), which needs the new window.
    if (target === "authorize") {
      openReschedule();
      return;
    }
    if (changeRequestTransitionRequiresReason(target)) {
      setReasonError(null);
      setReasonRecorded(false);
      setReasonTransition({ target });
      return;
    }
    patchCr.mutate(
      { id: cr.id, patch: buildTransitionPatch(target) },
      {
        onError: (err) =>
          showError(backendErrorMessage(err, transitionFallbackMessage(target)), err),
      },
    );
  };

  /**
   * Confirmed transition that needs a reason. The reason is recorded as an
   * ordinary comment *before* the state changes, deliberately in that order: the PATCH
   * contract carries no reason field, and a silent unexplained rollback or
   * cancellation is worse than a failed one. So a failed comment aborts
   * without touching the state.
   *
   * The reverse failure (comment recorded, patch rejected) is not rolled back
   * — there is no comment-delete endpoint — so it reports exactly that, and
   * `reasonRecorded` keeps the already-saved reason from being posted twice on
   * a retry.
   *
   * Posted as an internal work note rather than a customer-visible comment:
   * whether a rollback/cancellation reason should be shown to the
   * customer hasn't been decided, and a work note is the choice that can't leak.
   */
  const confirmReasonTransition = async (reason: string): Promise<void> => {
    const target = reasonTransition?.target;
    if (!target) return;
    setReasonError(null);

    if (!reasonRecorded) {
      try {
        await postComment.mutateAsync({
          changeRequestId: cr.id,
          // Posted verbatim, as plain text with real line breaks. The
          // backing store for these notes is a plain-text journal field, not
          // an HTML one: a sample of production entries carries raw newlines
          // and no escaped entities, so wrapping the reason in markup would
          // show literal tags to anyone reading the record at the source.
          // The portal's own renderer treats the note as HTML, which renders
          // `<` and line breaks imperfectly here; that mismatch is
          // pre-existing, applies equally to notes authored outside the
          // portal, and is being fixed on the render path, not by re-encoding
          // on the way in.
          bodyHtml: reason,
          internal: true,
        });
        setReasonRecorded(true);
      } catch (err) {
        setReasonError(
          backendErrorMessage(
            err,
            "Could not record the reason, so the state was left unchanged. Try again.",
          ),
        );
        return;
      }
    }

    try {
      await patchCr.mutateAsync({ id: cr.id, patch: buildTransitionPatch(target) });
      setReasonTransition(null);
      setReasonRecorded(false);
    } catch (err) {
      setReasonError(
        `Your reason was recorded as an internal note, but the state did not change: ${backendErrorMessage(
          err,
          transitionFallbackMessage(target),
        )} You don't need to retype it.`,
      );
    }
  };

  /**
   * Confirmed Re-schedule (or counter, or decline): the state + new planned window are patched, and the
   * optional reason is recorded as an internal comment once that has gone through. After, not before: a
   * refused attempt leaves no note behind, so a retry, or a dialog closed and opened again, can never post
   * it twice. If the note then cannot be saved the change is already updated: the page says so, and the
   * reason can be added as a comment. The backend's refusal (e.g. "re-scheduling requires a changed planned
   * start or end") is shown in the dialog as returned, except one that means the change is no longer what the
   * dialog showed: that closes it (see `closeAnswerDialogsAsStale`).
   */
  const confirmReschedule = async (
    patch: BePatchChangeRequestPayload,
    reason: string,
  ): Promise<void> => {
    setRescheduleError(null);
    const answeredProposal = !!reschedule?.proposal;
    try {
      await patchCr.mutateAsync({ id: cr.id, patch });
    } catch (err) {
      const fallback = answeredProposal
        ? "Could not answer the proposed time."
        : "Could not re-schedule this change request.";
      if (isStaleAnswerError(err)) closeAnswerDialogsAsStale(err, fallback);
      else setRescheduleError(backendErrorMessage(err, fallback));
      return;
    }
    if (reason) {
      try {
        await postComment.mutateAsync({ changeRequestId: cr.id, bodyHtml: reason, internal: true });
      } catch (err) {
        showError(
          `The change request was updated, but your reason could not be recorded as an internal note (${backendErrorMessage(
            err,
            "the request failed",
          )}). Add it as a comment instead.`,
          err,
        );
      }
    }
    setReschedule(null);
  };

  /**
   * Accept the customer's proposed time (the previous system's "Agree"): the backend applies the proposal to
   * the planned window and moves the change straight to Scheduled in one step. What is sent is the
   * proposal and the planned window the dialog showed, so a proposal or window that moved behind it
   * is refused in words instead of accepting a time its reader never saw: shown in the dialog, or, when
   * the refusal means the change is no longer what the dialog showed, on the page after the dialog closes.
   */
  const confirmAccept = async (): Promise<void> => {
    if (!accept) return;
    setAcceptError(null);
    const { cr: shown, proposal: proposed } = accept;
    try {
      await patchCr.mutateAsync({
        id: cr.id,
        patch: {
          confirmCustomerUpdatedDate: "agree",
          expectedCustomerUpdatedOn: proposed.startOn,
          ...(shown.plannedStartOn ? { expectedPlannedStartOn: shown.plannedStartOn } : {}),
          ...(shown.plannedEndOn ? { expectedPlannedEndOn: shown.plannedEndOn } : {}),
        },
      });
      setAccept(null);
    } catch (err) {
      const fallback = "Could not accept the proposed time.";
      if (isStaleAnswerError(err)) closeAnswerDialogsAsStale(err, fallback);
      else setAcceptError(backendErrorMessage(err, fallback));
    }
  };

  // Opens the create form pre-filled from this record, so promoting the same
  // change to another environment doesn't mean re-typing every field. Router
  // state (not a query string) carries the values across — same pattern as
  // "Create incident from case" — and the result is a new, independent change
  // request: nothing here links it back to `cr`. See
  // buildCloneChangeRequestNavState's doc comment for exactly which fields
  // can and can't be carried over today.
  const cloneChangeRequest = (): void => {
    navigate("/operations/change-requests/new", {
      state: buildCloneChangeRequestNavState(cr),
    });
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        {BackButton}
        <ExportPdfButton
          onExport={handleExportChangeRequestPdf}
          disabled={isCommentsLoading || isCommentsError}
        />
      </Box>

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
          <Typography
            variant="h6"
            sx={{
              fontFamily: "monospace",
              fontWeight: 700,
              letterSpacing: 0.2,
              lineHeight: 1.2,
            }}
          >
            {cr.number || cr.id}
          </Typography>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
            {cr.state && (
              <Chip
                size="small"
                color={changeRequestStateColor(cr.state)}
                label={changeRequestStateLabel(cr.state)}
              />
            )}
            {cr.impact && (
              <Chip
                size="small"
                variant="outlined"
                color={changeRequestImpactColor(cr.impact)}
                label={`${changeRequestImpactLabel(cr.impact)} impact`}
              />
            )}
            {blockingReason && (
              <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                <Clock size={14} />
                <Typography variant="body2" color="text.secondary">
                  {blockingReason}
                </Typography>
              </Box>
            )}
          </Box>
          <Typography variant="h5">{cr.subject || "Change request"}</Typography>
        </Box>
        <Box sx={{ flexShrink: 0, alignSelf: { xs: "stretch", md: "flex-start" } }}>
          <Box className="csm-print-hide" sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <ChangeRequestActionBar
              cr={cr}
              isPending={transitionPending}
              pendingCustomerReview={customerReviewPending}
              onAction={onTransition}
            />
            <Button
              variant="outlined"
              size="small"
              startIcon={<CopyPlus size={14} />}
              onClick={cloneChangeRequest}
              sx={{ flexShrink: 0 }}
            >
              Clone
            </Button>
            <Button
              variant="outlined"
              size="small"
              startIcon={<Pencil size={14} />}
              onClick={() => {
                // Clear any error left over from a previous save (or from a
                // lifecycle transition, which shares this mutation) so a
                // stale rejection doesn't appear to belong to this edit.
                patchCr.reset();
                setEditOpen(true);
              }}
              sx={{ flexShrink: 0 }}
            >
              Edit
            </Button>
          </Box>
        </Box>
      </Box>

      {staleNotice && (
        <Alert
          ref={staleNoticeRef}
          severity="warning"
          role="alert"
          tabIndex={-1}
          className="csm-print-hide"
          onClose={() => setStaleNotice(null)}
        >
          {staleNotice}
        </Alert>
      )}

      {/* Full width, under the header: eleven stages need more room than the
          header's left block leaves beside the action bar. */}
      <ChangeRequestLifecycleStepper
        state={cr.state}
        customerApprovalRequired={cr.customerApprovalRequired}
        customerReviewRequired={cr.customerReviewRequired}
        type={cr.type}
        approvals={approvalsData?.approvals}
        customerApproved={cr.hasCustomerApproved}
        hasCustomerContacts={cr.customerContacts ? cr.customerContacts.length > 0 : undefined}
      />

      {proposal && (
        <Box className="csm-print-hide">
          <ChangeRequestProposedTimeBanner
            cr={cr}
            proposal={proposal}
            isPending={transitionPending}
            onAccept={openAccept}
            onProposeDifferent={openReschedule}
          />
        </Box>
      )}

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Overview</Typography>
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, minmax(0, 1fr))",
              md: "repeat(3, minmax(0, 1fr))",
            },
          }}
        >
          <MetaCell label="Customer Project"><RefText value={cr.project} /></MetaCell>
          <MetaCell label="Type">
            <Typography variant="body2">{cr.type || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Linked case"><EntityRefLink value={cr.case} routeBase="/cases" /></MetaCell>
          <MetaCell label="Impact">
            {cr.impact ? (
              <Chip
                size="small"
                variant="outlined"
                color={changeRequestImpactColor(cr.impact)}
                label={changeRequestImpactLabel(cr.impact)}
              />
            ) : (
              <Typography variant="body2">—</Typography>
            )}
          </MetaCell>
          <MetaCell label="Deployment"><RefText value={cr.deployment} /></MetaCell>
          <MetaCell label="Deployed product"><RefText value={cr.deployedProduct} /></MetaCell>
          <MetaCell label="Product"><RefText value={cr.product} /></MetaCell>
          <MetaCell label="Deployments"><RefChips values={cr.deployments} /></MetaCell>
          <MetaCell label="Deployment products"><RefChips values={cr.deploymentProducts} /></MetaCell>
          <MetaCell label="Customer group"><RefChips values={cr.customerContacts} /></MetaCell>
          <MetaCell label="Category">
            <Typography variant="body2">{changeRequestCategoryLabel(cr.category)}</Typography>
          </MetaCell>
          <MetaCell label="Assigned engineer"><RefText value={cr.assignedEngineer} /></MetaCell>
          <MetaCell label="Assigned team"><RefText value={cr.assignedTeam} /></MetaCell>
          <MetaCell label="Duration">
            <Typography variant="body2">{cr.duration || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Planned start">
            <Typography variant="body2">{formatDateTime(cr.plannedStartOn)}</Typography>
          </MetaCell>
          <MetaCell label="Planned end">
            <Typography variant="body2">{formatDateTime(cr.plannedEndOn)}</Typography>
          </MetaCell>
          <MetaCell label="Created">
            <Typography variant="body2">{formatDateTime(cr.createdOn)}</Typography>
          </MetaCell>
          <MetaCell label="Last updated">
            <Typography variant="body2">{formatDateTime(cr.updatedOn)}</Typography>
          </MetaCell>
          <MetaCell label="Created by">
            <Typography variant="body2">{cr.createdBy || "—"}</Typography>
          </MetaCell>
        </Box>
      </Card>

      <Box className="csm-print-hide" sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={activeTab}
          onChange={(_, v) => setActiveTab(v as ChangeRequestTabId)}
          variant="scrollable"
          scrollButtons="auto"
        >
          {TAB_DEFS.map((t) => {
            // Counts shown only where the tab IS the list (unambiguous) —
            // mirrors CsmCaseDetailPage's tab-count pattern.
            const count =
              t.id === "comments"
                ? comments?.length
                : t.id === "attachments"
                  ? attachmentList.length
                  : undefined;
            return (
              <Tab
                key={t.id}
                value={t.id}
                icon={t.icon}
                iconPosition="start"
                label={count ? `${t.label} (${count})` : t.label}
                sx={{ minHeight: 44, textTransform: "none" }}
              />
            );
          })}
        </Tabs>
      </Box>

      {activeTab === "approval" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            <Typography
              variant="overline"
              color="text.secondary"
              sx={{ letterSpacing: 0.6 }}
            >
              Customer approval
            </Typography>
            <Card
              sx={{
                p: 2.5,
                display: "flex",
                flexDirection: "column",
                gap: 2,
                borderLeft: 3,
                borderColor: "info.main",
              }}
            >
              <Typography variant="body2" color="text.secondary">
                Whether this change requires customer approval and review, and what the
                customer has confirmed on it.
              </Typography>
              <Box
                sx={{
                  display: "grid",
                  gap: 2,
                  gridTemplateColumns: {
                    xs: "1fr",
                    sm: "repeat(2, minmax(0, 1fr))",
                    md: "repeat(3, minmax(0, 1fr))",
                  },
                }}
              >
                <MetaCell label="Customer approval required">
                  <YesNo value={cr.customerApprovalRequired} notApplicable={approvalNotApplicable} />
                </MetaCell>
                <MetaCell label="Customer review required">
                  <YesNo value={cr.customerReviewRequired} notApplicable={reviewNotApplicable} />
                </MetaCell>
                <MetaCell label="Customer approved">
                  <CustomerApprovedValue cr={cr} notApplicable={approvalNotApplicable} />
                </MetaCell>
                <MetaCell label="Customer reviewed">
                  <YesNo value={cr.hasCustomerReviewed} notApplicable={reviewNotApplicable} />
                </MetaCell>
                <MetaCell label="Approved by"><RefText value={cr.approvedBy} /></MetaCell>
                <MetaCell label="Approved on">
                  <Typography variant="body2">{formatDateTime(cr.approvedOn)}</Typography>
                </MetaCell>
              </Box>
            </Card>
          </Box>

          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            <Typography
              variant="overline"
              color="text.secondary"
              sx={{ letterSpacing: 0.6 }}
            >
              Approval workflow
            </Typography>
            {noCustomerGroupNote && (
              <Alert severity="info" sx={{ mb: 0.5 }}>
                {noCustomerGroupNote}
              </Alert>
            )}
            <ChangeRequestApprovals id={cr.id} isCreator={isCreator} customerContacts={cr.customerContacts} />
          </Box>
        </Box>
      )}

      {activeTab === "plan" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
          {/*
            Field order here mirrors the SRE change-review packet's reading
            order (description/justification first, then the impact and
            rollback/test detail, then the two fields the field-usage census
            found most often left "N/A" — service outage and communication
            plan — grouped last so a mostly-empty CR doesn't bury the fields
            that usually carry real content), plus the field-parity fields
            added 2026-08-20 (`implementationPlan`, the affected-services/
            components text, and the rollback-duration text) appended after
            them — see `CHANGES-cr-field-parity.md`.
          */}
          {[
            cr.description,
            cr.justification,
            cr.impactDescription,
            cr.rollbackPlan,
            cr.testPlan,
            cr.serviceOutage,
            cr.communicationPlan,
            cr.implementationPlan,
            cr.affectedServicesText,
            cr.affectedComponentsText,
          ].some((v) => v && !isBlankHtml(v)) ||
          cr.rollbackDurationText ? (
            <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2.5 }}>
              <Typography variant="subtitle2">Plan</Typography>
              <PlanSection title="Description" html={cr.description} />
              <PlanSection title="Justification" html={cr.justification} />
              <PlanSection title="Impact description" html={cr.impactDescription} />
              <PlanSection title="Rollback plan" html={cr.rollbackPlan} />
              <PlanSection title="Test plan" html={cr.testPlan} />
              <PlanSection title="Service outage" html={cr.serviceOutage} />
              <PlanSection title="Communication plan" html={cr.communicationPlan} />
              <PlanSection title="Implementation plan" html={cr.implementationPlan} />
              <PlanSection title="Affected services" html={cr.affectedServicesText} />
              <PlanSection title="Affected components" html={cr.affectedComponentsText} />
              {cr.rollbackDurationText && (
                <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
                  <Typography variant="subtitle2">Rollback duration</Typography>
                  <Typography variant="body2" color="text.secondary">
                    {cr.rollbackDurationText}
                  </Typography>
                </Box>
              )}
            </Card>
          ) : (
            <Typography variant="body2" color="text.secondary">
              No plan has been recorded for this change request.
            </Typography>
          )}

          {/*
            Read-only SRE metadata (`CHANGES-cr-field-parity.md`'s "group
            C2"/"group D"). Project / deployments / deployment
            products / customer group (the project's registered contacts) / category are shown (and editable) in
            the Overview above; the rest here have no write path anywhere in
            the stack yet (`EditChangeRequestDialog`'s doc comment on
            `BePatchChangeRequestPayload` explains why each is missing).
          */}
          <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
            <Typography variant="subtitle2">SRE details</Typography>
            <Box
              sx={{
                display: "grid",
                gap: 2,
                gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0, 1fr))", md: "repeat(3, minmax(0, 1fr))" },
              }}
            >
              <MetaCell label="Priority">
                <Typography variant="body2">{cr.priority?.label || "—"}</Typography>
              </MetaCell>
              <MetaCell label="Requested by"><RefText value={cr.requestedBy} /></MetaCell>
              <MetaCell label="Change request type">
                <Typography variant="body2">{cr.changeRequestType?.label || "—"}</Typography>
              </MetaCell>
              <MetaCell label="Likelihood">
                <Typography variant="body2">{cr.likelihood?.label || "—"}</Typography>
              </MetaCell>
              <MetaCell label="Implementation Plan visible to customers">
                <YesNo value={cr.isPlanningVisibleToCustomers} />
              </MetaCell>
              <MetaCell label="Customer updated">
                <Typography variant="body2">{formatDateTime(cr.customerUpdatedOn)}</Typography>
              </MetaCell>
              <MetaCell label="WSO2 answer to the customer's time">
                <Typography variant="body2">{wso2AnswerLabel(cr.confirmCustomerUpdatedDate)}</Typography>
              </MetaCell>
              <MetaCell label="Work start">
                <Typography variant="body2">{formatDateTime(cr.workStart)}</Typography>
              </MetaCell>
              <MetaCell label="Work end">
                <Typography variant="body2">{formatDateTime(cr.workEnd)}</Typography>
              </MetaCell>
              <MetaCell label="Git reference">
                {cr.gitReference && isHttpUrl(cr.gitReference) ? (
                  <Typography variant="body2" sx={{ wordBreak: "break-all" }}>
                    <a href={cr.gitReference} target="_blank" rel="noreferrer">
                      {cr.gitReference}
                    </a>
                  </Typography>
                ) : (
                  <Typography variant="body2" sx={{ wordBreak: "break-all" }}>
                    {cr.gitReference || "—"}
                  </Typography>
                )}
              </MetaCell>
            </Box>
            {!!cr.labels?.length && (
              <MetaCell label="Labels">
                <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.75 }}>
                  {cr.labels.map((label) => (
                    <Chip key={label} size="small" label={label} />
                  ))}
                </Box>
              </MetaCell>
            )}
          </Card>
        </Box>
      )}

      {activeTab === "comments" && (
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
          {composerOpen ? (
            <Box className="csm-print-hide" sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
              <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
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
                disabled={!id}
                publicCommentDisabledReason={changeRequestCommentGateReason(cr.state)}
                autoFocus
                onSubmit={async (bodyHtml, internal, commentAttachments) => {
                  if (!id) return;
                  const hasText =
                    bodyHtml.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim().length > 0;
                  if (hasText) {
                    await postComment.mutateAsync({
                      changeRequestId: id,
                      bodyHtml,
                      internal,
                    });
                  }
                  for (const { file, name } of commentAttachments) {
                    await postAttachment.mutateAsync({
                      caseId: id,
                      file,
                      name,
                      uploadedBy: engineerName,
                      referenceType: "change_request",
                    });
                  }
                  setComposerOpen(false);
                }}
              />
            </Box>
          ) : (
            <Button
              fullWidth
              variant="outlined"
              color="inherit"
              className="csm-print-hide"
              startIcon={<MessageSquarePlus size={18} />}
              onClick={() => setComposerOpen(true)}
              sx={{ justifyContent: "flex-start", textTransform: "none", py: 1.5, px: 2 }}
            >
              Add a comment…
            </Button>
          )}
          <CaseActivitiesFeed
            comments={comments ?? []}
            audit={[]}
            attachments={[]}
            onEditComment={onEditComment}
            onDeleteComment={onDeleteComment}
          />
        </Card>
      )}

      {activeTab === "attachments" && (
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
          <AttachmentsWidget
            attachments={attachmentList}
            uploading={postAttachment.isPending}
            uploadError={
              postAttachment.isError
                ? (postAttachment.error?.message ?? "Could not upload the attachment.")
                : null
            }
            onUpload={onUploadAttachment}
            onDownload={canDownloadAttachment ? onDownloadAttachment : undefined}
          />
        </Card>
      )}

      {reasonTransition && (
        <ChangeRequestTransitionReasonDialog
          target={reasonTransition.target}
          isSubmitting={transitionPending}
          error={reasonError}
          reasonRecorded={reasonRecorded}
          onClose={() => {
            if (transitionPending) return;
            setReasonTransition(null);
            setReasonError(null);
            setReasonRecorded(false);
          }}
          onConfirm={(reason) => void confirmReasonTransition(reason)}
        />
      )}

      {reschedule && (
        <ChangeRequestRescheduleDialog
          cr={reschedule.cr}
          proposal={reschedule.proposal}
          isSubmitting={transitionPending}
          error={rescheduleError}
          stale={rescheduleStale}
          onClose={() => {
            if (transitionPending) return;
            setReschedule(null);
            setRescheduleError(null);
          }}
          onSubmit={(patch, reason) => void confirmReschedule(patch, reason)}
        />
      )}

      {accept && (
        <ChangeRequestAcceptProposedTimeDialog
          cr={accept.cr}
          proposal={accept.proposal}
          isSubmitting={transitionPending}
          error={acceptError}
          stale={acceptStale}
          onClose={() => {
            if (transitionPending) return;
            setAccept(null);
            setAcceptError(null);
          }}
          onConfirm={() => void confirmAccept()}
        />
      )}

      {editOpen && (
        <EditChangeRequestDialog
          cr={cr}
          isSaving={patchCr.isPending}
          saveError={
            patchCr.isError
              ? backendErrorMessage(
                  patchCr.error,
                  "Could not update the change request.",
                )
              : null
          }
          onClose={() => {
            if (!patchCr.isPending) setEditOpen(false);
          }}
          onSave={(patch) =>
            patchCr.mutate(
              { id: cr.id, patch },
              {
                onSuccess: () => setEditOpen(false),
                onError: (err) =>
                  showError(
                    backendErrorMessage(
                      err,
                      "Could not update the change request.",
                    ),
                    err,
                  ),
              },
            )
          }
        />
      )}
    </Box>
  );
}
