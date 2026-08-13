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

/* eslint-disable react-refresh/only-export-components -- this is a config module of per-resourceType render helpers (like widgetResourceConfig.ts), not a component module; none of the individual XxxWidgetList functions are exported (fast-refresh DX only) */

import { Chip, IconButton, Tooltip, Typography } from "@wso2/oxygen-ui";
import { Eye } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useLocation } from "react-router";
import type {
  BeCaseSearchView,
  BeIncident,
  BeChangeRequestSearchView,
  BeProblemSearchView,
  BeTimeCardView,
  BeTaskSummary,
  BeWidgetResourceType,
} from "@api/backend/types";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useNavTransition } from "@hooks/useNavTransition";
import CasesList from "@features/csm-cases/components/CasesList";
import { mapCaseSearchViewToRow } from "@features/csm-cases/utils/caseSearchPayload";
import TimeCardsTable from "@features/csm-timecards/components/TimeCardsTable";
import { mapTimeCard } from "@features/csm-timecards/api/useTimeSheets";
import DashboardMiniTable from "@features/csm-dashboard/components/DashboardMiniTable";
import {
  incidentPriorityColor,
  incidentPriorityLabel,
  incidentStateColor,
  incidentStateLabel,
} from "@features/csm-operations/utils/incidents";
import IncidentPreviewDrawer from "@features/csm-operations/components/IncidentPreviewDrawer";
import {
  changeRequestImpactColor,
  changeRequestImpactLabel,
  changeRequestStateColor,
  changeRequestStateLabel,
} from "@features/csm-operations/utils/changeRequests";
import ChangeRequestPreviewDrawer from "@features/csm-operations/components/ChangeRequestPreviewDrawer";
import { problemStateColor, problemStateLabel } from "@features/csm-operations/utils/problems";
import ProblemPreviewDrawer from "@features/csm-operations/components/ProblemPreviewDrawer";
import { taskStateColor, taskStateLabel } from "@features/csm-cases/utils/taskState";
import { TaskDetailDialog } from "@features/csm-cases/components/TaskDetailDialog";
import CallRequestDetailModal from "@features/csm-cases/components/CallRequestDetailModal";
import { resolveAccountTier, type Account } from "@features/csm-accounts/types/csmAccounts";
import AccountPreviewDrawer from "@features/csm-accounts/components/AccountPreviewDrawer";
import type { Project } from "@features/csm-projects/types/csmProjects";
import ClosureStateChip from "@features/csm-projects/components/ClosureStateChip";
import ProjectPreviewDrawer from "@features/csm-projects/components/ProjectPreviewDrawer";
import { normalizeUser, type User, type SnUser } from "@features/csm-users/types/csmUsers";
import UserPreviewDrawer from "@features/csm-users/components/UserPreviewDrawer";
import { vulnerabilityPriorityColor } from "@features/csm-security-center/utils/vulnerabilities";
import ProductVulnerabilityPreviewDrawer from "@features/csm-security-center/components/ProductVulnerabilityPreviewDrawer";
import type { BeProductVulnerabilityView } from "@api/backend/types";
import type { BeCallRequestView } from "@api/backend/types";

/** Raw item shape a dashboard widget's `/search` response resolves to —
 * matches `WidgetItem` in `widgetResourceConfig.ts` (kept loose there since
 * that file's `primaryLabel`/`secondaryLabel` extractors are resourceType-
 * agnostic); each renderer below casts it to the same typed shape its own
 * tab already assumes, since it's the identical upstream response. */
type WidgetItem = Record<string, unknown>;

/** Router state carried on every list-shape widget row's navigation (every
 * renderer below other than `CaseWidgetList`/`TimeCardWidgetList`, which
 * embed their own tab's real list component and so already get this for
 * free — see `CasesList`'s own `useLocation()` call), so the destination
 * page's own Back button can return to this exact dashboard instead of
 * falling through to a hardcoded/generic destination. */
function useDashboardReturnState(): { from: string } {
  const location = useLocation();
  return { from: `${location.pathname}${location.search}` };
}

function formatDate(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, { year: "numeric", month: "short", day: "numeric" }) ??
    "—"
  );
}

/** Date + time, for columns where same-day values must stay distinguishable
 * (e.g. a call request's scheduled time) -- `formatDate` alone drops the
 * hour/minute and collapses same-day rows to an identical-looking value. */
function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
    }) ?? "—"
  );
}

export interface WidgetListRendererProps {
  items: WidgetItem[];
  isLoading: boolean;
}

/**
 * The always-visible quick-preview `Eye` icon cell, shared by every
 * `DashboardMiniTable`-based renderer below (every resourceType other than
 * `case`/`time_card`, which embed their own tab's real list component and
 * already have this — see `CasesList`/`TimeCardsTable`). `stopPropagation`
 * keeps the click from also bubbling up into the row's own `onClick` (which
 * would navigate to the full record instead of just previewing it).
 */
function previewCell(label: string, onPreview: () => void): JSX.Element {
  return (
    <Tooltip key="preview" title={`Quick preview ${label}`}>
      <IconButton
        size="small"
        aria-label={`Quick preview ${label}`}
        onClick={(e) => {
          e.stopPropagation();
          onPreview();
        }}
      >
        <Eye size={16} />
      </IconButton>
    </Tooltip>
  );
}

const PREVIEW_COLUMN = { label: "Preview", width: "auto" };

/** Case: reuses `CasesList` (the Cases tab's own table) verbatim, via the
 * same `mapCaseSearchViewToRow` mapper the tab itself uses — real reuse, not
 * a lookalike. `currentUserEmail` is omitted (only affects the "assigned to
 * me" highlight, not relevant to a dashboard preview). */
function CaseWidgetList({ items, isLoading }: WidgetListRendererProps): JSX.Element {
  const cases = items.map((item) =>
    mapCaseSearchViewToRow(item as unknown as BeCaseSearchView, undefined),
  );
  return <CasesList cases={cases} isLoading={isLoading} skeletonCount={4} />;
}

/** Time card: reuses `TimeCardsTable` verbatim via the tab's own `mapTimeCard`
 * mapper. Actions are off — approve/reject doesn't belong on a dashboard
 * preview — but the always-on "view details" eye icon still works. */
function TimeCardWidgetList({ items, isLoading }: WidgetListRendererProps): JSX.Element {
  const cards = items.map((item) => mapTimeCard(item as unknown as BeTimeCardView));
  return (
    <TimeCardsTable
      cards={cards}
      isLoading={isLoading}
      skeletonCount={4}
      emptyText="No time cards match this widget's filters."
      groupBy="case"
      showActionsColumn={false}
      roleFor={() => ({ isOwner: false, isApprover: false, isAdmin: false })}
      onCardAction={() => {}}
    />
  );
}

function IncidentWidgetList({ items, isLoading }: WidgetListRendererProps): JSX.Element {
  const incidents = items as unknown as BeIncident[];
  const dashboardReturnState = useDashboardReturnState();
  const navigate = useNavTransition();
  const [previewIncident, setPreviewIncident] = useState<BeIncident | null>(null);
  return (
    <>
      <DashboardMiniTable
        isLoading={isLoading}
        emptyMessage="No incidents match this widget's filters."
        columns={[
          { label: "Number", width: "minmax(90px, 0.7fr)" },
          { label: "Subject", width: "minmax(160px, 2fr)" },
          { label: "State", width: "minmax(90px, 1fr)" },
          { label: "Priority", width: "minmax(90px, 1fr)" },
          { label: "Updated", width: "minmax(90px, 1fr)" },
          PREVIEW_COLUMN,
        ]}
        rows={incidents.map((incident, i) => {
          const href = incident.id ? `/operations/incidents/${incident.id}` : undefined;
          const label = incident.number || incident.subject || "incident";
          return {
            key: incident.id ?? `incident-${i}`,
            onClick: href ? () => navigate(href, { state: dashboardReturnState }) : undefined,
            cells: [
              <Typography key="number" variant="body2" noWrap>
                {incident.number || "—"}
              </Typography>,
              <Typography key="subject" variant="body2" noWrap title={incident.subject ?? undefined}>
                {incident.subject || "—"}
              </Typography>,
              incident.state ? (
                <Chip
                  key="state"
                  size="small"
                  variant="outlined"
                  color={incidentStateColor(incident.state)}
                  label={incidentStateLabel(incident.state)}
                />
              ) : (
                <Typography key="state" variant="body2">
                  —
                </Typography>
              ),
              incident.priority ? (
                <Chip
                  key="priority"
                  size="small"
                  variant="outlined"
                  color={incidentPriorityColor(incident.priority)}
                  label={incidentPriorityLabel(incident.priority)}
                />
              ) : (
                <Typography key="priority" variant="body2">
                  —
                </Typography>
              ),
              <Typography key="updated" variant="caption" color="text.secondary" noWrap>
                {formatDate(incident.updatedOn)}
              </Typography>,
              previewCell(label, () => setPreviewIncident(incident)),
            ],
          };
        })}
      />
      <IncidentPreviewDrawer incident={previewIncident} onClose={() => setPreviewIncident(null)} />
    </>
  );
}

function ChangeRequestWidgetList({ items, isLoading }: WidgetListRendererProps): JSX.Element {
  const changeRequests = items as unknown as BeChangeRequestSearchView[];
  const dashboardReturnState = useDashboardReturnState();
  const navigate = useNavTransition();
  const [previewChangeRequest, setPreviewChangeRequest] =
    useState<BeChangeRequestSearchView | null>(null);
  return (
    <>
      <DashboardMiniTable
        isLoading={isLoading}
        emptyMessage="No change requests match this widget's filters."
        columns={[
          { label: "Number", width: "minmax(90px, 0.7fr)" },
          { label: "Subject", width: "minmax(160px, 2fr)" },
          { label: "State", width: "minmax(100px, 1fr)" },
          { label: "Impact", width: "minmax(90px, 1fr)" },
          { label: "Updated", width: "minmax(90px, 1fr)" },
          PREVIEW_COLUMN,
        ]}
        rows={changeRequests.map((cr, i) => {
          const href = cr.id ? `/operations/change-requests/${cr.id}` : undefined;
          const label = cr.number || cr.subject || "change request";
          return {
            key: cr.id ?? `cr-${i}`,
            onClick: href ? () => navigate(href, { state: dashboardReturnState }) : undefined,
            cells: [
              <Typography key="number" variant="body2" noWrap>
                {cr.number || "—"}
              </Typography>,
              <Typography key="subject" variant="body2" noWrap title={cr.subject ?? undefined}>
                {cr.subject || "—"}
              </Typography>,
              cr.state ? (
                <Chip
                  key="state"
                  size="small"
                  variant="outlined"
                  color={changeRequestStateColor(cr.state)}
                  label={changeRequestStateLabel(cr.state)}
                />
              ) : (
                <Typography key="state" variant="body2">
                  —
                </Typography>
              ),
              cr.impact ? (
                <Chip
                  key="impact"
                  size="small"
                  variant="outlined"
                  color={changeRequestImpactColor(cr.impact)}
                  label={changeRequestImpactLabel(cr.impact)}
                />
              ) : (
                <Typography key="impact" variant="body2">
                  —
                </Typography>
              ),
              <Typography key="updated" variant="caption" color="text.secondary" noWrap>
                {formatDate(cr.updatedOn)}
              </Typography>,
              previewCell(label, () => setPreviewChangeRequest(cr)),
            ],
          };
        })}
      />
      <ChangeRequestPreviewDrawer
        changeRequest={previewChangeRequest}
        onClose={() => setPreviewChangeRequest(null)}
      />
    </>
  );
}

function ProblemWidgetList({ items, isLoading }: WidgetListRendererProps): JSX.Element {
  const problems = items as unknown as BeProblemSearchView[];
  const dashboardReturnState = useDashboardReturnState();
  const navigate = useNavTransition();
  const [previewProblem, setPreviewProblem] = useState<BeProblemSearchView | null>(null);
  return (
    <>
      <DashboardMiniTable
        isLoading={isLoading}
        emptyMessage="No problems match this widget's filters."
        columns={[
          { label: "Number", width: "minmax(90px, 0.7fr)" },
          { label: "Subject", width: "minmax(160px, 2fr)" },
          { label: "State", width: "minmax(100px, 1fr)" },
          { label: "Assigned to", width: "minmax(100px, 1fr)" },
          PREVIEW_COLUMN,
        ]}
        rows={problems.map((problem, i) => {
          const href = problem.id ? `/operations/problems/${problem.id}` : undefined;
          const label = problem.number || problem.subject || "problem";
          return {
            key: problem.id ?? `problem-${i}`,
            onClick: href ? () => navigate(href, { state: dashboardReturnState }) : undefined,
            cells: [
              <Typography key="number" variant="body2" noWrap>
                {problem.number || "—"}
              </Typography>,
              <Typography key="subject" variant="body2" noWrap title={problem.subject ?? undefined}>
                {problem.subject || "—"}
              </Typography>,
              problem.state ? (
                <Chip
                  key="state"
                  size="small"
                  variant="outlined"
                  color={problemStateColor(problem.state)}
                  label={problemStateLabel(problem.state)}
                />
              ) : (
                <Typography key="state" variant="body2">
                  —
                </Typography>
              ),
              <Typography key="assignedTo" variant="body2" noWrap>
                {problem.assignedTo?.name || "—"}
              </Typography>,
              previewCell(label, () => setPreviewProblem(problem)),
            ],
          };
        })}
      />
      <ProblemPreviewDrawer problem={previewProblem} onClose={() => setPreviewProblem(null)} />
    </>
  );
}

function AccountWidgetList({ items, isLoading }: WidgetListRendererProps): JSX.Element {
  const accounts = items as unknown as Account[];
  const dashboardReturnState = useDashboardReturnState();
  const navigate = useNavTransition();
  const [previewAccount, setPreviewAccount] = useState<Account | null>(null);
  return (
    <>
      <DashboardMiniTable
        isLoading={isLoading}
        emptyMessage="No accounts match this widget's filters."
        columns={[
          { label: "Name", width: "minmax(140px, 2fr)" },
          { label: "Tier", width: "minmax(90px, 1fr)" },
          { label: "Region", width: "minmax(90px, 1fr)" },
          PREVIEW_COLUMN,
        ]}
        rows={accounts.map((a) => {
          const tier = resolveAccountTier(a);
          return {
            key: a.id,
            onClick: () => navigate(`/customers/accounts/${a.id}`, { state: dashboardReturnState }),
            cells: [
              <Typography key="name" variant="body2" noWrap title={a.name}>
                {a.name}
              </Typography>,
              tier ? (
                <Chip key="tier" size="small" variant="outlined" label={tier} />
              ) : (
                <Typography key="tier" variant="body2">
                  —
                </Typography>
              ),
              <Typography key="region" variant="body2" noWrap>
                {a.region ?? "—"}
              </Typography>,
              previewCell(a.name, () => setPreviewAccount(a)),
            ],
          };
        })}
      />
      <AccountPreviewDrawer account={previewAccount} onClose={() => setPreviewAccount(null)} />
    </>
  );
}

function ProjectWidgetList({ items, isLoading }: WidgetListRendererProps): JSX.Element {
  const projects = items as unknown as Project[];
  const dashboardReturnState = useDashboardReturnState();
  const navigate = useNavTransition();
  const [previewProject, setPreviewProject] = useState<Project | null>(null);
  return (
    <>
      <DashboardMiniTable
        isLoading={isLoading}
        emptyMessage="No projects match this widget's filters."
        columns={[
          { label: "Name", width: "minmax(140px, 2fr)" },
          { label: "Project key", width: "minmax(90px, 1fr)" },
          { label: "State", width: "minmax(100px, 1fr)" },
          PREVIEW_COLUMN,
        ]}
        rows={projects.map((p) => ({
          key: p.id,
          onClick: () => navigate(`/customers/projects/${p.id}`, { state: dashboardReturnState }),
          cells: [
            <Typography key="name" variant="body2" noWrap title={p.name}>
              {p.name}
            </Typography>,
            <Typography key="key" variant="body2" noWrap>
              {p.key}
            </Typography>,
            <ClosureStateChip key="state" closureState={p.closureState} emptyFallback="—" />,
            previewCell(p.name, () => setPreviewProject(p)),
          ],
        }))}
      />
      <ProjectPreviewDrawer project={previewProject} onClose={() => setPreviewProject(null)} />
    </>
  );
}

function UserWidgetList({ items, isLoading }: WidgetListRendererProps): JSX.Element {
  const users = items.map((item) => normalizeUser(item as unknown as User | SnUser));
  const dashboardReturnState = useDashboardReturnState();
  const navigate = useNavTransition();
  const [previewUser, setPreviewUser] = useState<(typeof users)[number] | null>(null);
  return (
    <>
      <DashboardMiniTable
        isLoading={isLoading}
        emptyMessage="No users match this widget's filters."
        columns={[
          { label: "User", width: "minmax(140px, 2fr)" },
          { label: "Email", width: "minmax(140px, 2fr)" },
          { label: "Status", width: "minmax(80px, 1fr)" },
          PREVIEW_COLUMN,
        ]}
        rows={users.map((u) => ({
          key: u.id,
          onClick: () =>
            navigate(`/people/${encodeURIComponent(u.id)}`, { state: dashboardReturnState }),
          // Plain text, not `UserRefLink` — that renders its own nested
          // RouterLink with no `state`, so clicking the name specifically
          // (vs. elsewhere in the row) would silently drop `dashboardReturnState`
          // and land on a plain default back target instead. The row itself
          // is already the link (with state), matching every sibling widget's
          // "name" cell (see AccountWidgetList/ProjectWidgetList above).
          cells: [
            <Typography key="user" variant="body2" noWrap>
              {u.userName}
            </Typography>,
            <Typography key="email" variant="body2" noWrap>
              {u.email}
            </Typography>,
            <Typography key="status" variant="body2">
              {u.active === undefined ? "—" : u.active ? "Active" : "Inactive"}
            </Typography>,
            previewCell(u.name || u.userName, () => setPreviewUser(u)),
          ],
        }))}
      />
      <UserPreviewDrawer user={previewUser} onClose={() => setPreviewUser(null)} />
    </>
  );
}

function ProductVulnerabilityWidgetList({ items, isLoading }: WidgetListRendererProps): JSX.Element {
  const vulnerabilities = items as unknown as BeProductVulnerabilityView[];
  const dashboardReturnState = useDashboardReturnState();
  const navigate = useNavTransition();
  const [previewVulnerability, setPreviewVulnerability] =
    useState<BeProductVulnerabilityView | null>(null);
  return (
    <>
      <DashboardMiniTable
        isLoading={isLoading}
        emptyMessage="No vulnerabilities match this widget's filters."
        columns={[
          { label: "CVE / ID", width: "minmax(100px, 1fr)" },
          { label: "Product", width: "minmax(120px, 2fr)" },
          { label: "Priority", width: "minmax(90px, 1fr)" },
          PREVIEW_COLUMN,
        ]}
        rows={vulnerabilities.map((vuln) => {
          const label = vuln.cveId || vuln.vulnerabilityId || "vulnerability";
          return {
            key: vuln.id,
            onClick: () =>
              navigate(`/security-center/vulnerabilities/${encodeURIComponent(vuln.id)}`, {
                state: dashboardReturnState,
              }),
            cells: [
              <Typography key="cve" variant="body2" noWrap sx={{ fontFamily: "monospace" }}>
                {vuln.cveId || vuln.vulnerabilityId || "—"}
              </Typography>,
              <Typography key="product" variant="body2" noWrap>
                {vuln.productName || "—"}
              </Typography>,
              vuln.priority ? (
                <Chip
                  key="priority"
                  size="small"
                  variant="outlined"
                  color={vulnerabilityPriorityColor(vuln.priority)}
                  label={vuln.priority}
                />
              ) : (
                <Typography key="priority" variant="body2">
                  —
                </Typography>
              ),
              previewCell(label, () => setPreviewVulnerability(vuln)),
            ],
          };
        })}
      />
      <ProductVulnerabilityPreviewDrawer
        vulnerability={previewVulnerability}
        onClose={() => setPreviewVulnerability(null)}
      />
    </>
  );
}

/** Task: no standalone list page exists (tasks are only ever shown inside a
 * case's own Tasks tab or this dialog), so rows open {@link TaskDetailDialog}
 * in place rather than navigating -- that dialog shows the call/task details
 * and its own real link through to the parent case, which is the actual
 * destination a row click should reach (a task is not a first-class page of
 * its own). */
function TaskWidgetList({ items, isLoading }: WidgetListRendererProps): JSX.Element {
  const tasks = items as unknown as BeTaskSummary[];
  const [openTaskId, setOpenTaskId] = useState<string | null>(null);
  return (
    <>
      <DashboardMiniTable
        isLoading={isLoading}
        emptyMessage="No tasks match this widget's filters."
        columns={[
          { label: "Subject", width: "minmax(160px, 2fr)" },
          { label: "State", width: "minmax(90px, 1fr)" },
          { label: "Assigned to", width: "minmax(100px, 1fr)" },
          { label: "Updated", width: "minmax(90px, 1fr)" },
        ]}
        rows={tasks.map((task, i) => ({
          key: task.id ?? `task-${i}`,
          onClick: task.id ? () => setOpenTaskId(task.id) : undefined,
          cells: [
            <Typography key="subject" variant="body2" noWrap title={task.subject ?? undefined}>
              {task.subject || "—"}
            </Typography>,
            task.state ? (
              <Chip
                key="state"
                size="small"
                variant="outlined"
                color={taskStateColor(task.state)}
                label={taskStateLabel(task.state)}
              />
            ) : (
              <Typography key="state" variant="body2">
                —
              </Typography>
            ),
            <Typography key="assignedTo" variant="body2" noWrap>
              {task.assignedTo?.name || "—"}
            </Typography>,
            <Typography key="updated" variant="caption" color="text.secondary" noWrap>
              {formatDate(task.updatedOn)}
            </Typography>,
          ],
        }))}
      />
      {openTaskId && (
        <TaskDetailDialog taskId={openTaskId} onClose={() => setOpenTaskId(null)} />
      )}
    </>
  );
}

/** Call request: unlike task, `CallRequestView.case.id` is always present, so
 * rows navigate straight to the owning case's real detail page rather than
 * opening a dialog. The quick-preview icon reuses `CallRequestDetailModal`
 * verbatim — the same read-only dialog `CallRequestsTable`'s own eye icon
 * already opens, entirely from fields already on `BeCallRequestView`, so
 * there's no reason to duplicate it as a second, lookalike drawer. */
function CallRequestWidgetList({ items, isLoading }: WidgetListRendererProps): JSX.Element {
  const callRequests = items as unknown as BeCallRequestView[];
  const dashboardReturnState = useDashboardReturnState();
  const navigate = useNavTransition();
  const [previewCallRequest, setPreviewCallRequest] = useState<BeCallRequestView | null>(null);
  return (
    <>
      <DashboardMiniTable
        isLoading={isLoading}
        emptyMessage="No call requests match this widget's filters."
        columns={[
          { label: "Number", width: "minmax(90px, 0.7fr)" },
          { label: "Reason", width: "minmax(160px, 2fr)" },
          { label: "State", width: "minmax(100px, 1fr)" },
          { label: "Scheduled", width: "minmax(90px, 1fr)" },
          PREVIEW_COLUMN,
        ]}
        rows={callRequests.map((cr, i) => {
          const href = cr.case?.id ? `/cases/${cr.case.id}` : undefined;
          const label = cr.number || cr.reason || "call request";
          return {
            key: cr.id ?? `call-request-${i}`,
            onClick: href ? () => navigate(href, { state: dashboardReturnState }) : undefined,
            cells: [
              <Typography key="number" variant="body2" noWrap>
                {cr.number || "—"}
              </Typography>,
              <Typography key="reason" variant="body2" noWrap title={cr.reason ?? undefined}>
                {cr.reason || "—"}
              </Typography>,
              cr.state?.label ? (
                <Chip key="state" size="small" variant="outlined" label={cr.state.label} />
              ) : (
                <Typography key="state" variant="body2">
                  —
                </Typography>
              ),
              <Typography key="scheduled" variant="caption" color="text.secondary" noWrap>
                {formatDateTime(cr.scheduleTime)}
              </Typography>,
              previewCell(label, () => setPreviewCallRequest(cr)),
            ],
          };
        })}
      />
      {previewCallRequest && (
        <CallRequestDetailModal
          callRequest={previewCallRequest}
          onClose={() => setPreviewCallRequest(null)}
        />
      )}
    </>
  );
}

/** Per-resourceType renderer for a `shape: "list"` dashboard widget. Every
 * resource type is covered — `WIDGET_RESOURCE_CONFIG` (in
 * `widgetResourceConfig.ts`) is keyed the same way, so a missing entry here
 * would be a compile error, not a silent gap. */
export const WIDGET_LIST_RENDERERS: Record<
  BeWidgetResourceType,
  (props: WidgetListRendererProps) => JSX.Element
> = {
  case: CaseWidgetList,
  // service_request / security_report_analysis / announcement / engagement
  // all route to the same /cases/search response shape as `case` (see
  // widgetResourceConfig.ts) -- their rows are case rows, so they reuse
  // CaseWidgetList verbatim rather than a lookalike renderer.
  service_request: CaseWidgetList,
  security_report_analysis: CaseWidgetList,
  announcement: CaseWidgetList,
  engagement: CaseWidgetList,
  incident: IncidentWidgetList,
  change_request: ChangeRequestWidgetList,
  problem: ProblemWidgetList,
  account: AccountWidgetList,
  project: ProjectWidgetList,
  user: UserWidgetList,
  time_card: TimeCardWidgetList,
  product_vulnerability: ProductVulnerabilityWidgetList,
  task: TaskWidgetList,
  call_request: CallRequestWidgetList,
};
