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
  Activity,
  ArrowLeft,
  Clock,
  Layers,
  Link as LinkIcon,
  ListChecks,
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
  AuditTimelineWidget,
  CustomerContextWidget,
  LinkedItemsWidget,
  ProductContextWidget,
  SlaTimelineWidget,
  TimeLogsWidget,
  WatchersWidget,
} from "@features/csm-cases/components/CaseDetailWidgets";
import { useRecordRecentView } from "@features/csm-recent/hooks/useRecentViews";
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
};

type CaseTabId = "activities" | "details" | "sla" | "related" | "time";

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
  { id: "related", label: "Related", icon: <LinkIcon size={16} /> },
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
  const { showError } = useErrorBanner();
  const [toast, setToast] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<CaseTabId>("activities");
  const [metaCollapsed, setMetaCollapsed] = useState(false);

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
    if (!toast) return;
    const t = setTimeout(() => setToast(null), 2500);
    return () => clearTimeout(t);
  }, [toast]);

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
      const message =
        typeof action === "string"
          ? LIFECYCLE_TOAST[action]
          : SECONDARY_TOAST[action.secondary] ?? `Action: ${action.secondary}`;
      setToast(message);

      if (typeof action !== "string" && action.secondary === "copy_link" && data) {
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
  const rawComments = comments ?? [];
  const descriptionComment = buildDescriptionComment(c);
  // The description renders as the first comment in the activity stream,
  // dated 1s after `created` so it always lands directly after that event.
  const safeComments = descriptionComment
    ? [descriptionComment, ...rawComments]
    : rawComments;

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
          <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
            <Typography variant="overline" color="text.secondary">
              {c.caseNumber}
            </Typography>
            <Chip
              size="small"
              label={`${c.severity} — ${SEVERITY_LABEL[c.severity]}`}
              color={SEVERITY_COLOR[c.severity]}
            />
            <Chip
              size="small"
              label={STATE_LABEL[c.state]}
              variant="outlined"
              color={isClosed ? "success" : "primary"}
            />
            {!isClosed && (
              <Chip
                size="small"
                variant="outlined"
                color={breached ? "error" : c.minutesToBreach <= 60 ? "warning" : "default"}
                label={`${SLA_CLOCK_LABEL[c.slaClockType]} · ${formatTimeToBreach(c.minutesToBreach)}`}
              />
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
        navigate={navigate}
        collapsed={metaCollapsed}
        onToggleCollapsed={() => setMetaCollapsed((v) => !v)}
      />

      {toast && (
        <Box
          sx={{
            p: 1,
            borderRadius: 1,
            backgroundColor: "info.50",
            border: 1,
            borderColor: "info.main",
            color: "info.main",
            fontSize: "0.875rem",
          }}
        >
          {toast}
        </Box>
      )}

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={activeTab}
          onChange={(_, v) => setActiveTab(v as CaseTabId)}
          variant="scrollable"
          scrollButtons="auto"
        >
          {TAB_DEFS.map((t) => (
            <Tab
              key={t.id}
              value={t.id}
              icon={t.icon}
              iconPosition="start"
              label={t.label}
              sx={{ minHeight: 44, textTransform: "none" }}
            />
          ))}
        </Tabs>
      </Box>

      {activeTab === "activities" && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Typography variant="subtitle2">Reply</Typography>
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
              }}
            />
          </Card>

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
          <CustomerContextWidget ctx={c.customerContext} />
          <ProductContextWidget ctx={c.productContext} />
          <WatchersWidget
            watchers={c.watchers}
            onAdd={() => onAction({ secondary: "manage_watchers" })}
          />
          <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Typography variant="subtitle2">Timestamps</Typography>
            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: "1fr 1fr",
                gap: 2,
              }}
            >
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

      {activeTab === "related" && (
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: "1fr" }}>
          <LinkedItemsWidget
            items={c.linkedItems}
            onLink={() => onAction({ secondary: "link_case" })}
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
