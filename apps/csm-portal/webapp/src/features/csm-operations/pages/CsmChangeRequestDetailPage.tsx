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
  useState,
} from "react";
import { useLocation } from "react-router";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import { BackendApiError } from "@api/backend/client";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useEngineerDisplayName } from "@hooks/useEngineerDisplayName";
import { useRecordRecentView } from "@features/csm-recent/hooks/useRecentViews";
import { useGetChangeRequest } from "@features/csm-operations/api/useGetChangeRequest";
import { useGetChangeRequestApprovals } from "@features/csm-operations/api/useGetChangeRequestApprovals";
import { usePatchChangeRequest } from "@features/csm-operations/api/usePatchChangeRequest";
import {
  useGetCsmChangeRequestComments,
  usePostCsmChangeRequestComment,
} from "@features/csm-operations/api/useCsmChangeRequestComments";
import ChangeRequestActionBar from "@features/csm-operations/components/ChangeRequestActionBar";
import ChangeRequestApprovals from "@features/csm-operations/components/ChangeRequestApprovals";
import ChangeRequestLifecycleStepper from "@features/csm-operations/components/ChangeRequestLifecycleStepper";
import ChangeRequestTransitionReasonDialog from "@features/csm-operations/components/ChangeRequestTransitionReasonDialog";
import EditChangeRequestDialog from "@features/csm-operations/components/EditChangeRequestDialog";
import EntityRefLink from "@features/csm-operations/components/EntityRefLink";
import {
  buildCloneChangeRequestNavState,
  changeRequestBlockingReason,
  changeRequestCommentGateReason,
  changeRequestTransitionRequiresReason,
  changeRequestImpactColor,
  changeRequestImpactLabel,
  changeRequestStateColor,
  changeRequestStateLabel,
} from "@features/csm-operations/utils/changeRequests";
import CaseActivitiesFeed from "@features/csm-cases/components/CaseActivitiesFeed";
import CsmCaseCommentInput from "@features/csm-cases/components/CsmCaseCommentInput";
import { AttachmentsWidget } from "@features/csm-cases/components/CaseDetailWidgets";
import {
  useGetCsmCaseAttachments,
  usePostCsmCaseAttachment,
  useDownloadCsmCaseAttachment,
} from "@features/csm-cases/api/useCsmCaseAttachments";
import type { BeEntityRef, BePatchChangeRequestPayload } from "@api/backend/types";
import { useNavTransition } from "@hooks/useNavTransition";
import { useNormalizedIdParam } from "@hooks/useNormalizedIdParam";
import { useCaseRouteOverride } from "@context/case-tabs/CaseRouteOverrideContext";
import { useReportCaseTabMeta } from "@features/case-tabs/hooks/useReportCaseTabMeta";
import { useReportCaseTabDraft } from "@features/case-tabs/hooks/useReportCaseTabDraft";
import { useQueryParamTabs } from "@hooks/useSectionTabs";

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
 * Every target goes through the generic `state` field except `assess`: the
 * New -> Assess move has its own `requestApproval` flag, which additionally
 * raises the approval request that setting `state` alone does not. Both are
 * accepted on the same endpoint; `state` is not mutually exclusive with
 * anything (only `isCustomerApproved`/`isCustomerReviewed`/`requestApproval`
 * are, with each other), but a transition is always sent on its own anyway so
 * a rejection can only ever be about the transition.
 */
function buildTransitionPatch(target: string): BePatchChangeRequestPayload {
  return target === "assess" ? { requestApproval: true } : { state: target };
}

/**
 * Fallback message for a failed transition, used only when the backend gave
 * no usable 4xx reason of its own.
 */
function transitionFallbackMessage(target: string): string {
  if (target === "assess") return "Could not request approval for this change request.";
  return `Could not move this change request to ${changeRequestStateLabel(target)}.`;
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

function RefText({ value }: { value?: BeEntityRef | null }): JSX.Element {
  return <Typography variant="body2">{value?.name || "—"}</Typography>;
}

function YesNo({ value }: { value?: boolean }): JSX.Element {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
      {value ? <Check size={14} /> : <X size={14} />}
      <Typography variant="body2">{value ? "Yes" : "No"}</Typography>
    </Box>
  );
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
  const { data, isLoading, isError } = useGetChangeRequest(id);
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
  const { data: approvalsData } = useGetChangeRequestApprovals(id);
  const { showError } = useErrorBanner();
  const patchCr = usePatchChangeRequest();
  const [editOpen, setEditOpen] = useState(false);
  // Kept in the URL (`?tab=`), not local state, so a shared/bookmarked link
  // to a specific tab survives a refresh.
  const { activeTab, setActiveTab } = useQueryParamTabs<ChangeRequestTabId>(
    CHANGE_REQUEST_TAB_IDS,
    "approval",
  );
  const engineerName = useEngineerDisplayName();

  const { data: comments } = useGetCsmChangeRequestComments(id);
  const postComment = usePostCsmChangeRequestComment();
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
  // Destructive transition awaiting confirmation (`rollback`/`canceled`), the
  // inline error for that attempt, and whether its reason comment already
  // landed — the last one so a retry after a failed patch re-sends only the
  // state change instead of duplicating the comment.
  const [reasonTarget, setReasonTarget] = useState<string | null>(null);
  const [reasonError, setReasonError] = useState<string | null>(null);
  const [reasonRecorded, setReasonRecorded] = useState(false);

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
  // Only meaningful while the CR is actively moving through approval —
  // closed/canceled/rollback are terminal or off-ramp states where "awaiting
  // approval" no longer describes what's happening.
  const blockingReason =
    cr.state === "closed" || cr.state === "canceled" || cr.state === "rollback"
      ? null
      : changeRequestBlockingReason(approvalsData?.approvals);
  // A transition is in flight whenever either half of a destructive
  // transition (the reason comment, then the patch) or a plain patch is
  // running, so the bar stays disabled across both and a double-click can't
  // fire two transitions.
  const transitionPending = patchCr.isPending || postComment.isPending;

  /**
   * Apply `target` to this change request. Destructive targets are diverted
   * into the confirmation dialog first — see `confirmReasonTransition` for
   * the comment-then-patch ordering they then follow.
   */
  const onTransition = (target: string): void => {
    if (changeRequestTransitionRequiresReason(target)) {
      setReasonError(null);
      setReasonRecorded(false);
      setReasonTarget(target);
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
   * Confirmed destructive transition. The reason is recorded as an ordinary
   * comment *before* the state changes, deliberately in that order: the PATCH
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
   * whether a rollback/cancellation reason should be shown to the customer
   * hasn't been decided, and a work note is the choice that can't leak.
   */
  const confirmReasonTransition = async (reason: string): Promise<void> => {
    const target = reasonTarget;
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
      setReasonTarget(null);
      setReasonRecorded(false);
    } catch (err) {
      setReasonError(
        `Your reason was recorded as a comment, but the state did not change: ${backendErrorMessage(
          err,
          transitionFallbackMessage(target),
        )} You don't need to retype it.`,
      );
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
      {BackButton}

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
          <ChangeRequestLifecycleStepper state={cr.state} />
        </Box>
        <Box sx={{ flexShrink: 0, alignSelf: { xs: "stretch", md: "flex-start" } }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <ChangeRequestActionBar
              cr={cr}
              isPending={transitionPending}
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
          <MetaCell label="Project"><RefText value={cr.project} /></MetaCell>
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

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
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
                What the customer has confirmed on this change.
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
                <MetaCell label="Customer approved"><YesNo value={cr.hasCustomerApproved} /></MetaCell>
                <MetaCell label="Customer reviewed"><YesNo value={cr.hasCustomerReviewed} /></MetaCell>
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
              Internal approval workflow
            </Typography>
            <ChangeRequestApprovals id={cr.id} />
          </Box>
        </Box>
      )}

      {activeTab === "plan" && (
        // Field order here mirrors the SRE change-review packet's reading
        // order (description/justification first, then the impact and
        // rollback/test detail, then the two fields the field-usage census
        // found most often left "N/A" — service outage and communication
        // plan — grouped last so a mostly-empty CR doesn't bury the fields
        // that usually carry real content). This is a relabel/reorder only:
        // the same seven fields the tab already showed, nothing added.
        [
          cr.description,
          cr.justification,
          cr.impactDescription,
          cr.rollbackPlan,
          cr.testPlan,
          cr.serviceOutage,
          cr.communicationPlan,
        ].some((v) => v && !isBlankHtml(v)) ? (
          <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2.5 }}>
            <Typography variant="subtitle2">Plan</Typography>
            <PlanSection title="Description" html={cr.description} />
            <PlanSection title="Justification" html={cr.justification} />
            <PlanSection title="Impact description" html={cr.impactDescription} />
            <PlanSection title="Rollback plan" html={cr.rollbackPlan} />
            <PlanSection title="Test plan" html={cr.testPlan} />
            <PlanSection title="Service outage" html={cr.serviceOutage} />
            <PlanSection title="Communication plan" html={cr.communicationPlan} />
          </Card>
        ) : (
          <Typography variant="body2" color="text.secondary">
            No plan has been recorded for this change request.
          </Typography>
        )
      )}

      {activeTab === "comments" && (
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
          {composerOpen ? (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
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
            onDownload={onDownloadAttachment}
          />
        </Card>
      )}

      {reasonTarget && (
        <ChangeRequestTransitionReasonDialog
          target={reasonTarget}
          isSubmitting={transitionPending}
          error={reasonError}
          reasonRecorded={reasonRecorded}
          onClose={() => {
            if (transitionPending) return;
            setReasonTarget(null);
            setReasonError(null);
            setReasonRecorded(false);
          }}
          onConfirm={(reason) => void confirmReasonTransition(reason)}
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
