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

import { Box, Chip, IconButton, Tooltip, Typography } from "@wso2/oxygen-ui";
import { Eye } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useState, type JSX, type ReactNode } from "react";
import { useLocation } from "react-router";
import type {
  BeCaseFeedback,
  BeCaseSearchView,
  BeIncident,
  BeChangeRequestSearchView,
  BeProblemSearchView,
  BeIncidentTaskSearchView,
  BeTimeCardView,
  BeTaskSummary,
  BeWidgetResourceType,
} from "@api/backend/types";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { useNavTransition } from "@hooks/useNavTransition";
import {
  getColumnPreferencesUserKey,
  useColumnPreferences,
} from "@hooks/useColumnPreferences";
import ColumnCustomizerButton from "@components/column-customizer/ColumnCustomizerButton";
import CasesList from "@features/csm-cases/components/CasesList";
import { mapCaseSearchViewToRow } from "@features/csm-cases/utils/caseSearchPayload";
import {
  CASE_OPTIONAL_COLUMNS,
  type CaseOptionalColumnId,
} from "@features/csm-cases/utils/caseListColumns";
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
  /** Only read by `CaseWidgetList` today (to gate the Severity column and to
   * key its own "Customise columns" preferences per resourceType) — every
   * other renderer below ignores it. */
  resourceType: BeWidgetResourceType;
  /**
   * Lets a renderer that has its own "Customise columns" button (today,
   * only `CaseWidgetList`) hand that button up to the caller instead of
   * rendering it in its own toolbar row — `DashboardWidgetTile.tsx` uses
   * this to put it next to the tile's existing refresh button (same line)
   * rather than splitting the two across separate rows, reported live as
   * reading like two unrelated controls. Called with the built button
   * element whenever the caller supplies this (and the renderer re-renders
   * with new column state), or `null` right before this component
   * unmounts, so a caller holding it in state can clear it. When omitted
   * (every other current caller), `CaseWidgetList` keeps rendering its own
   * button in-place, exactly as before this prop existed.
   */
  onColumnCustomizerChange?: (node: ReactNode | null) => void;
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
 * me" highlight, not relevant to a dashboard preview). This renderer is
 * shared by every `resourceType` whose rows are case rows
 * (`service_request`/`security_report_analysis`/`announcement`/`engagement`
 * — see `WIDGET_LIST_RENDERERS` below), so they all get the same columns,
 * gated the same way `CsmIssuesView`/`CaseFamilyWidgetPreview` gate theirs:
 * Severity only where it's a real concept (`case`), and it's shown by
 * default since that matched this renderer's own long-standing hardcoded
 * set before "Customise columns" existed here at all — only Product/Type/
 * (Severity)/Assignee were ever on by default, so that default is
 * preserved exactly, just now genuinely editable (and, for a non-`case`
 * resourceType, no longer offering a Severity column that only ever
 * rendered "—"). Preferences are keyed per resourceType, not per widget, so
 * every "case"-shaped tile across the dashboard (there can be more than
 * one) shares one layout — the same granularity the main list pages use. */
function CaseWidgetList({
  items,
  isLoading,
  resourceType,
  onColumnCustomizerChange,
}: WidgetListRendererProps): JSX.Element {
  const cases = items.map((item) =>
    mapCaseSearchViewToRow(item as unknown as BeCaseSearchView, undefined),
  );

  const currentUserId = useCurrentUser().user?.id;
  const currentUserEmail = useIdTokenClaims()?.email;
  const showSeverityColumn = resourceType === "case";
  // Memoized so `columnPrefs`'s own `allColumns`/etc. stay referentially
  // stable across renders that don't actually change anything — without
  // this, a fresh array literal on every render made `useColumnPreferences`
  // recompute (and return new references for) `allColumns` on every render
  // too, which made the `onColumnCustomizerChange` effect below re-fire
  // every render, which (via the caller's own setState) re-rendered this
  // component, which created a new array again: an infinite loop. Content
  // only actually changes when `showSeverityColumn` does.
  const availableOptionalColumns = useMemo<CaseOptionalColumnId[]>(
    () => [
      "product",
      "type",
      "issueType",
      ...(showSeverityColumn ? (["severity"] as const) : []),
      "assignee",
      "createdBy",
      "customer",
      "createdAt",
    ],
    [showSeverityColumn],
  );
  const defaultVisibleOptionalColumns = useMemo<CaseOptionalColumnId[]>(
    () => [
      "product",
      "type",
      ...(showSeverityColumn ? (["severity"] as const) : []),
      "assignee",
    ],
    [showSeverityColumn],
  );
  const columnOptions = useMemo(
    () => availableOptionalColumns.map((id) => ({ id, label: CASE_OPTIONAL_COLUMNS[id].label })),
    [availableOptionalColumns],
  );
  const columnPrefs = useColumnPreferences({
    viewId: `case-list:dashboard-tile-${resourceType}`,
    userKey: getColumnPreferencesUserKey({ id: currentUserId, email: currentUserEmail }),
    columns: columnOptions,
    defaultVisibleIds: defaultVisibleOptionalColumns,
  });

  const columnCustomizerButton = (
    <ColumnCustomizerButton
      allColumns={columnPrefs.allColumns}
      isVisible={columnPrefs.isVisible}
      onToggle={columnPrefs.toggleColumn}
      onMove={columnPrefs.moveColumn}
      onReorder={columnPrefs.reorderColumn}
      onReset={columnPrefs.resetToDefault}
      label="Customise columns"
    />
  );

  // `onColumnCustomizerChange` lets a caller (the dashboard tile, so it can
  // show this next to its own refresh button instead of in a separate row)
  // take over where the button renders. Re-hands it up whenever the
  // underlying column state actually changes (these callbacks are only ever
  // new references when `state` itself changes — see `useColumnPreferences`),
  // and clears it on unmount so the caller doesn't keep holding a stale node.
  useEffect(() => {
    if (!onColumnCustomizerChange) return;
    onColumnCustomizerChange(columnCustomizerButton);
    return () => onColumnCustomizerChange(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- re-fires exactly when the button's own inputs change, per the comment above.
  }, [
    onColumnCustomizerChange,
    columnPrefs.allColumns,
    columnPrefs.isVisible,
    columnPrefs.toggleColumn,
    columnPrefs.moveColumn,
    columnPrefs.reorderColumn,
    columnPrefs.resetToDefault,
  ]);

  return (
    <CasesList
      cases={cases}
      isLoading={isLoading}
      skeletonCount={4}
      hideSeverityColumn={!showSeverityColumn}
      optionalColumns={columnPrefs.visibleColumns.map((c) => c.id as CaseOptionalColumnId)}
      columnCustomizer={onColumnCustomizerChange ? undefined : columnCustomizerButton}
    />
  );
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
          PREVIEW_COLUMN,
          { label: "Number", width: "minmax(90px, 0.7fr)" },
          { label: "Subject", width: "minmax(160px, 2fr)" },
          { label: "State", width: "minmax(90px, 1fr)" },
          { label: "Priority", width: "minmax(90px, 1fr)" },
          { label: "Updated", width: "minmax(90px, 1fr)" },
        ]}
        rows={incidents.map((incident, i) => {
          const href = incident.id ? `/operations/incidents/${incident.id}` : undefined;
          const label = incident.number || incident.subject || "incident";
          return {
            key: incident.id ?? `incident-${i}`,
            onClick: href ? () => navigate(href, { state: dashboardReturnState }) : undefined,
            cells: [
              previewCell(label, () => setPreviewIncident(incident)),
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
          PREVIEW_COLUMN,
          { label: "Number", width: "minmax(90px, 0.7fr)" },
          { label: "Subject", width: "minmax(160px, 2fr)" },
          { label: "State", width: "minmax(100px, 1fr)" },
          { label: "Impact", width: "minmax(90px, 1fr)" },
          { label: "Updated", width: "minmax(90px, 1fr)" },
        ]}
        rows={changeRequests.map((cr, i) => {
          const href = cr.id ? `/operations/change-requests/${cr.id}` : undefined;
          const label = cr.number || cr.subject || "change request";
          return {
            key: cr.id ?? `cr-${i}`,
            onClick: href ? () => navigate(href, { state: dashboardReturnState }) : undefined,
            cells: [
              previewCell(label, () => setPreviewChangeRequest(cr)),
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
          PREVIEW_COLUMN,
          { label: "Number", width: "minmax(90px, 0.7fr)" },
          { label: "Subject", width: "minmax(160px, 2fr)" },
          { label: "State", width: "minmax(100px, 1fr)" },
          { label: "Assigned to", width: "minmax(100px, 1fr)" },
        ]}
        rows={problems.map((problem, i) => {
          const href = problem.id ? `/operations/problems/${problem.id}` : undefined;
          const label = problem.number || problem.subject || "problem";
          return {
            key: problem.id ?? `problem-${i}`,
            onClick: href ? () => navigate(href, { state: dashboardReturnState }) : undefined,
            cells: [
              previewCell(label, () => setPreviewProblem(problem)),
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
            ],
          };
        })}
      />
      <ProblemPreviewDrawer problem={previewProblem} onClose={() => setPreviewProblem(null)} />
    </>
  );
}

/** Incident task: no standalone list or detail page exists for incident
 * tasks in this app (unlike `problem`), so — mirroring `CallRequestWidgetList`
 * below linking a call request's rows to its owning case — rows navigate
 * straight to the owning incident's real detail page instead. Deliberately
 * simpler than `ProblemWidgetList`: no preview drawer, since there is no
 * incident-task-specific preview surface worth building for a resource with
 * no detail page of its own to preview into (a plain list is an honest
 * simplification here, not a shortcut). `stateLabel` (pre-humanized by the
 * data source) is used for the state chip rather than `state` (a raw,
 * data-source-specific integer — see `BeIncidentTaskSearchView.state`). */
function IncidentTaskWidgetList({ items, isLoading }: WidgetListRendererProps): JSX.Element {
  const incidentTasks = items as unknown as BeIncidentTaskSearchView[];
  const dashboardReturnState = useDashboardReturnState();
  const navigate = useNavTransition();
  return (
    <DashboardMiniTable
      isLoading={isLoading}
      emptyMessage="No incident tasks match this widget's filters."
      columns={[
        { label: "Number", width: "minmax(90px, 0.7fr)" },
        { label: "Subject", width: "minmax(160px, 2fr)" },
        { label: "State", width: "minmax(100px, 1fr)" },
        { label: "Assignment group", width: "minmax(120px, 1fr)" },
        { label: "Assigned to", width: "minmax(100px, 1fr)" },
      ]}
      rows={incidentTasks.map((task, i) => {
        const incidentId = task.incident?.id;
        const href = incidentId ? `/operations/incidents/${incidentId}` : undefined;
        return {
          key: task.id ?? `incident-task-${i}`,
          onClick: href ? () => navigate(href, { state: dashboardReturnState }) : undefined,
          cells: [
            <Typography key="number" variant="body2" noWrap>
              {task.number || "—"}
            </Typography>,
            <Typography key="subject" variant="body2" noWrap title={task.subject ?? undefined}>
              {task.subject || "—"}
            </Typography>,
            task.stateLabel ? (
              <Chip key="state" size="small" variant="outlined" label={task.stateLabel} />
            ) : (
              <Typography key="state" variant="body2">
                —
              </Typography>
            ),
            <Typography key="assignmentGroup" variant="body2" noWrap>
              {task.assignmentGroup?.name || "—"}
            </Typography>,
            <Typography key="assignedTo" variant="body2" noWrap>
              {task.assignedTo?.name || "—"}
            </Typography>,
          ],
        };
      })}
    />
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
          PREVIEW_COLUMN,
          { label: "Name", width: "minmax(140px, 2fr)" },
          { label: "Tier", width: "minmax(90px, 1fr)" },
          { label: "Region", width: "minmax(90px, 1fr)" },
        ]}
        rows={accounts.map((a) => {
          const tier = resolveAccountTier(a);
          return {
            key: a.id,
            onClick: () => navigate(`/customers/accounts/${a.id}`, { state: dashboardReturnState }),
            cells: [
              previewCell(a.name, () => setPreviewAccount(a)),
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
          PREVIEW_COLUMN,
          { label: "Name", width: "minmax(140px, 2fr)" },
          { label: "Project key", width: "minmax(90px, 1fr)" },
          { label: "State", width: "minmax(100px, 1fr)" },
        ]}
        rows={projects.map((p) => ({
          key: p.id,
          onClick: () => navigate(`/customers/projects/${p.id}`, { state: dashboardReturnState }),
          cells: [
            previewCell(p.name, () => setPreviewProject(p)),
            <Typography key="name" variant="body2" noWrap title={p.name}>
              {p.name}
            </Typography>,
            <Typography key="key" variant="body2" noWrap>
              {p.key}
            </Typography>,
            <ClosureStateChip key="state" closureState={p.closureState} emptyFallback="—" />,
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
          PREVIEW_COLUMN,
          { label: "User", width: "minmax(140px, 2fr)" },
          { label: "Email", width: "minmax(140px, 2fr)" },
          { label: "Status", width: "minmax(80px, 1fr)" },
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
            previewCell(u.name || u.userName, () => setPreviewUser(u)),
            <Typography key="user" variant="body2" noWrap>
              {u.userName}
            </Typography>,
            <Typography key="email" variant="body2" noWrap>
              {u.email}
            </Typography>,
            <Typography key="status" variant="body2">
              {u.active === undefined ? "—" : u.active ? "Active" : "Inactive"}
            </Typography>,
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
          PREVIEW_COLUMN,
          { label: "CVE / ID", width: "minmax(100px, 1fr)" },
          { label: "Product", width: "minmax(120px, 2fr)" },
          { label: "Priority", width: "minmax(90px, 1fr)" },
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
              previewCell(label, () => setPreviewVulnerability(vuln)),
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
          PREVIEW_COLUMN,
          { label: "Number", width: "minmax(90px, 0.7fr)" },
          { label: "Reason", width: "minmax(160px, 2fr)" },
          { label: "State", width: "minmax(100px, 1fr)" },
          { label: "Scheduled", width: "minmax(90px, 1fr)" },
        ]}
        rows={callRequests.map((cr, i) => {
          const href = cr.case?.id ? `/cases/${cr.case.id}` : undefined;
          const label = cr.number || cr.reason || "call request";
          return {
            key: cr.id ?? `call-request-${i}`,
            onClick: href ? () => navigate(href, { state: dashboardReturnState }) : undefined,
            cells: [
              previewCell(label, () => setPreviewCallRequest(cr)),
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

/**
 * Hardcoded renderer for `case_feedback` — the primary renderer for this
 * resourceType's `shape: "list"` widget (the `case-feedback.json` dashboard's
 * own list widget sets no `columns`, so `DashboardWidgetTile`'s
 * `hasColumns` branch always dispatches here rather than to the generic
 * `GenericColumnList` — a widget that *does* set `columns` would use that
 * path instead, same relationship `columns` has to every other hardcoded
 * renderer here). Renders rating/comment/case/submitted-by/submitted.
 */
function CaseFeedbackWidgetList({ items, isLoading }: WidgetListRendererProps): JSX.Element {
  const feedback = items as unknown as BeCaseFeedback[];
  const dashboardReturnState = useDashboardReturnState();
  const navigate = useNavTransition();
  return (
    <DashboardMiniTable
      isLoading={isLoading}
      emptyMessage="No feedback records match this widget's filters."
      columns={[
        { label: "Rating", width: "minmax(90px, 0.6fr)" },
        { label: "Comment", width: "minmax(200px, 3fr)" },
        { label: "Case", width: "minmax(140px, 1fr)" },
        { label: "Submitted by", width: "minmax(140px, 1.5fr)" },
        { label: "Submitted", width: "minmax(90px, 1fr)" },
      ]}
      rows={feedback.map((f, i) => {
        const href = f.caseId ? `/cases/${f.caseId}` : undefined;
        return {
          key: f.instanceId ?? `feedback-${i}`,
          onClick: href ? () => navigate(href, { state: dashboardReturnState }) : undefined,
          cells: [
            <Typography key="rating" variant="body2" noWrap>
              {f.ratingLabel || "—"}
            </Typography>,
            <Typography key="comment" variant="body2" noWrap title={f.comment ?? undefined}>
              {f.comment || "—"}
            </Typography>,
            <Box key="case" sx={{ minWidth: 0 }}>
              {f.caseInternalId && (
                <Typography
                  variant="body2"
                  noWrap
                  title={f.caseInternalId}
                  sx={{ fontFamily: "monospace", fontWeight: 600 }}
                >
                  {f.caseInternalId}
                </Typography>
              )}
              <Typography
                variant={f.caseInternalId ? "caption" : "body2"}
                color={f.caseInternalId ? "text.secondary" : undefined}
                noWrap
                sx={{ fontFamily: "monospace", display: "block" }}
              >
                {f.caseNumber || f.caseId || "—"}
              </Typography>
            </Box>,
            <Typography key="submitter" variant="body2" noWrap title={f.submitterName ?? undefined}>
              {f.submitterName || "Customer"}
            </Typography>,
            <Typography key="submitted" variant="caption" color="text.secondary" noWrap>
              {formatDate(f.submittedAt)}
            </Typography>,
          ],
        };
      })}
    />
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
  incident_task: IncidentTaskWidgetList,
  account: AccountWidgetList,
  project: ProjectWidgetList,
  user: UserWidgetList,
  time_card: TimeCardWidgetList,
  product_vulnerability: ProductVulnerabilityWidgetList,
  task: TaskWidgetList,
  call_request: CallRequestWidgetList,
  case_feedback: CaseFeedbackWidgetList,
};
