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
  TriangleAlert,
  X,
} from "@wso2/oxygen-ui-icons-react";
import { useCallback, useEffect, useState, type JSX } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { useGetCsmCaseDetail } from "@features/csm-cases/api/useGetCsmCaseDetail";
import {
  useGetCsmCaseComments,
  usePostCsmCaseComment,
} from "@features/csm-cases/api/useCsmCaseComments";
import CsmCaseCommentInput from "@features/csm-cases/components/CsmCaseCommentInput";
import CaseActionBar from "@features/csm-cases/components/CaseActionBar";
import CaseActivitiesFeed from "@features/csm-cases/components/CaseActivitiesFeed";
import CaseMetaBand from "@features/csm-cases/components/CaseMetaBand";
import {
  AttachmentsWidget,
  AuditTimelineWidget,
  CustomerContextWidget,
  LinkedItemsWidget,
  ProductContextWidget,
  SlaTimelineWidget,
  TimeLogsWidget,
  WatchersWidget,
} from "@features/csm-cases/components/CaseDetailWidgets";
import { TIER_COLOR, TIER_LABEL } from "@features/csm-cases/utils/caseTier";
import { useRecordRecentView } from "@features/csm-recent/hooks/useRecentViews";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import {
  SEVERITY_COLOR,
  SEVERITY_LABEL,
  SLA_CLOCK_LABEL,
  STATE_LABEL,
  formatTimeToBreach,
} from "@features/csm-dashboard/utils/abtDashboard";
import RelativeTime from "@components/RelativeTime";
import type {
  CaseLifecycleAction,
  CsmCaseComment,
  CsmCaseDetail,
} from "@features/csm-cases/types/csmCases";

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

// TODO: replace with the engineer name from useGetUserDetails once the
// `firstName`/`lastName` shape from the CSM backend is wired.
const CURRENT_ENGINEER_NAME = "Sajith Ekanayaka";

// Reopening a closed case is a lead-only override. We match the lead role from
// the ID-token group claims.
// TODO(authz): confirm the exact Asgardeo group(s)/role(s) that designate a
// "lead" with the gateway/permission model — these identifiers are provisional.
const LEAD_REOPEN_ROLES = new Set([
  "cre_lead",
  "CRE_LEADS_GROUP",
  "leadership",
]);

const LIFECYCLE_TOAST: Record<CaseLifecycleAction, string> = {
  start_work: "Started work on this case.",
  assign_to_me: "Assigned to you.",
  propose_solution: "Solution proposed to the customer.",
  request_info: "Requested additional info from the customer.",
  wait_on_wso2: "Marked as waiting on internal WSO2 dependency.",
  resume_work: "Resumed work on this case.",
  close: "Case closed.",
  close_no_response: "Closed (no response received).",
  reopen: "Case reopened.",
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
  reopen: "info",
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
  raise_git_issue: "Raise internal Git issue (mock).",
  create_task: "Create task dialog (mock).",
  request_call: "Request a call dialog (mock).",
  log_time: "Log time dialog (mock).",
  copy_link: "Case link copied to clipboard.",
  download_all_attachments: "Preparing all attachments for download (mock).",
  download_attachment: "Downloading attachment (mock).",
};

type CaseTabId =
  | "activities"
  | "details"
  | "sla"
  | "attachments"
  | "time";

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

/**
 * Synthesize the case description as a virtual "first comment" so it shows up
 * in the activity stream immediately after the `created` audit event. SN does
 * the same thing — the customer's initial problem statement reads as the
 * opening comment of the case.
 *
 * Dated 1 second after `case.createdAt` so chronological sort places it
 * AFTER the `Case created` audit entry (which is exactly at `createdAt`).
 */
function buildDescriptionComment(c: CsmCaseDetail): CsmCaseComment | null {
  if (!c.description?.trim()) return null;
  const escaped = c.description
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\n\n+/g, "</p><p>")
    .replace(/\n/g, "<br>");
  const createdMs = new Date(c.createdAt).getTime();
  const at = new Date(createdMs + 1000).toISOString();
  return {
    id: `description`,
    caseId: c.id,
    authorName:
      c.createdBy || c.customerContext.primaryContact || c.customer,
    authorRole: "customer",
    bodyHtml: `<p>${escaped}</p>`,
    createdAt: at,
  };
}

const TAB_DEFS: Array<{
  id: CaseTabId;
  label: string;
  icon: JSX.Element;
}> = [
  { id: "activities", label: "Activities", icon: <Activity size={16} /> },
  { id: "details", label: "Details", icon: <ListChecks size={16} /> },
  { id: "sla", label: "SLA", icon: <Clock size={16} /> },
  { id: "attachments", label: "Attachments", icon: <Paperclip size={16} /> },
  { id: "time", label: "Time tracking", icon: <Layers size={16} /> },
];

export default function CsmCaseDetailPage(): JSX.Element {
  const { caseId } = useParams<{ caseId: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const { data, isLoading, isError } = useGetCsmCaseDetail(caseId);
  const {
    data: comments,
    isLoading: isCommentsLoading,
    isError: isCommentsError,
  } = useGetCsmCaseComments(caseId);
  const postComment = usePostCsmCaseComment();
  const recordView = useRecordRecentView();
  const claims = useIdTokenClaims();
  const { showError } = useErrorBanner();
  const [feedback, setFeedback] = useState<Feedback | null>(null);
  const [activeTab, setActiveTab] = useState<CaseTabId>("activities");
  const [metaCollapsed, setMetaCollapsed] = useState(false);
  const [composerOpen, setComposerOpen] = useState(false);

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
    setActiveTab("activities");
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
        setTimeout(() => {
          target.style.transition = prevTransition;
        }, 350);
      }, 1500);
      return () => clearTimeout(reset);
    }, 250);
    return () => clearTimeout(timer);
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
      title: `${data.caseNumber} · ${data.subject}`,
      subtitle: `${data.customer} · ${data.projectName}`,
      href: `/cases/${data.id}`,
    });
  }, [data, recordView]);

  const onAction = useCallback(
    (action: CaseLifecycleAction | { secondary: string }) => {
      if (typeof action === "string") {
        // Lifecycle transition: sticky, semantically-colored feedback.
        setFeedback({
          message: LIFECYCLE_TOAST[action],
          severity: LIFECYCLE_SEVERITY[action],
          sticky: true,
        });
        return;
      }

      setFeedback({
        message: SECONDARY_TOAST[action.secondary] ?? `Action: ${action.secondary}`,
        severity: "info",
        sticky: false,
      });

      if (action.secondary === "copy_link" && data) {
        navigator.clipboard
          ?.writeText(`${window.location.origin}/cases/${data.id}`)
          .catch(() => showError("Could not copy link."));
      }
    },
    [data, showError],
  );

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
          onClick={() => navigate("/cases")}
          sx={{ alignSelf: "flex-start" }}
        >
          Back to cases
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
          onClick={() => navigate("/cases")}
          sx={{ alignSelf: "flex-start" }}
        >
          Back to cases
        </Button>
        <Typography variant="h5">Case not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No case with id <code>{caseId}</code> in the current mock dataset.
        </Typography>
      </Box>
    );
  }

  const c = data;
  const isClosed = c.state === "closed";
  const breached = c.minutesToBreach < 0;
  const canReopenClosed = (claims?.groups ?? []).some((g) =>
    LEAD_REOPEN_ROLES.has(g),
  );
  const rawComments = comments ?? [];
  // The description is the case's opening comment. It is injected into the
  // stream dated 1s after `created`, so with the default latest-first sort it
  // sits at the bottom as the oldest entry — read in context after the latest
  // activity, which is how engineers actually work a case here.
  const descriptionComment = buildDescriptionComment(c);
  const safeComments = descriptionComment
    ? [descriptionComment, ...rawComments]
    : rawComments;

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
        onClick={() => navigate("/cases")}
        sx={{ alignSelf: "flex-start" }}
      >
        Back to cases
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
            <Box component="span" sx={{ color: "text.secondary" }}>
              {c.wso2CaseId}
            </Box>
            <Box component="span" sx={{ color: "text.disabled", mx: 0.25 }}>
              /
            </Box>
            <Box component="span" sx={{ color: "text.primary" }}>
              {c.caseNumber}
            </Box>
          </Typography>

          {/* Status group (severity, lifecycle state, SLA) kept visually
              distinct from the free-form tags by a divider, so the current
              state doesn't get lost among the tag chips. */}
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
            <Chip
              size="small"
              label={`${c.severity} — ${SEVERITY_LABEL[c.severity]}`}
              color={SEVERITY_COLOR[c.severity]}
            />
            <Chip
              size="small"
              label={STATE_LABEL[c.state]}
              color={isClosed ? "success" : "primary"}
              sx={{ fontWeight: 600 }}
            />
            <Chip
              size="small"
              label={TIER_LABEL[c.customerContext.tier]}
              color={TIER_COLOR[c.customerContext.tier]}
            />
            {!isClosed && (
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
          <CaseActionBar
            caseDetail={c}
            onAction={onAction}
            canReopenClosed={canReopenClosed}
          />
        </Box>
      </Box>

      <CaseMetaBand
        detail={c}
        navigate={navigate}
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
                ? c.attachments.length
                : t.id === "time"
                  ? c.timeLogs.length
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

      {activeTab === "activities" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          {/* Comments are latest-in-top (WSO2 convention), so the composer
              belongs at the top. It stays collapsed behind a button to keep
              the thread the focal point until the engineer chooses to reply. */}
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
                onSubmit={async (bodyHtml, internal) => {
                  if (!caseId) return;
                  await postComment.mutateAsync({
                    caseId,
                    bodyHtml,
                    authorName: CURRENT_ENGINEER_NAME,
                    internal,
                  });
                  // Collapse only on success; on error the input keeps its
                  // draft and surfaces the failure.
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
              Compose a reply to the customer…
            </Button>
          )}

          <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
              <Typography variant="subtitle2">Activity timeline</Typography>
              {!isCommentsLoading && (
                <Chip
                  size="small"
                  variant="outlined"
                  label={`${safeComments.length + c.audit.length + c.attachments.length} entries`}
                />
              )}
            </Box>

            {isCommentsLoading ? (
              <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
                {[0, 1, 2].map((i) => (
                  <Skeleton key={i} variant="rectangular" height={56} />
                ))}
              </Box>
            ) : isCommentsError ? (
              <Typography variant="body2" color="error">
                Could not load comments.
              </Typography>
            ) : (
              <CaseActivitiesFeed
                comments={safeComments}
                audit={c.audit}
                attachments={c.attachments}
              />
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
              <MetaCell label="Case ID">
                <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
                  {c.caseNumber}
                </Typography>
              </MetaCell>
              <MetaCell label="WSO2 case ID">
                <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
                  {c.wso2CaseId}
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
            attachments={c.attachments}
            onDownloadAll={() => onAction({ secondary: "download_all_attachments" })}
            onDownload={() => onAction({ secondary: "download_attachment" })}
          />
        </Box>
      )}

      {activeTab === "time" && (
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: "1fr" }}>
          <TimeLogsWidget
            logs={c.timeLogs}
            onAdd={() => onAction({ secondary: "log_time" })}
          />
        </Box>
      )}
    </Box>
  );
}
