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
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  Activity,
  ArrowLeft,
  CheckSquare,
  Clock,
  Eye,
  Layers,
  Link as LinkIcon,
  ListChecks,
  MessageSquarePlus,
  Paperclip,
  PauseCircle,
  Phone,
  X,
} from "@wso2/oxygen-ui-icons-react";
import { useCallback, useEffect, useMemo, useRef, useState, type JSX } from "react";
import { useLocation } from "react-router";
import { useGetCsmCaseDetail } from "@features/csm-cases/api/useGetCsmCaseDetail";
import {
  usePatchCsmCase,
  usePatchCsmCaseById,
} from "@features/csm-cases/api/usePatchCsmCase";
import {
  useFindMyOngoingCases,
  type MyOngoingCase,
} from "@features/csm-cases/api/useFindMyOngoingCases";
import type {
  BeCaseCause,
  BeCaseResolutionCode,
  BeCaseState,
  BeCaseUpdatePayload,
  BeCreateCaseGithubIssueResponse,
  BeCreateCaseTaskPayload,
} from "@api/backend/types";
import { beStateFromUi, priorityFromSeverity } from "@api/backend/mappers";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import { BackendApiError } from "@api/backend/client";
import {
  useGetCsmCaseComments,
  usePostCsmCaseComment,
} from "@features/csm-cases/api/useCsmCaseComments";
import { useGetCsmConversationMessages } from "@features/csm-cases/api/useCsmConversationMessages";
import { useGetCsmCaseActivities } from "@features/csm-cases/api/useCsmCaseActivities";
import { useGetCsmCaseFeedback } from "@features/csm-cases/api/useCsmCaseFeedback";
import {
  useGetCsmCaseAttachments,
  usePostCsmCaseAttachment,
  useDownloadCsmCaseAttachment,
  useDeleteCsmCaseAttachment,
  useGetCsmCaseAttachmentContent,
} from "@features/csm-cases/api/useCsmCaseAttachments";
import CsmCaseCommentInput from "@features/csm-cases/components/CsmCaseCommentInput";
import CaseActionBar, {
  canAcknowledge,
} from "@features/csm-cases/components/CaseActionBar";
import AssignEngineerDialog from "@features/csm-cases/components/AssignEngineerDialog";
import ResolutionDialog from "@features/csm-cases/components/ResolutionDialog";
import ChangeSeverityDialog from "@features/csm-cases/components/ChangeSeverityDialog";
import ChangeCaseTypeDialog, {
  type CaseTypeTransferSubmission,
} from "@features/csm-cases/components/ChangeCaseTypeDialog";
import { caseTypeTransferLabel } from "@features/csm-cases/utils/caseTypeTransfer";
import SetAutocloseHoldDialog from "@features/csm-cases/components/SetAutocloseHoldDialog";
import EditCaseDetailsDialog, {
  type FieldSaveResult,
} from "@features/csm-cases/components/EditCaseDetailsDialog";
import LinkIncidentDialog from "@features/csm-cases/components/LinkIncidentDialog";
import LinkCaseDialog, {
  type CaseLinkType,
} from "@features/csm-cases/components/LinkCaseDialog";
import SetFixEtaDialog, {
  type FixEtaSavePayload,
} from "@features/csm-cases/components/SetFixEtaDialog";
import RequestUpdateDialog, {
  type RequestUpdateSavePayload,
} from "@features/csm-cases/components/RequestUpdateDialog";
import { useRequestCaseUpdate } from "@features/csm-cases/api/useRequestCaseUpdate";
import { deriveCaseUpdateRequestCategory } from "@features/csm-cases/utils/caseUpdateRequests";
import CreateTaskDialog from "@features/csm-cases/components/CreateTaskDialog";
import AddTagDialog from "@features/csm-cases/components/AddTagDialog";
import { useCreateCaseTask } from "@features/csm-cases/api/useCreateCaseTask";
import { useAddCaseTag, useRemoveCaseTag } from "@features/csm-cases/api/useCaseTags";
import { ChildCasesWidget } from "@features/csm-cases/components/ChildCasesWidget";
import { LinkedServiceRequestsWidget } from "@features/csm-cases/components/LinkedServiceRequestsWidget";
import { LinkedChangeRequestsWidget } from "@features/csm-cases/components/LinkedChangeRequestsWidget";
import { LinkedIncidentWidget } from "@features/csm-cases/components/LinkedIncidentWidget";
import { CreateGithubIssueDialog } from "@features/csm-cases/components/CreateGithubIssueDialog";
import { isCloudSupportSubscription } from "@features/csm-projects/utils/subscriptionType";
import { usePostCaseGithubIssue } from "@features/csm-cases/api/useCsmCaseGithubIssue";
import CaseActivitiesFeed from "@features/csm-cases/components/CaseActivitiesFeed";
import { scrollToFragmentWithRetry } from "@features/csm-cases/utils/permalinkScroll";
import CaseMetaBand from "@features/csm-cases/components/CaseMetaBand";
import RefreshButton from "@components/RefreshButton";
import {
  AttachmentsWidget,
  CustomerContextWidget,
  ProductContextWidget,
  RequestDetailsWidget,
  TagsWidget,
  WatchersWidget,
} from "@features/csm-cases/components/CaseDetailWidgets";
import { CallRequestsWidget } from "@features/csm-cases/components/CallRequestsWidget";
import { useGetCsmCaseCallRequests } from "@features/csm-cases/api/useCsmCaseCallRequests";
import { TasksWidget } from "@features/csm-cases/components/TasksWidget";
import { useSearchCaseTasks } from "@features/csm-cases/api/useSearchCaseTasks";
import { useSearchDeployments } from "@features/csm-cases/api/useSearchDeployments";
import { useGetProject } from "@features/csm-projects/api/useGetProject";
import { CaseSlaTable } from "@features/csm-cases/components/CaseSlaTable";
import { useGetCsmCaseSlas } from "@features/csm-cases/api/useGetCsmCaseSlas";
import CaseTimeCardsPanel from "@features/csm-timecards/components/CaseTimeCardsPanel";
import LogTimeCardDialog from "@features/csm-timecards/components/LogTimeCardDialog";
import {
  useCaseTimeCards,
  usePostTimeCard,
  useUpdateTimeCard,
} from "@features/csm-timecards/api/useTimeCards";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";
import { caseIdLabel } from "@features/csm-cases/utils/caseIdentity";
import { useReportCaseTabDraft } from "@features/case-tabs/hooks/useReportCaseTabDraft";
import { useReportCaseTabMeta } from "@features/case-tabs/hooks/useReportCaseTabMeta";
import { useCaseRouteOverride } from "@context/case-tabs/CaseRouteOverrideContext";
import { formatAbsoluteForUser } from "@utils/dateTime";
import {
  isBlankHtml,
  isDescriptionEchoedInComment,
  stripHtmlTags,
} from "@utils/sanitizeHtml";
import {
  canResumeToUnlockPublicReply as computeCanResumeToUnlockPublicReply,
  effectiveWorkState,
  publicCommentGateReason,
  WORK_STATE_LABEL,
} from "@features/csm-cases/utils/caseWorkState";
import { useRecordRecentView } from "@features/csm-recent/hooks/useRecentViews";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import QueryErrorState from "@components/QueryErrorState";
import RelativeTime from "@components/RelativeTime";
import SeverityChip from "@components/SeverityChip";
import StateChip from "@components/StateChip";
import { CASE_TYPE_LABEL } from "@features/csm-cases/utils/caseType";
import type {
  CaseAttachment,
  CaseLifecycleAction,
  CreateChangeRequestFromCaseNavState,
  CreateIncidentFromCaseNavState,
  CreateRelatedCaseNavState,
  CreateServiceRequestFromCaseNavState,
  CsmCaseComment,
} from "@features/csm-cases/types/csmCases";
import type { CaseState } from "@features/csm-dashboard/types/abtDashboard";
import { useNavTransition } from "@hooks/useNavTransition";
import { useNormalizedIdParam } from "@hooks/useNormalizedIdParam";
import { useQueryParamTabs } from "@hooks/useSectionTabs";

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
  // Unused: intercepted before this map is read (see onAction) since it
  // navigates instead of showing a toast. Present only to satisfy the
  // exhaustive Record.
  create_related_case: "",
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
  // Unused — see the matching note in LIFECYCLE_TOAST.
  create_related_case: "info",
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

// Only covers secondary actions that are handled inline below with a fixed,
// literal toast — every action with real branching feedback (success/error,
// dynamic text) sets its own message directly instead of reading this map.
// Currently empty (its one entry, copy_link, was removed with the menu item)
// but kept as the lookup fallback below for the next fixed-toast action.
const SECONDARY_TOAST: Record<string, string> = {};

type CaseTabId =
  | "activities"
  | "details"
  | "related"
  | "watchers"
  | "sla"
  | "attachments"
  | "time"
  | "call-requests"
  | "tasks";


const TAB_DEFS: Array<{
  id: CaseTabId;
  label: string;
  icon: JSX.Element;
  disabled?: boolean;
  /** Hidden from the visible tab bar for now, without removing the tab's
   * content/data — see the "tasks" entry below. */
  hidden?: boolean;
}> = [
  { id: "activities", label: "Activities", icon: <Activity size={16} /> },
  { id: "details", label: "Details", icon: <ListChecks size={16} /> },
  // Label is "Linked Items", not "Related" — "related" is also a distinct
  // link type (LinkCaseDialog's CaseLinkType) shown inside this same tab,
  // and reusing the word for the tab name too was confusing the two. Same
  // chain-link icon as "Link to another case"/"Linked service requests"
  // inside this tab, not the people icon "Related" used.
  { id: "related", label: "Linked Items", icon: <LinkIcon size={16} /> },
  { id: "watchers", label: "Watchers", icon: <Eye size={16} /> },
  { id: "sla", label: "SLAs", icon: <Clock size={16} /> },
  { id: "attachments", label: "Attachments", icon: <Paperclip size={16} /> },
  { id: "time", label: "Time tracking", icon: <Layers size={16} /> },
  { id: "call-requests", label: "Call requests", icon: <Phone size={16} /> },
  // Hidden from the tab bar for now (review follow-up) — the underlying
  // tasks feature (data, hooks, widget, create-task action) is untouched, and
  // the tab's content section still renders if `activeTab` is ever "tasks";
  // it's just unreachable via tab navigation while hidden.
  { id: "tasks", label: "Tasks", icon: <CheckSquare size={16} />, hidden: true },
];
// Only the ids with a rendered `<Tab>` — a caller-supplied list for
// `useQueryParamTabs`, not the nav tree. A hidden tab like "tasks" is
// deliberately excluded: `useQueryParamTabs` would otherwise treat
// `?tab=tasks` as "recognised" and select it as `activeTab`, but with no
// matching `<Tab>` in the strip that leaves the underlying MUI `Tabs` value
// out of range and nothing visually selected.
const CASE_TAB_IDS: readonly CaseTabId[] = TAB_DEFS.filter(
  (t) => !t.hidden,
).map((t) => t.id);

export default function CsmCaseDetailPage(): JSX.Element {
  // Real router hooks — called unconditionally regardless of `routeOverride`
  // below (rules of hooks), but their VALUES are only actually used when
  // this instance isn't part of an open in-app case tab. `routeOverride`
  // presence never changes for the lifetime of a given mounted instance
  // (an isolated-tab instance always has one; a directly-routed page — a
  // deep link, or a case opened past the open-tab cap — never does), so
  // preferring one or the other is stable across this instance's renders.
  //
  // The override exists because this page can be mounted several times at
  // once (one per open tab, all kept alive in the background — see
  // `CaseTabIsolatedRouter`), while there is only ever ONE real matched
  // route/location for the app as a whole. Without it, every background
  // tab's `useParams`/`useLocation` would resolve to whatever route is
  // CURRENTLY on-screen, not the case this particular instance represents.
  const routedCaseId = useNormalizedIdParam("caseId");
  const routedNavigate = useNavTransition();
  const routedLocation = useLocation();
  const routeOverride = useCaseRouteOverride();
  const caseId = routeOverride?.caseId ?? routedCaseId;
  const navigate = routeOverride?.navigate ?? routedNavigate;
  const location = routeOverride
    ? {
        pathname: routeOverride.pathname,
        search: routeOverride.search,
        hash: routeOverride.hash,
        state: routeOverride.state,
      }
    : routedLocation;
  const isEngagementRoute = location.pathname.startsWith("/engagements/");
  const isServiceRequestRoute = location.pathname.startsWith("/operations/service-requests/");
  const isAnnouncementRoute = location.pathname.startsWith("/announcements/");
  const isSecurityReportRoute = location.pathname.startsWith(
    "/security-center/security-reports/",
  );
  const backPath = isEngagementRoute
    ? "/engagements"
    : isServiceRequestRoute
      ? "/operations/service-requests"
      : isAnnouncementRoute
        ? "/announcements"
        : isSecurityReportRoute
          ? "/security-center/security-reports"
          : "/cases";
  const detailPath = isEngagementRoute
    ? `/engagements/${caseId}`
    : isServiceRequestRoute
      ? `/operations/service-requests/${caseId}`
      : isAnnouncementRoute
        ? `/announcements/${caseId}`
        : isSecurityReportRoute
          ? `/security-center/security-reports/${caseId}`
          : `/cases/${caseId}`;
  // The row link on the originating list carries its own (filtered) URL
  // forward as router state, so the back button below returns to that exact
  // list view — filters, search text, sort — instead of a bare list path.
  // Falls back to the hardcoded backPath for a bookmarked or directly-linked
  // detail page, which never got the state set.
  const fromListState = location.state as { from?: string } | undefined;
  const resolvedBackPath = fromListState?.from ?? backPath;
  const {
    data,
    isLoading,
    isError,
    error,
    refetch: refetchCaseDetail,
    isFetching: isFetchingCaseDetail,
    dataUpdatedAt: caseDetailUpdatedAt,
  } = useGetCsmCaseDetail(caseId);
  // Reports this case's number ONLY (not the wso2CaseId/subject the header
  // and recent-views entry show — a deliberately short tab chip label, by
  // request) up to the in-app case-tabs layer. Same field for all five
  // case-like kinds this page renders (engagements, service requests,
  // announcements, security reports all carry a `caseNumber` too, from the
  // same `CsmCaseDetail` shape).
  useReportCaseTabMeta(caseId, {
    label: data?.caseNumber,
    internalId: data?.wso2CaseId,
    subject: data?.subject,
  });
  // The route alone isn't a reliable signal once data has loaded: a "Related
  // case" link always points at /cases/:id regardless of the target's actual
  // type, so an announcement opened that way would otherwise render the full
  // case UI. Combine the route with the loaded case's own caseType.
  const isAnnouncement = isAnnouncementRoute || data?.caseType === "announcement";
  // Same reasoning as isAnnouncement above — Service Requests don't carry a
  // severity, so combine the route with the loaded case's own caseType.
  const isServiceRequest =
    isServiceRequestRoute || data?.caseType === "service_request";
  // Security report analyses have no dedicated pre-load route (they open via
  // the generic /cases/:caseId route), so caseType is the only signal.
  const isSecurityReport = data?.caseType === "security_report_analysis";
  // Same reasoning as isAnnouncement above — Engagements don't carry a
  // severity, so combine the route with the loaded case's own caseType.
  const isEngagement = isEngagementRoute || data?.caseType === "engagement";

  // Engagements, Announcements, Service Requests, and Security Reports each
  // have a dedicated route; opening a case under any route other than its own
  // canonical one (e.g. a "Related case" link, which always points at
  // /cases/:id, or a dedicated route reached with a case of a different
  // type) should redirect to its real destination rather than silently
  // rendering under the wrong section.
  const canonicalDetailPath =
    data?.caseType === "engagement"
      ? `/engagements/${caseId}`
      : data?.caseType === "announcement"
        ? `/announcements/${caseId}`
        : data?.caseType === "service_request"
          ? `/operations/service-requests/${caseId}`
          : data?.caseType === "security_report_analysis"
            ? `/security-center/security-reports/${caseId}`
            : `/cases/${caseId}`;
  const isMisrouted = !!data && detailPath !== canonicalDetailPath;

  useEffect(() => {
    if (!caseId || !isMisrouted) return;
    // Carry the originating list location through the canonical redirect. Without
    // it, a record reached on a non-canonical route (announcements, service
    // requests, engagements, security reports) lands on its dedicated route with
    // empty state, and Back then falls through to the bare route-specific path,
    // dropping the filters, search and sort that got the user here.
    //
    // Also carries the current `?tab=` and `#fragment` forward onto the
    // canonical target — without this, following a "Related case" link (which
    // always points at the non-canonical /cases/:id) to a case opened on a
    // specific tab, or a permalink, silently dropped both the moment the case
    // turned out to be an engagement/announcement/service request/security
    // report and got redirected to its real route.
    navigate(
      { pathname: canonicalDetailPath, search: location.search, hash: location.hash },
      { replace: true, state: { from: resolvedBackPath } },
    );
  }, [
    isMisrouted,
    canonicalDetailPath,
    caseId,
    navigate,
    resolvedBackPath,
    location.search,
    location.hash,
  ]);

  const {
    data: comments,
    isLoading: isCommentsLoading,
    isError: isCommentsError,
    refetch: refetchComments,
    isFetching: isFetchingComments,
  } = useGetCsmCaseComments(caseId);
  // Audited field/state changes (the "State changes" lifecycle lane), loaded
  // from the dedicated activities endpoint — kept separate from the comments
  // hook above, which is the sole source for comments/work notes.
  const {
    data: activityAudit,
    isLoading: isActivityLoading,
    isError: isActivityError,
    refetch: refetchActivities,
    isFetching: isFetchingActivities,
  } = useGetCsmCaseActivities(caseId);
  // Case Feedback (CSAT survey) submissions for this case, if any — almost
  // always empty for an open case (the survey goes out after closure), which
  // is expected and renders no feedback lane rather than an error.
  const {
    data: caseFeedback,
    isLoading: isFeedbackLoading,
    isError: isFeedbackError,
    refetch: refetchFeedback,
    isFetching: isFetchingFeedback,
  } = useGetCsmCaseFeedback(caseId);
  // The chat transcript the case was spawned from, when linked. Loaded lazily
  // off the case's conversation id and merged into the comment stream below so
  // it renders as the earliest activity entries — mirrors the customer portal.
  // Disabled (no fetch) when the case has no linked conversation, so
  // isChatLoading/isChatError stay false for chat-less cases.
  const {
    data: chatMessages,
    isLoading: isChatLoading,
    isError: isChatError,
    refetch: refetchChat,
    isFetching: isFetchingChat,
  } = useGetCsmConversationMessages(data?.conversationId);
  const postComment = usePostCsmCaseComment();
  const {
    data: attachments,
    isLoading: isAttachmentsLoading,
    isError: isAttachmentsError,
    refetch: refetchAttachments,
    isFetching: isFetchingAttachments,
    dataUpdatedAt: attachmentsUpdatedAt,
  } = useGetCsmCaseAttachments(caseId);
  const postAttachment = usePostCsmCaseAttachment();
  const downloadAttachment = useDownloadCsmCaseAttachment();
  const getAttachmentPreviewContent = useGetCsmCaseAttachmentContent();
  const deleteAttachment = useDeleteCsmCaseAttachment();
  // Fetched unconditionally (not just while their tab is active) purely for
  // the tab-label counts below; each widget still runs its own scoped query
  // when its tab mounts, deduped against this one by react-query's cache.
  // Skipped for announcements (undefined caseId disables the query) since
  // neither tab is shown there.
  const { data: slaList } = useGetCsmCaseSlas(
    isAnnouncement ? undefined : caseId,
  );
  const {
    data: callRequests,
    refetch: refetchCallRequests,
    isFetching: isFetchingCallRequests,
  } = useGetCsmCaseCallRequests(isAnnouncement ? undefined : caseId);
  const { data: caseTasks } = useSearchCaseTasks(
    isAnnouncement ? undefined : caseId,
  );
  const { data: caseTimeCards } = useCaseTimeCards(
    isAnnouncement ? undefined : caseId,
  );
  // Live deployment lookup for the Details tab's "Deployment info" widget —
  // only runs when the case actually has a deployment link (SN-sourced cases
  // may have none). Reuses the project's deployment list rather than a
  // single-deployment GET, since the backend has no `/deployments/{id}` route.
  const {
    data: projectDeployments,
    isLoading: isProjectDeploymentsLoading,
    refetch: refetchProjectDeployments,
    isFetching: isFetchingProjectDeployments,
  } = useSearchDeployments(
    data?.productContext.deploymentId ? data.projectId : undefined,
  );
  const liveDeployment = projectDeployments?.find(
    (d) => d.id === data?.productContext.deploymentId,
  );
  // Richer account/project facts for the Customer card, beyond what's
  // embedded in the case-detail payload's `customerContext` snapshot.
  const {
    data: caseProject,
    isLoading: isCaseProjectLoading,
    refetch: refetchCaseProject,
    isFetching: isFetchingCaseProject,
  } = useGetProject(data?.projectId);
  const patchCase = usePatchCsmCase(caseId);
  const patchCaseById = usePatchCsmCaseById();
  const createTask = useCreateCaseTask(caseId);
  const addTag = useAddCaseTag(caseId);
  const removeTag = useRemoveCaseTag(caseId);
  const requestCaseUpdate = useRequestCaseUpdate();
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
  const { showSuccess } = useSuccessBanner();
  const [feedback, setFeedback] = useState<Feedback | null>(null);
  // Kept in the URL (`?tab=`), not local state, so a shared/bookmarked link
  // to a specific tab survives a refresh. Unlike the plain `useState` this
  // replaced, `setActiveTab` writes through the router — see the two
  // "force to Activities" effects below for why the render-time adjustments
  // that used to call it directly were moved into effects instead: calling a
  // *router* navigation synchronously during render (as opposed to this
  // component's own local `useState` setters, which the surrounding
  // render-time resets still use safely) risks updating the Router's state
  // while this component is still rendering.
  const { activeTab, setActiveTab } = useQueryParamTabs<CaseTabId>(
    CASE_TAB_IDS,
    "activities",
  );
  // Permalink fragment (`/cases/:id#<entry-id>`), consumed by the scroll and
  // highlight effect further down. Hoisted up here because two render-time
  // state adjustments below both need it: the per-case reset and the
  // per-fragment Activities-tab force.
  const permalinkFragment = location.hash?.replace(/^#/, "") ?? "";
  // Tracked separately from patchCase.isPending: that flag is shared with the
  // lifecycle transitions, and reusing it would spin the state button whenever
  // someone acknowledges (and vice versa).
  const [isAcknowledging, setIsAcknowledging] = useState(false);
  const [metaCollapsed, setMetaCollapsed] = useState(false);
  const [composerOpen, setComposerOpen] = useState(false);
  // Reports composerOpen up to the in-app case-tabs layer, purely so closing
  // this case's tab from the tab strip can confirm first — see the hook's
  // own doc comment for what this signal does and doesn't guarantee.
  useReportCaseTabDraft(caseId, composerOpen);
  const [assignOpen, setAssignOpen] = useState(false);
  const [linkCaseOpen, setLinkCaseOpen] = useState(false);
  const [linkIncidentOpen, setLinkIncidentOpen] = useState(false);
  const [autocloseHoldOpen, setAutocloseHoldOpen] = useState(false);
  const [editDetailsOpen, setEditDetailsOpen] = useState(false);
  const [createTaskOpen, setCreateTaskOpen] = useState(false);
  const [fixEtaOpen, setFixEtaOpen] = useState(false);
  const [requestUpdateOpen, setRequestUpdateOpen] = useState(false);
  const [addTagOpen, setAddTagOpen] = useState(false);
  // ISSU-026: closing or proposing a solution opens this instead of PATCHing
  // immediately — it collects the Post Resolution Activity and doubles as
  // the confirmation step for these two customer-notifying transitions.
  const [resolutionDialog, setResolutionDialog] = useState<{
    kind: "close" | "propose_solution";
    targetState: BeCaseState;
  } | null>(null);
  const [severityOpen, setSeverityOpen] = useState(false);
  const [changeCaseTypeOpen, setChangeCaseTypeOpen] = useState(false);
  const [logTimeOpen, setLogTimeOpen] = useState(false);
  // The card being edited, if any — mutually exclusive with logTimeOpen
  // (create); LogTimeCardDialog is rendered once for whichever is set.
  const [editTimeCard, setEditTimeCard] = useState<CsmTimeCard | null>(null);
  const [githubIssueOpen, setGithubIssueOpen] = useState(false);
  // Inline error shown inside the Git-issue dialog (e.g. the SN routing 422 /
  // state 409). Cleared when the dialog opens or a submit is retried.
  const [githubIssueError, setGithubIssueError] = useState<string | null>(null);
  // Set on a successful submit so the dialog can show its own "created" view
  // with a clickable link instead of closing itself immediately.
  const [githubIssueResult, setGithubIssueResult] =
    useState<BeCreateCaseGithubIssueResponse | null>(null);
  const postGithubIssue = usePostCaseGithubIssue();
  const postTimeCard = usePostTimeCard();
  const updateTimeCard = useUpdateTimeCard();
  // Attachment pending delete confirmation (drives the confirm dialog).
  const [pendingDelete, setPendingDelete] = useState<CaseAttachment | null>(
    null,
  );
  // Attachment shown in the inline preview dialog.
  const [previewTarget, setPreviewTarget] = useState<CaseAttachment | null>(
    null,
  );
  // When starting work would leave the engineer with more than one ongoing case,
  // hold the other ongoing case(s) here to drive the confirm dialog.
  const [pauseConflict, setPauseConflict] = useState<MyOngoingCase[] | null>(
    null,
  );
  // Email of the signed-in engineer, for the "Assign to me" shortcut.
  const currentUserEmail = claims?.email ?? undefined;

  // This page stays mounted across case-to-case navigation (route pattern is
  // the same, only :caseId changes), so state left over from the previous
  // case — e.g. a sticky "requested more info" banner, or an open dialog —
  // would otherwise leak into the newly opened case. Reset it synchronously
  // during render (React's recommended pattern for resetting state when a
  // prop changes) rather than in an effect, to avoid an extra render pass.
  const [prevCaseId, setPrevCaseId] = useState(caseId);
  // Distinguishes this render's view of the page from every prior one, even
  // a return visit to the same caseId (A -> B -> A) — a plain caseId
  // comparison can't tell those apart, which is exactly what let a stale
  // mutation callback from the first visit to A slip through on the second.
  // Bumped inside the reset block below, once per genuine transition.
  const caseViewTokenRef = useRef(0);
  if (caseId !== prevCaseId) {
    setPrevCaseId(caseId);
    setFeedback(null);
    setComposerOpen(false);
    setAssignOpen(false);
    setResolutionDialog(null);
    setSeverityOpen(false);
    setChangeCaseTypeOpen(false);
    setLogTimeOpen(false);
    // Not just cosmetic: the edit dialog renders on editTimeCard alone, so a
    // card left open here would stay mounted against the new case and submit
    // a PATCH for the *previous* case's card.
    setEditTimeCard(null);
    setGithubIssueOpen(false);
    setGithubIssueError(null);
    setGithubIssueResult(null);
    setPendingDelete(null);
    setPreviewTarget(null);
    setPauseConflict(null);
    setLinkCaseOpen(false);
    setLinkIncidentOpen(false);
    setAutocloseHoldOpen(false);
    setEditDetailsOpen(false);
    setCreateTaskOpen(false);
    setFixEtaOpen(false);
    setRequestUpdateOpen(false);
    setAddTagOpen(false);
    // A new view of the page, distinct from every prior one even if it's a
    // return visit to the same caseId (A -> B -> A) — see caseViewTokenRef
    // below, which onRequestUpdate compares against instead of caseId itself.
    caseViewTokenRef.current += 1;
    // The permalink-fragment-triggered force to Activities (for both this
    // case-change and a same-case fragment change) lives in one effect below
    // — see `permalinkForceRef` — rather than here, since `setActiveTab` now
    // writes through the router and so can't be called synchronously during
    // render the way this block's own local `useState` resets safely can.
  }

  // isAnnouncement can only be confirmed once `data` loads (see its
  // definition above) — if a case reached via /cases/:id turns out to be an
  // announcement and the active tab is one hidden for announcements, fall
  // back to Activities. Unlike the plain local-state resets above, this
  // can't safely run during render any more: `setActiveTab` now writes
  // through the router (`useQueryParamTabs`), and a router write during
  // render risks updating the Router's state while this component is still
  // rendering — so it's an effect instead.
  useEffect(() => {
    if (
      isAnnouncement &&
      (activeTab === "related" ||
        activeTab === "watchers" ||
        activeTab === "sla" ||
        activeTab === "time" ||
        activeTab === "call-requests" ||
        activeTab === "tasks")
    ) {
      setActiveTab("activities");
    }
  }, [isAnnouncement, activeTab, setActiveTab]);

  // Twitter-style permalinks: when the URL has a fragment matching an entry id,
  // jump to the Activities tab and hand off to `scrollToFragmentWithRetry`,
  // which scrolls the entry into view and flashes it once it actually exists
  // in the DOM.
  //
  // The target only exists once every activity-feed source has loaded:
  // comments, the linked chat transcript, and the audit trail (see the
  // `isCommentsLoading || isChatLoading || isActivityLoading` skeleton gate
  // below). A brand-new tab starts all three of those requests cold and they
  // don't resolve in a fixed order, so gate the whole permalink attempt on
  // every one of them finishing rather than retrying only when `comments`
  // itself changes — otherwise a case where chat/audit resolve after comments
  // retries once, too early, and never gets another chance. On an
  // already-open tab this race has usually already settled by the time a
  // fragment link is followed, which is why the bug reads as "new tab only."
  const activitiesFeedReady =
    !isCommentsLoading && !isChatLoading && !isActivityLoading && !isFeedbackLoading;
  // Forces the Activities tab exactly once per permalink — on the case or the
  // fragment actually changing, tracked by this ref rather than `activeTab`
  // itself (which would re-force every time the *effect* below re-ran, e.g.
  // when `activitiesFeedReady` flips false → true as the three feed sources
  // settle, dragging a user who had since switched tabs back to Activities).
  // A ref rather than the `prevCaseId`-style render-time `useState` reset
  // above: `setActiveTab` now writes through the router (`useQueryParamTabs`),
  // so it can't run synchronously during render — this has to be an effect,
  // and an effect needs its "did this change" comparison done inside itself
  // (a `useState`-based previous-value comparison during render, like
  // `prevCaseId`'s, would have already resolved to "unchanged" by the time
  // this effect runs).
  //
  // Following a permalink to a *different* case with the *same* fragment
  // (e.g. #description → #description) is covered by comparing `caseId` here
  // too, not just `permalinkFragment` — the fragment alone wouldn't change in
  // that case, so the old case's tab would otherwise carry over unforced.
  // Starts unset rather than pre-seeded with the current `caseId`/fragment:
  // on a cold load that already has a permalink fragment in the URL (e.g.
  // `?tab=attachments#entry-9`), a pre-seeded ref would make the "did it
  // change" check below false on the very first render, so the tab would
  // never get forced to Activities and the page would stay wherever `?tab=`
  // pointed instead of jumping to the linked entry.
  const permalinkForceRef = useRef<
    { caseId: string | undefined; fragment: string } | undefined
  >(undefined);
  useEffect(() => {
    const prev = permalinkForceRef.current;
    const changed =
      !prev || prev.caseId !== caseId || prev.fragment !== permalinkFragment;
    permalinkForceRef.current = { caseId, fragment: permalinkFragment };
    if (changed && permalinkFragment) {
      setActiveTab("activities");
    }
  }, [caseId, permalinkFragment, setActiveTab]);
  useEffect(() => {
    // Wait for the feed to actually be able to render the target before
    // attempting to find it — see the comment above.
    if (!permalinkFragment || !activitiesFeedReady) return;

    return scrollToFragmentWithRetry(permalinkFragment, {
      onNotFound: () => {
        // Every source has loaded and the id still isn't in the DOM — it's
        // not a timing problem. Say so rather than leaving the page silently
        // scrolled to the top as if the link were malformed (e.g. a deleted
        // comment, or one the viewer isn't permitted to see).
        showError(
          "Could not find the linked entry — it may have been removed, or you may not have permission to view it.",
        );
      },
    });
  }, [permalinkFragment, activitiesFeedReady, showError]);

  // Re-runs every source the Activities tab's merged timeline draws from —
  // comments, the audit trail, the linked chat transcript (a no-op when
  // disabled), attachments, and call requests (the last only enriches a
  // field-change entry that references one, but a stale label there is still
  // worth refreshing). ServiceNow only refreshes a single tab like this in
  // its own case UI; the whole page previously had to be reloaded to see
  // this tab's data change.
  const isRefreshingActivities =
    isFetchingComments ||
    isFetchingActivities ||
    isFetchingChat ||
    isFetchingAttachments ||
    isFetchingCallRequests ||
    isFetchingFeedback;
  const refreshActivitiesTab = (): void => {
    void refetchComments();
    void refetchActivities();
    void refetchChat();
    void refetchAttachments();
    void refetchCallRequests();
    void refetchFeedback();
  };

  // Re-runs every source the Details tab renders: the case itself plus the
  // two supplemental lookups (project, live deployment) the Customer/Product
  // cards enrich with.
  const isRefreshingDetails =
    isFetchingCaseDetail || isFetchingCaseProject || isFetchingProjectDeployments;
  const refreshDetailsTab = (): void => {
    void refetchCaseDetail();
    void refetchCaseProject();
    void refetchProjectDeployments();
  };

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
      // Snapshot of the fields QuickNavCaseCard needs, so Pinned/Recent
      // entries for this case render the same rich card a live search hit
      // does, without the palette re-fetching it.
      caseHit: {
        caseNumber: data.caseNumber,
        wso2CaseId: data.wso2CaseId,
        subject: data.subject,
        severity: data.severity,
        state: data.state,
        workState: data.workState,
        caseType: data.caseType,
        updatedOn: data.updatedAt,
        assigneeName: data.assigneeName,
      },
    });
  }, [data, recordView]);

  // Resolve the single-active-case rule once the engineer's other ongoing
  // cases are already known: mark THIS case ongoing if there are none, or
  // prompt to pause the others first (handled in onConfirmStartWork). Shared
  // by `startWork` (after moving to Work in progress) and the resume-work
  // path in `onAction` (case is already Work in progress, just un-pausing).
  const resolveOngoingConflict = useCallback(
    async (
      others: MyOngoingCase[],
      successMessage: string,
      successSeverity: FeedbackSeverity,
      // Defaults to the inline sticky banner; `startWork`'s `assign_to_me`
      // caller overrides this to the floating success toast instead — see
      // the note on `startWork` below.
      reportSuccess: (message: string) => void = () =>
        setFeedback({ message: successMessage, severity: successSeverity, sticky: true }),
    ) => {
      if (others.length > 0) {
        setPauseConflict(others);
        return;
      }
      try {
        await patchCase.mutateAsync({ workState: "ongoing" });
      } catch (err) {
        showError("Could not mark the case ongoing. Please try again.", err);
        return;
      }
      reportSuccess(successMessage);
    },
    [patchCase, showError],
  );

  // Acknowledge: claim the case as the signed-in engineer. First-write-wins
  // upstream, so a race with the out-of-band acknowledgement link (or another
  // engineer clicking at the same moment) resolves to whoever landed first and
  // is reported as such rather than surfacing as an error. The refetch that
  // usePatchCsmCase triggers is what removes the button, so nothing here has to
  // reconcile local state.
  const onAcknowledge = useCallback(async (): Promise<void> => {
    setIsAcknowledging(true);
    try {
      const result = await patchCase.mutateAsync({ acknowledge: true });
      const holder = result.case?.acknowledgedBy?.name?.trim();
      showSuccess(
        result.case?.alreadyAcknowledged
          ? holder
            ? `This case was already acknowledged by ${holder}.`
            : "This case was already acknowledged."
          : "Case acknowledged.",
      );
    } catch (err) {
      showError("Could not acknowledge the case. Please try again.", err);
    } finally {
      setIsAcknowledging(false);
    }
  }, [patchCase, showError, showSuccess]);

  // Starting work: enforce the single-active-case rule.
  // 1) look up the engineer's other ongoing cases (abort on failure — we
  //    must not transition without knowing), 2) move this case to
  // work_in_progress, 3) resolve via `resolveOngoingConflict`. Shared by the
  // "Start progress" transition and the "Assign to me" shortcut, which also
  // puts the case into progress once the assignment lands.
  const startWork = useCallback(
    async (
      successMessage: string,
      successSeverity: FeedbackSeverity,
      // "Assign to me" passes the floating success toast here instead of the
      // default inline banner (see `resolveOngoingConflict`) — everything
      // else sharing this function (Start/Resume work) keeps the banner.
      reportSuccess?: (message: string) => void,
    ) => {
      if (!data) return;
      const caseId = data.id;
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

      // Auto-acknowledge as a side effect of starting work: an engineer who
      // starts work on an unacknowledged, acknowledgeable case has implicitly
      // claimed it, so don't make them separately click "Acknowledge" too.
      // Evaluated against the pre-PATCH `data` closed over above, not a
      // refetch — severity never changes via this flow and `acknowledgedBy`
      // can only change if someone else acknowledges concurrently, which the
      // backend's first-write-wins semantics already handle gracefully. The
      // `state` PATCH above and this `acknowledge` PATCH must stay separate
      // calls: the entity-service rejects combining `state` and `acknowledge`
      // (and other mutually-exclusive fields) in one request. This is a
      // bonus, not the action the user asked for, so it's best-effort: any
      // failure here must not block `resolveOngoingConflict` below, and we
      // don't surface a separate error toast — the Acknowledge button simply
      // stays visible afterward, which is a safe, correct fallback the
      // engineer can act on manually.
      if (canAcknowledge(data)) {
        try {
          await patchCase.mutateAsync({ acknowledge: true });
        } catch {
          // Swallow: see comment above.
        }
      }

      await resolveOngoingConflict(others, successMessage, successSeverity, reportSuccess);
    },
    [data, findMyOngoingCases, patchCase, showError, resolveOngoingConflict],
  );

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

        // "Assign to me" on the Change-state button: the case isn't the
        // engineer's yet, so claim it (PATCH assigneeEmail) before starting
        // work — a plain state PATCH would move it to Work in progress
        // without ever making it the clicking engineer's case. Guarded on its
        // own (not folded into the generic work_in_progress branch below) so
        // a missing email surfaces an error instead of silently starting work
        // on a case that was never actually assigned.
        if (action === "assign_to_me" && data) {
          if (!currentUserEmail) {
            showError("Could not assign the case to you: no signed-in email found.");
            return;
          }
          patchCase.mutate(
            { assigneeEmail: currentUserEmail },
            {
              onSuccess: () =>
                void startWork(
                  LIFECYCLE_TOAST.assign_to_me,
                  LIFECYCLE_SEVERITY.assign_to_me,
                  showSuccess,
                ),
              onError: (err) =>
                showError("Could not assign the case to you.", err),
            },
          );
          return;
        }

        // ISSU-004: the backend puts `reopened` in a closed case's
        // `nextStates` only as a signal — there is no real reopen (the data
        // source has no such transition). Never PATCH it; open the new-case
        // form pre-filled with relatedCaseId instead. Must run before the
        // generic `targetState` PATCH below, since `beStateFromUi("reopened")`
        // is truthy and would otherwise be sent as a state transition.
        if (action === "create_related_case" && data) {
          const navState: CreateRelatedCaseNavState = {
            projectId: data.projectId,
            relatedCaseId: data.id,
            relatedCaseNumber: data.caseNumber,
            deploymentId: data.productContext.deploymentId,
            deployedProductId: data.productContext.deployedProductId,
            // The related case's severity only prefills the new-case form
            // when it's a real S0-S4 value — an "unset" source severity
            // leaves the (required) field blank so the engineer must pick
            // one, same as any other case with no severity to carry over.
            severity: data.severity === "unset" ? undefined : data.severity,
            issueType: data.issueType,
            subject: `Related Case : ${data.subject}`,
          };
          navigate("/cases/new", { state: navState });
          return;
        }

        // ISSU-026: closing or proposing a solution records the Post
        // Resolution Activity first — open that dialog instead of PATCHing
        // immediately. Must run before the generic `targetState` PATCH below.
        if ((action === "close" || action === "propose_solution") && targetState) {
          setResolutionDialog({ kind: action, targetState });
          return;
        }

        if (targetState === "work_in_progress" && data) {
          void startWork(LIFECYCLE_TOAST[action], LIFECYCLE_SEVERITY[action]);
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

      // Change severity opens the severity picker; the PATCH happens in
      // onChangeSeverity once a new value is confirmed.
      if (action.secondary === "change_severity") {
        setSeverityOpen(true);
        return;
      }

      // Change case type opens the transfer dialog; the PATCH(es) happen in
      // onChangeCaseType once a target type is confirmed.
      if (action.secondary === "change_case_type") {
        setChangeCaseTypeOpen(true);
        return;
      }

      // Hold auto-closure opens the date picker; the PATCH happens in
      // onSetAutocloseHold once a date is confirmed.
      if (action.secondary === "hold_auto_close") {
        setAutocloseHoldOpen(true);
        return;
      }

      // Edit case details opens the subject/description/deployment/deployed
      // product form; the sequential PATCHes happen in onEditCaseDetails.
      if (action.secondary === "edit_case_details") {
        setEditDetailsOpen(true);
        return;
      }

      // Create incident from case navigates to the create-incident form,
      // pre-filled with this case as the new incident's parent (ServiceNow's
      // generic task-parent reference — see CreateIncidentPage.tsx's read of
      // the nav state).
      if (action.secondary === "create_incident" && data) {
        const navState: CreateIncidentFromCaseNavState = {
          caseId: data.id,
          caseNumber: data.caseNumber,
          subject: data.subject,
          // The case description is rich-text HTML; the incident form's
          // Description field is plain text (sent as additionalComments), so
          // strip tags rather than carrying markup through as visible text.
          description: isBlankHtml(data.description)
            ? undefined
            : stripHtmlTags(data.description),
        };
        navigate("/operations/incidents/new", { state: navState });
        return;
      }

      // Create service request navigates to the service-request create form,
      // pre-filled with this case as the new SR's linked case — same nav-state
      // shape and target route as the Related tab's "Linked service requests"
      // card (see that widget's onCreateServiceRequest above); this is just a
      // second entry point onto the same flow, mirroring create_incident.
      if (action.secondary === "create_service_request" && data) {
        const navState: CreateServiceRequestFromCaseNavState = {
          projectId: data.projectId,
          relatedCaseId: data.id,
          relatedCaseNumber: data.caseNumber,
          deploymentId: data.productContext.deploymentId,
          deployedProductId: data.productContext.deployedProductId,
        };
        navigate("/operations/service-requests/new", { state: navState });
        return;
      }

      // Create change request navigates to the change-request create form,
      // pre-filled with this service request as the new change request's
      // "Originating service request" — mirrors the create_incident handler
      // above. Only offered for a service request (see CaseActionBar's
      // caseType gate on this menu item).
      if (action.secondary === "create_change_request" && data) {
        const navState: CreateChangeRequestFromCaseNavState = {
          caseId: data.id,
          caseNumber: data.caseNumber,
          caseSubject: data.subject,
          projectId: data.projectId,
        };
        navigate("/operations/change-requests/new", { state: navState });
        return;
      }

      // Set fix ETA opens the date/time picker; the PATCH happens in
      // onSetFixEta once a value is confirmed.
      if (action.secondary === "set_fix_eta") {
        setFixEtaOpen(true);
        return;
      }

      // Request update opens the reminder-template dialog; the POST happens
      // in onRequestUpdate once a stage (or custom message) is confirmed.
      if (action.secondary === "request_update") {
        setRequestUpdateOpen(true);
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
                    "Work paused — you can't post public replies until you resume.",
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
          await resolveOngoingConflict(others, "Resumed work on this case.", "info");
        })();
        return;
      }

      if (action.secondary === "log_time") {
        // Time cards can still be logged after a case is closed — engineers
        // often record time after the fact.
        setLogTimeOpen(true);
        return;
      }

      // ISSU-020: file an internal GitHub issue from the case. Closed cases
      // are blocked at the menu level (CaseActionBar's raise_git_issue item
      // is disabled — see caseClosed there); this handler only runs for a
      // non-closed case. The SN side still resolves the target repo from the
      // product and may reject other states we don't otherwise pre-gate, so
      // any backend rejection still surfaces inline in the dialog.
      if (action.secondary === "raise_git_issue") {
        setGithubIssueError(null);
        setGithubIssueOpen(true);
        return;
      }

      // Reachable only for a secondary action with no handler above — every
      // menu item that isn't wired up yet is disabled in CaseActionBar, so
      // this is a defensive fallback, not a normal path.
      setFeedback({
        message: SECONDARY_TOAST[action.secondary] ?? "This action isn't available yet.",
        severity: "info",
        sticky: false,
      });
    },
    [
      data,
      showError,
      showSuccess,
      patchCase,
      findMyOngoingCases,
      startWork,
      resolveOngoingConflict,
      currentUserEmail,
      navigate,
    ],
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
            showSuccess("Case reassigned.");
          },
          onError: (err) => showError("Could not reassign the case.", err),
        },
      );
    },
    [patchCase, showError, showSuccess],
  );

  // Submits the Post Resolution Activity dialog: PATCHes state alongside
  // resolutionCode/cause/closeNotes in one call (the backend accepts all
  // four together for these two transitions — see BeCaseUpdatePayload).
  const onResolutionSubmit = useCallback(
    (fields: {
      resolutionCode: BeCaseResolutionCode;
      cause: BeCaseCause;
      closeNotes: string;
    }) => {
      if (!resolutionDialog) return;
      const { kind, targetState } = resolutionDialog;
      patchCase.mutate(
        { state: targetState, ...fields },
        {
          onSuccess: () => {
            setResolutionDialog(null);
            setFeedback({
              message: LIFECYCLE_TOAST[kind],
              severity: LIFECYCLE_SEVERITY[kind],
              sticky: true,
            });
          },
          onError: (err) =>
            showError("Could not update the case. Please try again.", err),
        },
      );
    },
    [patchCase, resolutionDialog, showError],
  );

  const onChangeSeverity = useCallback(
    (next: Severity) => {
      patchCase.mutate(
        { severity: priorityFromSeverity(next) },
        {
          onSuccess: () => {
            setSeverityOpen(false);
            setFeedback({
              message: `Severity changed to ${next}.`,
              severity: "success",
              sticky: true,
            });
          },
          onError: (err) => showError("Could not change the severity.", err),
        },
      );
    },
    [patchCase, showError],
  );

  // Changes the case's type. The type change itself is one
  // PATCH ({type} alone, or {type, engagementType, engagementPaymentType}
  // together for a transfer into engagement — the backend requires them
  // combined there). Severity is
  // a separate, optional follow-up PATCH when transferring into `case`: it's
  // a data-completeness extra, not required to complete the transfer (see
  // caseTypeTransfer.ts), so its failure is reported but doesn't roll back
  // or block the type change that already succeeded. The refetch that
  // usePatchCsmCase triggers picks up the new caseType, which then trips the
  // canonical-route redirect above (e.g. onto /engagements/:id) on its own —
  // no manual navigation needed here.
  const onChangeCaseType = useCallback(
    (submission: CaseTypeTransferSubmission) => {
      // One atomic PATCH, not a transfer followed by a severity patch: the backend
      // requires severity and issueType *in the same call* as `type: "case"`, and a
      // standalone severity patch is rejected outright on a case of any other type.
      patchCase.mutate(
        submission.targetType === "engagement"
          ? {
              type: "engagement",
              engagementType: submission.engagementType,
              engagementPaymentType: submission.engagementPaymentType,
            }
          : submission.targetType === "security_report_analysis"
            ? { type: "security_report_analysis" }
            : submission.targetType === "service_request"
              ? {
                  type: "service_request",
                  catalogId: submission.catalogId,
                  catalogItemId: submission.catalogItemId,
                  variables: submission.variables,
                }
              : {
                  type: "case",
                  severity: priorityFromSeverity(submission.severity),
                  issueType: submission.issueType,
                },
        {
          onSuccess: () => {
            setChangeCaseTypeOpen(false);
            setFeedback({
              message: `Case type changed to ${caseTypeTransferLabel(submission.targetType)}.`,
              severity: "success",
              sticky: true,
            });
          },
          onError: (err) => showError("Could not change the case type.", err),
        },
      );
    },
    [patchCase, showError],
  );

  // Watchers are edited inline in the Watchers tab. There is no
  // add-one/remove-one endpoint: `PATCH /cases/{id}` takes the *whole*
  // `watchList` as user UUIDs and replaces what is stored. WatchersWidget
  // computes that replacement list (add and remove alike) and hands it over
  // finished, so this page only forwards it — see WatchersWidget's doc
  // comment for why the last watcher on a case can't be removed. Supported by
  // one data source only; the backend rejects it on the others and the error
  // surfaces via showError.
  const onReplaceWatchers = useCallback(
    (nextWatcherIds: string[], action: "add" | "remove") => {
      patchCase.mutate(
        { watchList: nextWatcherIds },
        {
          onSuccess: () =>
            setFeedback({
              message: action === "add" ? "Watcher added." : "Watcher removed.",
              severity: "success",
              sticky: false,
            }),
          onError: (err) => {
            // The watch-list 400s name the offending value (an unknown or
            // malformed user id), which is far more actionable than a generic
            // string — same treatment as every other 4xx on this page.
            const msg =
              err instanceof BackendApiError && err.status < 500 && err.message
                ? err.message
                : action === "add"
                  ? "Could not add the watcher."
                  : "Could not remove the watcher.";
            showError(msg, err);
          },
        },
      );
    },
    [patchCase, showError],
  );

  const onSetAutocloseHold = useCallback(
    (holdUntilIso: string) => {
      patchCase.mutate(
        { autocloseHoldUntil: holdUntilIso },
        {
          onSuccess: () => {
            setAutocloseHoldOpen(false);
            setFeedback({
              message: "Case placed on auto-closure hold.",
              severity: "success",
              sticky: false,
            });
          },
          onError: (err) => showError("Could not hold auto-closure.", err),
        },
      );
    },
    [patchCase, showError],
  );

  // Fires one sequential PATCH per changed field — the backend accepts
  // exactly one field per call (see BeCaseUpdatePayload) — and resolves with
  // a per-field result once every attempt has settled, so a partial failure
  // (e.g. subject saves but deployment doesn't) is reported field-by-field
  // instead of silently swallowed or rolled into one generic error.
  const onEditCaseDetails = useCallback(
    async (changes: {
      subject?: string;
      description?: string;
      deploymentId?: string;
      deployedProductId?: string;
    }): Promise<FieldSaveResult[]> => {
      const entries = Object.entries(changes) as [
        keyof typeof changes,
        string,
      ][];
      const results: FieldSaveResult[] = [];
      for (const [field, value] of entries) {
        try {
          await patchCase.mutateAsync({
            [field]: value,
          } as unknown as BeCaseUpdatePayload);
          results.push({ field, ok: true });
        } catch (err) {
          results.push({
            field,
            ok: false,
            error:
              err instanceof BackendApiError && err.message
                ? err.message
                : "Could not save this field.",
          });
        }
      }
      return results;
    },
    [patchCase],
  );

  const onLinkCase = useCallback(
    (targetCaseId: string, linkType: CaseLinkType) => {
      patchCase.mutate(
        linkType === "parent"
          ? { parentId: targetCaseId }
          : { relatedCaseId: targetCaseId },
        {
          onSuccess: () => {
            setLinkCaseOpen(false);
            setFeedback({
              message:
                linkType === "parent"
                  ? "Case linked as parent."
                  : "Case linked as related.",
              severity: "success",
              sticky: false,
            });
          },
          onError: (err) => showError("Could not link this case.", err),
        },
      );
    },
    [patchCase, showError],
  );

  const onLinkIncident = useCallback(
    (targetIncidentId: string) => {
      patchCase.mutate(
        { parentId: targetIncidentId },
        {
          onSuccess: () => {
            setLinkIncidentOpen(false);
            setFeedback({
              message: "Incident linked as parent.",
              severity: "success",
              sticky: false,
            });
          },
          onError: (err) => showError("Could not link this incident.", err),
        },
      );
    },
    [patchCase, showError],
  );

  const onCreateTask = useCallback(
    (payload: BeCreateCaseTaskPayload) => {
      createTask.mutate(payload, {
        onSuccess: () => {
          setCreateTaskOpen(false);
          // Not jumping to the Tasks tab here: it's hidden from the tab bar
          // for now (see the `hidden` flag on its TAB_DEFS entry), so setting
          // activeTab to a tab with no corresponding rendered Tab would leave
          // the Tabs component with an out-of-range value. The task itself is
          // still created — just no tab to land on to see it.
          setFeedback({
            message: "Task created.",
            severity: "success",
            sticky: false,
          });
        },
        onError: (err) => showError("Could not create the task.", err),
      });
    },
    [createTask, showError],
  );

  const onSetFixEta = useCallback(
    (patch: FixEtaSavePayload) => {
      patchCase.mutate(patch as BeCaseUpdatePayload, {
        onSuccess: () => {
          setFeedback({
            message: "Fix ETA updated.",
            severity: "success",
            sticky: false,
          });
        },
        onError: (err) => showError("Could not set the fix ETA.", err),
      });
    },
    [patchCase, showError],
  );

  const onRequestUpdate = useCallback(
    (payload: RequestUpdateSavePayload) => {
      if (!caseId) return;
      // Compared against caseViewTokenRef in the callbacks below, not caseId
      // itself: a plain caseId comparison can't tell a still-pending request
      // from *this* view of the case apart from one left over from an
      // earlier visit to the same case (A -> B -> A), since the id is
      // identical in both. The token bumps on every transition, including a
      // return to a previously-visited case, so it can.
      const submittedViewToken = caseViewTokenRef.current;
      requestCaseUpdate.mutate(
        { caseId, ...payload },
        {
          onSuccess: () => {
            if (caseViewTokenRef.current !== submittedViewToken) return;
            setRequestUpdateOpen(false);
            setFeedback({
              message: "Update request posted.",
              severity: "success",
              sticky: false,
            });
          },
          onError: (err) => {
            if (caseViewTokenRef.current !== submittedViewToken) return;
            // 403 (not the assigned engineer) / 409 (case moved out of the
            // eligible state since the dialog opened) both carry an
            // actionable, specific backend message — surface it instead of a
            // generic fallback, same treatment as every other 4xx on this page.
            const msg =
              err instanceof BackendApiError && err.status < 500 && err.message
                ? err.message
                : "Could not post the update request.";
            showError(msg, err);
          },
        },
      );
    },
    [caseId, requestCaseUpdate, showError],
  );

  const onAddTag = useCallback(
    (label: string) => {
      addTag.mutate(label, {
        onSuccess: () => {
          setAddTagOpen(false);
          setFeedback({
            message: "Tag added.",
            severity: "success",
            sticky: false,
          });
        },
        onError: (err) => showError("Could not add the tag.", err),
      });
    },
    [addTag, showError],
  );

  const onRemoveTag = useCallback(
    (tagId: string) => {
      removeTag.mutate(tagId, {
        onError: (err) => showError("Could not remove the tag.", err),
      });
    },
    [removeTag, showError],
  );

  const attachmentList = useMemo(() => attachments ?? [], [attachments]);

  // Case comments + the linked chat transcript, as one list for the activity
  // feed. Memoised so the feed's own sort doesn't rerun on every render.
  const mergedComments = useMemo(
    () => [...(comments ?? []), ...(chatMessages ?? [])],
    [comments, chatMessages],
  );

  // The origin comment — the earliest comment on the case, sorted rather
  // than assumed to already be `comments[0]`. Cases created by customers
  // consistently echo the description as this comment (often with a wrapper
  // like a signature); cases created by internal automation frequently don't
  // — sometimes there's no first comment at all. `descriptionEchoed` below
  // decides whether the description still needs its own card.
  const originComment = useMemo(() => {
    if (!comments || comments.length === 0) return undefined;
    return [...comments].sort(
      (a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime(),
    )[0];
  }, [comments]);
  const descriptionEchoedInOriginComment = isDescriptionEchoedInComment(
    data?.description ?? "",
    originComment?.bodyHtml,
  );

  // The case description usually arrives as the opening comment too, so the
  // real comment already carries it and no extra entry is needed. When it
  // doesn't (see `descriptionEchoedInOriginComment` above — content-based,
  // covers cases created by internal automation and cases with no origin
  // comment at all, and today always covers announcements too since their
  // description is submitted through a path that never creates an origin
  // comment) a synthetic comment-shaped entry is appended here so the
  // Activities tab always shows the description inline in the timeline,
  // attributed to the case creator, rather than as a visually distinct card.
  // Deliberately just `descriptionEchoedInOriginComment` — not `isAnnouncement
  // || !descriptionEchoedInOriginComment` — so this stays correct even if
  // announcement creation ever starts producing a real echoed origin comment;
  // an unconditional carve-out would silently start double-rendering the
  // description at that point instead of adapting. It carries `synthetic:
  // true` so `CsmCaseCommentBubble` suppresses the author-role chip — the
  // creator's real role isn't known on the frontend (see the field's doc
  // comment), so nothing is claimed about it. This is folded into
  // `safeComments` itself (not a separate prop) so the "N entries" count and
  // the feed's own chronological sort both pick it up naturally. This is now
  // the only place the description renders — the Details tab's duplicate
  // fallback card was removed.
  const safeComments = useMemo(() => {
    if (isBlankHtml(data?.description ?? "") || descriptionEchoedInOriginComment) {
      return mergedComments;
    }
    const synthetic: CsmCaseComment = {
      id: `case-description-${data?.id}`,
      caseId: data?.id ?? "",
      authorName: data?.createdBy ?? data?.customerContext?.primaryContact ?? "—",
      authorEmail: data?.createdByEmail,
      authorUser: data?.createdByUser,
      // Not a claim that the creator is actually a customer — "customer" is
      // simply the enum value that renders the neutral grey avatar with no
      // engineer styling, and the real role is unknowable here.
      authorRole: "customer",
      bodyHtml: data?.description ?? "",
      createdAt: data?.createdAt ?? "",
      internal: false,
      synthetic: true,
    };
    return [...mergedComments, synthetic];
  }, [data, descriptionEchoedInOriginComment, mergedComments]);

  const onUploadAttachment = useCallback(
    (file: File) => {
      if (!caseId) return;
      if (data?.state === "closed") {
        showError("This case is closed — attachments are read-only.");
        return;
      }
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
    [caseId, engineerName, postAttachment, data, showError],
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

  if (isLoading || isMisrouted) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rounded" height={32} width={240} />
        <Skeleton variant="rounded" height={200} />
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
          onClick={() => navigate(resolvedBackPath)}
          sx={{ alignSelf: "flex-start" }}
        >
          Back
        </Button>
        <QueryErrorState
          message={error instanceof Error && error.message.trim() ? error.message : "Could not load this case."}
          error={error}
        />
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
          onClick={() => navigate(resolvedBackPath)}
          sx={{ alignSelf: "flex-start" }}
        >
          Back
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
  // work_in_progress + ongoing AND the signed-in engineer is the case's
  // assignee. Internal work notes are allowed in any state, so this only
  // disables the public-reply path in the composer — never work notes.
  // Mirrors the BFF comment guard so the engineer sees a clear reason instead
  // of a generic error.
  //
  // Announcements are exempt from this whole gate on the backend (see
  // CreateCaseComment's announcement carve-out): they publish immediately,
  // have no work_in_progress/ongoing workflow, and may carry no assigned
  // engineer at all. The only thing that still blocks a comment there is the
  // case being closed, which `isClosed` above (and the composer's `disabled`
  // prop below) already covers — so skip the state/ownership gate here rather
  // than have it report a reason ("...actively in progress"/"...assigned
  // engineer...") that would never resolve for an announcement.
  const publicReplyGateReason = isAnnouncement
    ? null
    : publicCommentGateReason(c.state, c.workState, c.assigneeIsMe);
  // The composer's inline "Resume work" quick-fix only applies to this one
  // lock reason — the case is already work_in_progress and assigned to the
  // signed-in engineer, just paused, so resuming is the single-field PATCH
  // `onAction` already runs for "toggle_work_state" below. The other lock
  // reason (not started yet) needs the full assign/start flow, which doesn't
  // belong in the composer; `assigneeIsMe` also excludes another engineer's
  // paused case, matching CaseActionBar's own gate on the same action.
  // Never applicable to announcements — see publicReplyGateReason above.
  const canResumeToUnlockPublicReply = isAnnouncement
    ? false
    : computeCanResumeToUnlockPublicReply(c.state, c.workState, c.assigneeIsMe);
  // FE-only, advisory close-gate: warn when the case has an open task, so the
  // engineer isn't surprised by a close rejection. Best-effort — the task
  // *list* (`POST /cases/{id}/tasks/search`) returns `BeTaskSummary`, which
  // doesn't carry `visibleToCustomer` (only the single-task `GET /tasks/{id}`
  // does), so this can't restrict to customer-visible tasks specifically as
  // originally scoped; it flags ANY open task, which over-blocks slightly
  // more than the real (server-side) gate is likely to. This is UI-only — the
  // entity-service enforces the authoritative close gate, and a rejection
  // still surfaces via showError even if this signal is stale or absent.
  const hasOpenTask = (caseTasks?.tasks ?? []).some((t) => t.state === "OPEN");
  const closeBlockedReason = hasOpenTask
    ? "This case has an open task. Closing may be rejected until it's resolved or closed."
    : undefined;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(resolvedBackPath)}
        sx={{ alignSelf: "flex-start" }}
      >
        Back
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
            {!isAnnouncement && c.caseType && CASE_TYPE_LABEL[c.caseType] && (
              <Chip
                size="small"
                variant="outlined"
                label={CASE_TYPE_LABEL[c.caseType]}
                sx={{ fontWeight: 600 }}
              />
            )}
            {!isAnnouncement &&
              !isServiceRequest &&
              !isSecurityReport &&
              !isEngagement && (
                <SeverityChip severity={c.severity} withLabel />
              )}
            {!isAnnouncement && <StateChip state={c.state} />}
            {/* Related/Parent moved to CaseMetaBand's Overview cells — those
                are singular facts (never more than one each), so a compact
                "Cell" fits better than a chip crowding this row, especially
                once both are present on the same case at once. */}
            {!isAnnouncement &&
              c.autoclosureStep &&
              c.autoclosureStep !== "DEFAULT" && (
                <Chip
                  size="small"
                  variant="outlined"
                  color="warning"
                  icon={<PauseCircle size={14} />}
                  label={
                    c.autoclosureStateTime
                      ? `On hold until ${formatAbsoluteForUser(c.autoclosureStateTime) ?? "—"}`
                      : "On auto-closure hold"
                  }
                  sx={{ fontWeight: 600 }}
                />
              )}
            {!isAnnouncement && c.state === "work_in_progress" && (
              <Chip
                size="small"
                variant="outlined"
                color={effectiveWorkState(c.workState) === "paused" ? "warning" : "default"}
                label={WORK_STATE_LABEL[effectiveWorkState(c.workState)]}
                sx={{ fontWeight: 600 }}
              />
            )}
            {!isAnnouncement && c.tags.length > 0 && (
              <Divider orientation="vertical" flexItem sx={{ mx: 0.5, my: 0.25 }} />
            )}
            {!isAnnouncement &&
              c.tags.map((t) => (
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
        {!isAnnouncement && (
          <Box sx={{ flexShrink: 0, alignSelf: { xs: "stretch", md: "flex-start" } }}>
            <CaseActionBar
              caseDetail={c}
              onAction={onAction}
              closeBlockedReason={closeBlockedReason}
              isPending={patchCase.isPending && !isAcknowledging}
              onAcknowledge={onAcknowledge}
              isAcknowledging={isAcknowledging}
            />
          </Box>
        )}
      </Box>

      <CaseMetaBand
        detail={c}
        collapsed={metaCollapsed}
        onToggleCollapsed={() => setMetaCollapsed((v) => !v)}
      />

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
          {TAB_DEFS.filter(
            (t) =>
              !t.hidden &&
              (!isAnnouncement ||
                (t.id !== "related" &&
                  t.id !== "watchers" &&
                  t.id !== "sla" &&
                  t.id !== "time" &&
                  t.id !== "call-requests")),
          ).map((t) => {
            // Counts shown only where the tab IS the list (unambiguous), or
            // where the parent case-detail object already has the list in
            // hand. "related" sums linkedChangeRequests + linkedServiceRequests
            // (both already present on `c`); ChildCasesWidget is still
            // excluded since it runs its own scoped query and would need an
            // extra fetch to get a count.
            const count =
              t.id === "watchers"
                ? c.watchers.length
                : t.id === "sla"
                  ? slaList?.count
                  : t.id === "attachments"
                    ? attachmentList.length
                    : t.id === "time"
                      ? caseTimeCards?.total
                      : t.id === "call-requests"
                        ? callRequests?.length
                        : t.id === "tasks"
                          ? caseTasks?.total
                          : t.id === "related"
                            ? (c.parentCase?.type === "incident" ? 1 : 0) +
                              (c.linkedChangeRequests?.length ?? 0) +
                              (c.linkedServiceRequests?.length ?? 0)
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
              `publicReplyGateReason` when the case isn't in-progress/ongoing —
              always null for an announcement, per the note on that constant
              above. Shown for announcements too (backend now accepts both
              comment types there), despite the hidden CaseActionBar above —
              that hides case-lifecycle patch actions, which don't apply to an
              announcement, not the ability to reply to one. */}
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
                disabled={!caseId || isClosed}
                publicCommentDisabledReason={publicReplyGateReason}
                canResumeToUnlockPublicReply={canResumeToUnlockPublicReply}
                onResumeWork={() => onAction({ secondary: "toggle_work_state" })}
                isResumingWork={patchCase.isPending}
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
              disabled={isClosed}
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
              {isClosed
                ? "This case is closed — comments and work notes are read-only."
                : publicReplyGateReason
                  ? "Add an internal work note…"
                  : "Compose a reply to the customer…"}
            </Button>
          )}

          <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: 1.5,
              }}
            >
              <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
                <Typography variant="subtitle2">Activity timeline</Typography>
                {!isCommentsLoading && (
                  <Chip
                    size="small"
                    variant="outlined"
                    label={`${safeComments.length + (activityAudit?.length ?? 0) + attachmentList.length + (caseFeedback?.length ?? 0)} entries`}
                  />
                )}
              </Box>
              <RefreshButton
                onRefresh={refreshActivitiesTab}
                isFetching={isRefreshingActivities}
                label="Refresh activity timeline"
              />
            </Box>

            {isCommentsLoading || isChatLoading || isActivityLoading || isFeedbackLoading ? (
              // Wait for the comments, linked chat transcript, activity
              // audit, and Case Feedback so nothing pops into an
              // already-rendered timeline. isChatLoading is false for
              // chat-less cases (query disabled).
              <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                {[0, 1, 2].map((i) => (
                  <Skeleton key={i} variant="rounded" height={56} />
                ))}
              </Box>
            ) : (
              // A comments/chat/activity failure shouldn't blank the timeline —
              // the description and whatever else loaded fine. Show them with
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
                {isFeedbackError && (
                  <Typography variant="body2" color="error">
                    Could not load Case Feedback. Showing the rest of the
                    activity — reload to try again.
                  </Typography>
                )}
                {isActivityError && (
                  <Typography variant="body2" color="error">
                    Could not load state changes. Showing the rest of the
                    activity — reload to try again.
                  </Typography>
                )}
                <CaseActivitiesFeed
                  comments={safeComments}
                  audit={activityAudit ?? []}
                  attachments={attachmentList}
                  feedback={caseFeedback ?? []}
                  callRequests={callRequests ?? []}
                  onDownloadAttachment={onDownloadAttachment}
                  preview={{
                    onGetPreviewContent: getAttachmentPreviewContent,
                    previewTarget,
                    onPreviewTargetChange: setPreviewTarget,
                  }}
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
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
              <Typography variant="subtitle2">Identifiers &amp; timestamps</Typography>
              <RefreshButton
                onRefresh={refreshDetailsTab}
                isFetching={isRefreshingDetails}
                updatedAt={caseDetailUpdatedAt}
                label="Refresh case details"
              />
            </Box>
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
            </Box>
          </Card>
          {/* Service-request-only: the catalog answers the requester filled
              in. Reuses the page's single `isServiceRequest` signal (route +
              loaded caseType) rather than adding a parallel one. Always
              rendered for an SR — the widget itself shows the empty state, so
              a request that captured no answers stays visible as a data
              problem instead of silently vanishing. */}
          {isServiceRequest && (
            <RequestDetailsWidget
              catalog={c.catalog}
              catalogItem={c.catalogItem}
              variables={c.requestVariables}
            />
          )}
          <CustomerContextWidget
            ctx={c.customerContext}
            project={caseProject}
            isLoadingProject={isCaseProjectLoading}
            accountId={c.accountId}
          />
          <ProductContextWidget
            ctx={c.productContext}
            liveDeployment={liveDeployment}
            isLoadingLiveDeployment={
              !!c.productContext.deploymentId && isProjectDeploymentsLoading
            }
          />
          <TagsWidget
            tags={c.tags}
            onAdd={isClosed ? undefined : () => setAddTagOpen(true)}
            onRemove={isClosed ? undefined : (t) => onRemoveTag(t.id)}
            removingId={removeTag.isPending ? removeTag.variables : null}
          />
        </Box>
      )}

      {activeTab === "related" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          {/* Case-level action, not scoped to any one card below: the dialog
              lets the user pick parent-vs-related for any target case, so it
              doesn't belong nested inside one specific relationship card. The
              refresh button on the other side of this same row re-runs the
              case-detail query, which is where each card's linked-item refs
              (id/number/name) come from — each card's own enrichment data
              (state, assignee, ...) has its own refresh button instead. */}
          <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 1 }}>
            {/* Same read-only-once-closed rule as the comment composer,
                attachment upload, and tag add/remove below — a link PATCH
                on a closed case would otherwise fail server-side and only
                surface as a generic error banner instead of being disabled
                up front with a clear reason. */}
            <Tooltip title={isClosed ? "This case is closed — it's read-only." : ""}>
              <Box component="span">
                <Button
                  size="small"
                  variant="outlined"
                  startIcon={<LinkIcon size={14} />}
                  onClick={() => setLinkCaseOpen(true)}
                  disabled={isClosed}
                >
                  Link to another case
                </Button>
              </Box>
            </Tooltip>
            <RefreshButton
              onRefresh={() => void refetchCaseDetail()}
              isFetching={isFetchingCaseDetail}
              updatedAt={caseDetailUpdatedAt}
              label="Refresh linked items"
            />
          </Box>
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
            <ChildCasesWidget caseId={c.id} />
            <LinkedIncidentWidget
              caseId={c.id}
              parentCase={c.parentCase}
              onLinkIncident={() => setLinkIncidentOpen(true)}
              linkDisabled={isClosed}
            />
            {/* Change requests are only ever raised from a service request,
                never directly from a plain case — gate solely on
                `isServiceRequest` rather than falling back to
                `linkedChangeRequests` having entries, which a plain case
                should never carry anyway (see `linkedChangeRequests` doc
                comment on the case-detail type). Not a data-source gate. */}
            {isServiceRequest && (
              <LinkedChangeRequestsWidget changeRequests={c.linkedChangeRequests} />
            )}
            <LinkedServiceRequestsWidget
              caseId={c.id}
              linkedServiceRequests={c.linkedServiceRequests}
              createDisabled={isClosed}
              onCreateServiceRequest={() => {
                const navState: CreateServiceRequestFromCaseNavState = {
                  projectId: c.projectId,
                  relatedCaseId: c.id,
                  relatedCaseNumber: c.caseNumber,
                  deploymentId: c.productContext.deploymentId,
                  deployedProductId: c.productContext.deployedProductId,
                };
                navigate("/operations/service-requests/new", { state: navState });
              }}
            />
          </Box>
        </Box>
      )}

      {activeTab === "watchers" && (
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: "1fr" }}>
          {/* Watchers list — moved off the (single-line) overview Cell so a
              long watch list has room to wrap as chips, and given its own
              tab (split out of "Linked Items") since it's not actually a
              linked record. Add/remove are inline here (no separate dialog);
              "Manage watchers…" in the action bar just jumps to this tab. */}
          <WatchersWidget
            entityKind="case"
            watchers={c.watchers}
            onReplace={onReplaceWatchers}
            isSaving={patchCase.isPending}
            onRefresh={() => void refetchCaseDetail()}
            isRefreshing={isFetchingCaseDetail}
            refreshedAt={caseDetailUpdatedAt}
          />
        </Box>
      )}

      {activeTab === "sla" &&
        caseId && <CaseSlaTable caseId={caseId} />}

      {activeTab === "attachments" && (
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: "1fr" }}>
          <AttachmentsWidget
            attachments={attachmentList}
            loading={isAttachmentsLoading}
            error={isAttachmentsError}
            onRetry={() => void refetchAttachments()}
            onRefresh={() => void refetchAttachments()}
            isRefreshing={isFetchingAttachments}
            refreshedAt={attachmentsUpdatedAt}
            uploading={postAttachment.isPending}
            uploadError={
              postAttachment.isError
                ? (postAttachment.error?.message ??
                  "Could not upload the attachment.")
                : null
            }
            onUpload={isClosed ? undefined : onUploadAttachment}
            onDownloadAll={onDownloadAllAttachments}
            onDownload={onDownloadAttachment}
            onDelete={setPendingDelete}
            deletingId={deleteAttachment.isPending ? pendingDelete?.id : null}
            preview={{
              onGetPreviewContent: getAttachmentPreviewContent,
              previewTarget,
              onPreviewTargetChange: setPreviewTarget,
            }}
          />
        </Box>
      )}

      {activeTab === "time" && (
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: "1fr" }}>
          <CaseTimeCardsPanel
            caseId={c.id}
            onLogTime={() => setLogTimeOpen(true)}
            onEditTimeCard={setEditTimeCard}
          />
        </Box>
      )}

      {activeTab === "call-requests" && caseId && (
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: "1fr" }}>
          <CallRequestsWidget
            caseId={caseId}
            severity={c.severity}
            caseState={c.state}
            isClosed={isClosed}
          />
        </Box>
      )}

      {activeTab === "tasks" && caseId && (
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: "1fr" }}>
          <TasksWidget caseId={caseId} />
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

      {resolutionDialog && (
        <ResolutionDialog
          kind={resolutionDialog.kind}
          isSubmitting={patchCase.isPending}
          initial={
            data?.resolution
              ? {
                  resolutionCode: data.resolution.resolutionCode,
                  cause: data.resolution.cause,
                  closeNotes: data.resolution.notes,
                }
              : undefined
          }
          onClose={() => setResolutionDialog(null)}
          onSubmit={onResolutionSubmit}
        />
      )}

      {severityOpen && (
        <ChangeSeverityDialog
          currentSeverity={c.severity}
          // S0 is reserved for Managed Cloud, same rule as case creation (see
          // CsmCaseCreatePage.tsx) — caseProject is the same project fetch
          // already used for the Customer card above.
          isManagedCloud={caseProject?.subscriptionType === "managed_cloud_subscription"}
          isChanging={patchCase.isPending}
          onClose={() => setSeverityOpen(false)}
          onChange={onChangeSeverity}
        />
      )}

      {changeCaseTypeOpen && (
        <ChangeCaseTypeDialog
          currentType={c.caseType ?? "case"}
          currentSeverity={c.severity}
          hasAttachments={attachmentList.length > 0}
          currentProjectName={c.projectName}
          currentDeploymentName={c.productContext.deployment}
          currentProductName={c.productContext.product}
          currentWatchers={c.watchers}
          currentTags={c.tags}
          deployedProductId={c.productContext.deployedProductId}
          onUploadAttachment={onUploadAttachment}
          isUploadingAttachment={postAttachment.isPending}
          uploadAttachmentError={
            postAttachment.isError
              ? (postAttachment.error?.message ?? "Could not upload the attachment.")
              : undefined
          }
          isSubmitting={patchCase.isPending}
          onClose={() => setChangeCaseTypeOpen(false)}
          onSubmit={onChangeCaseType}
        />
      )}

      {autocloseHoldOpen && (
        <SetAutocloseHoldDialog
          currentHoldUntil={c.autoclosureStateTime}
          isSaving={patchCase.isPending}
          onClose={() => setAutocloseHoldOpen(false)}
          onSave={onSetAutocloseHold}
        />
      )}

      {editDetailsOpen && (
        <EditCaseDetailsDialog
          projectId={c.projectId}
          currentSubject={c.subject}
          currentDescriptionHtml={c.description}
          currentDeploymentId={c.productContext.deploymentId}
          currentDeployedProductId={c.productContext.deployedProductId}
          isSaving={patchCase.isPending}
          onClose={() => setEditDetailsOpen(false)}
          onSubmit={onEditCaseDetails}
        />
      )}

      {linkCaseOpen && (
        <LinkCaseDialog
          currentCaseId={c.id}
          isLinking={patchCase.isPending}
          onClose={() => setLinkCaseOpen(false)}
          onLink={onLinkCase}
        />
      )}

      {linkIncidentOpen && (
        <LinkIncidentDialog
          isLinking={patchCase.isPending}
          onClose={() => setLinkIncidentOpen(false)}
          onLink={onLinkIncident}
        />
      )}

      {createTaskOpen && (
        <CreateTaskDialog
          isSaving={createTask.isPending}
          onClose={() => setCreateTaskOpen(false)}
          onSubmit={onCreateTask}
        />
      )}

      {fixEtaOpen && (
        <SetFixEtaDialog
          currentBestCaseFixEta={c.bestCaseFixEta}
          currentMostLikelyFixEta={c.mostLikelyFixEta}
          currentWorstCaseFixEta={c.worstCaseFixEta}
          isSaving={patchCase.isPending}
          onClose={() => setFixEtaOpen(false)}
          onSave={onSetFixEta}
        />
      )}

      {requestUpdateOpen && (
        <RequestUpdateDialog
          category={deriveCaseUpdateRequestCategory(c)}
          isSaving={requestCaseUpdate.isPending}
          onClose={() => setRequestUpdateOpen(false)}
          onSave={onRequestUpdate}
        />
      )}

      {addTagOpen && (
        <AddTagDialog
          existingLabels={c.tags.map((t) => t.label)}
          isSaving={addTag.isPending}
          onClose={() => setAddTagOpen(false)}
          onSave={onAddTag}
        />
      )}

      {(logTimeOpen || editTimeCard) && (
        <LogTimeCardDialog
          caseId={c.id}
          caseNumber={c.caseNumber ?? c.id}
          caseSeverity={c.severity}
          projectId={c.projectId}
          projectName={c.projectName}
          editingCard={editTimeCard ?? undefined}
          isSubmitting={editTimeCard ? updateTimeCard.isPending : postTimeCard.isPending}
          onClose={() => {
            setLogTimeOpen(false);
            setEditTimeCard(null);
          }}
          onSubmit={(input) => {
            // "cardId" only exists on an edit submission — see
            // LogTimeCardSubmit's doc comment.
            if ("cardId" in input) {
              updateTimeCard.mutate(input, {
                onSuccess: () => {
                  setEditTimeCard(null);
                  setFeedback({
                    message: "Time card updated.",
                    severity: "success",
                    sticky: false,
                  });
                },
                onError: (err) => {
                  const msg =
                    err instanceof BackendApiError && err.status < 500 && err.message
                      ? err.message
                      : "Could not save your changes.";
                  showError(msg, err);
                },
              });
              return;
            }
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
            });
          }}
        />
      )}

      {githubIssueOpen && (
        <CreateGithubIssueDialog
          open={githubIssueOpen}
          submitting={postGithubIssue.isPending}
          error={githubIssueError}
          createdIssue={githubIssueResult}
          defaultUpdateLevel={c.productContext.updateLevel}
          defaultTitle={c.subject}
          defaultDescription={c.description}
          showRepoField={isCloudSupportSubscription(caseProject?.subscriptionType)}
          onClose={() => {
            setGithubIssueOpen(false);
            setGithubIssueError(null);
            setGithubIssueResult(null);
          }}
          onOpenConfirm={() => setGithubIssueError(null)}
          onSubmit={(payload) => {
            setGithubIssueError(null);
            postGithubIssue.mutate(
              { caseId: c.id, ...payload },
              {
                onSuccess: (res) => {
                  // Dialog shows its own "created" view with a clickable
                  // link (see createdIssue) instead of closing immediately —
                  // still switch to the activities feed in the background so
                  // the SN-written work-note entry is visible once they're
                  // done reading the confirmation.
                  setActiveTab("activities");
                  setGithubIssueResult(res);
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
