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

import { Box, Button, Chip, TablePagination, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type JSX,
  type ReactNode,
} from "react";
import { useLocation, useSearchParams } from "react-router";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import ColumnCustomizerButton from "@components/column-customizer/ColumnCustomizerButton";
import {
  getColumnPreferencesUserKey,
  useColumnPreferences,
} from "@hooks/useColumnPreferences";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { useNavTransition } from "@hooks/useNavTransition";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useBackendApi } from "@api/backend/client";
import FilteredCsvExportButton from "@components/FilteredCsvExportButton";
import CasesFilterBar, {
  type CasesFilters,
} from "@features/csm-cases/components/CasesFilterBar";
import CasesList from "@features/csm-cases/components/CasesList";
import {
  CASE_OPTIONAL_COLUMNS,
  type CaseOptionalColumnId,
} from "@features/csm-cases/utils/caseListColumns";
import { useGetCsmCases } from "@features/csm-cases/api/useGetCsmCases";
import {
  ASSIGNEE_FILTER_RESOLVED_EMPTY,
  buildCaseSearchFilters,
  mapCaseSearchViewToRow,
  resolveAssignedUserIds,
} from "@features/csm-cases/utils/caseSearchPayload";
import { useDirectoryUsers } from "@api/useDirectoryUsers";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { ALL_CASE_TYPES } from "@features/csm-cases/utils/caseType";
import {
  DEFAULT_CASES_FILTERS,
  readCasesFiltersFromUrl,
  writeCasesFiltersToUrl,
} from "@features/csm-cases/utils/casesFiltersUrl";
import {
  DEFAULT_CASES_SORT,
  type CasesSortField,
  type CasesSortOrder,
} from "@features/csm-cases/utils/casesSort";
import { stateLabel } from "@features/csm-dashboard/utils/abtDashboard";
import { WORK_STATE_LABEL } from "@features/csm-cases/utils/caseWorkState";
import RefreshButton from "@components/RefreshButton";
import type { BeCaseSearchPayload, BeCaseSearchResponse } from "@api/backend/types";
import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, DEFAULT_ROWS_PER_PAGE, BE_MAX_PAGE_LIMIT];

// URL params owned by the filter state; cleared/rewritten on change while any
// other params (e.g. a `tab` selection) are preserved. Must cover every key
// `writeCasesFiltersToUrl` can write — a key missing here never gets deleted
// when its filter clears back to empty/null, so the stale URL value keeps
// getting read back on the next render, making that one filter look
// impossible to fully clear (found via workStates: selecting a work state
// then trying to deselect it back to none silently failed because
// `workStates` wasn't in this list).
const FILTER_PARAM_KEYS = [
  "search",
  "severities",
  "states",
  "excludeStates",
  "types",
  "assignees",
  "workStates",
  "projects",
  "engagementTypes",
  "products",
  "csTeams",
  "sreTeams",
  "tags",
  "excludeTags",
  "onboardingStatuses",
  "slaPctGte",
  "slaPctLte",
  "escalation",
  "escalationLevels",
  "projectTypes",
  "createdFrom",
  "createdTo",
  "updatedFrom",
  "updatedTo",
  "closedFrom",
  "closedTo",
  // Same class of bug as `workStates` above, found the same way while
  // live-verifying the unified Advanced-mode row builder: removing the last
  // `filters.advancedFilters` row (or the last `anyOfBranches` OR-group) —
  // e.g. a "Created by is me" row with no typed `CasesFilters` slot to fall
  // back to — silently did nothing, because `af`/`anyOf` were missing here.
  // `writeCasesFiltersToUrl` only ever *sets* these two once there's at
  // least one row/branch again; it never explicitly clears them, so without
  // an explicit `delete` here first, the stale value from the previous URL
  // just kept getting read back on the next render.
  "af",
  "anyOf",
] as const;

interface CsmIssuesViewProps {
  /** Optional heading shown on the left of the header row. */
  title?: string;
  /** Optional right-aligned actions (e.g. a "Create" button). */
  actions?: ReactNode;
  /** Plural noun for the count subtitle / empty states. Default "cases". */
  entityNoun?: string;
  /** Filter values forced onto every query and hidden from the user (e.g. a
   *  locked case type or project). Merged over the user's URL filters. */
  lockedFilters?: Partial<CasesFilters>;
  /** Hide the case-type filter control (use when `lockedFilters` fixes it). */
  hideTypeFilter?: boolean;
  /**
   * Case type(s) to pre-select the FIRST time this view loads with no `types`
   * param in the URL at all -- unlike `lockedFilters`, this is a starting
   * point the user's own control can freely change (and once they do, their
   * choice round-trips through the URL like any other filter). Only
   * meaningful when the type control is visible (`!hideTypeFilter`); a
   * caller that locks and hides the type filter has no use for a separate
   * "default" on top of that lock. `CsmCasesPage` (Support) is the only
   * current user: it wants "Case" pre-selected on a fresh visit, but every
   * other type still one click away.
   */
  defaultCaseTypes?: CasesFilters["caseTypes"];
  /** Label for the case-type filter control; see `CasesFilterBar`'s own
   * `typeFilterLabel` doc comment. Defaults to "Case type". */
  typeFilterLabel?: string;
  /** Hide the project filter control (use when the view is project-scoped). */
  hideProjectFilter?: boolean;
  /**
   * Hide the "Onboarding status"/"CRE Team" Simple-mode controls — see
   * `CasesFilterBar`'s own doc comments. Pass when the view is already
   * scoped to one project (both are per-project attributes, so filtering by
   * them on a single-project view is a no-op).
   */
  hideOnboardingStatusFilter?: boolean;
  hideCreTeamFilter?: boolean;
  /** Show the engagement-type sub-filter (pass when the view is locked to engagement cases). */
  showEngagementTypeFilter?: boolean;
  /**
   * Force the Severity filter/column on or off, overriding the default
   * "only when `lockedFilters.caseTypes` is locked to `case`" rule below.
   * For a caller whose type filter is *unlocked and visible* (e.g. the
   * project Work items tab, which spans every case type but lets the user
   * narrow it with the "Work item type" control) that default would always
   * read false, permanently hiding Severity even once the user filters
   * down to just Cases. Pass `true` there — Severity is still a genuinely
   * useful control on a mixed list (non-case rows simply have no severity
   * to match, so picking a value implicitly narrows to cases, same as
   * picking "Case" in the type control would) — or `false` to force it off
   * regardless of the lock.
   */
  showSeverityFilter?: boolean;
  /** Base path for row detail links. Defaults to "/cases". */
  detailBasePath?: string;
  /** Hide the Severity column in the list (severity is a support-case
   * concept — SRA and Engagements don't surface it, but the main case list
   * keeps it). */
  hideSeverityColumn?: boolean;
  /**
   * Suppress this view's own "Back" button. Set when embedding this view as
   * a sub-tab of a page that already renders its own page-level Back button
   * (e.g. a project's Work items tab) — `location.state.from` is set on the
   * *route*, not per-tab, so an embedded view still sees it and would
   * otherwise render a second, redundant Back button pointing at the exact
   * same destination as the outer page's.
   */
  hideBackButton?: boolean;
  /**
   * Opt into the "Customise columns" control for this view's list (add,
   * remove, and reorder the optional columns between Subject and State),
   * persisted per user via `useColumnPreferences`. Off by default — enable
   * per view deliberately rather than changing every `CsmIssuesView` caller
   * (cases, service requests, security reports) at once. `columnsViewId`
   * must be unique across enabled views (defaults to `entityNoun`).
   */
  enableColumnCustomization?: boolean;
  /** Storage key suffix for this view's column layout. Defaults to
   * `entityNoun`; override only if two enabled views would otherwise share
   * an `entityNoun`. */
  columnsViewId?: string;
}

/**
 * Shared issues list: the cases filter bar + list + pagination, backed by
 * `POST /cases/search`. Reused for the all-cases page, the per-type lists
 * (service requests, security reports) and the project-scoped issues tab —
 * each just supplies `lockedFilters` (and hides the now-fixed control) so the
 * one component covers every "list issues of kind X" surface.
 */
export default function CsmIssuesView({
  title,
  actions,
  entityNoun = "cases",
  lockedFilters,
  hideTypeFilter,
  defaultCaseTypes,
  typeFilterLabel,
  hideProjectFilter,
  hideOnboardingStatusFilter,
  hideCreTeamFilter,
  showEngagementTypeFilter,
  showSeverityFilter: showSeverityFilterOverride,
  detailBasePath,
  hideSeverityColumn,
  hideBackButton,
  enableColumnCustomization,
  columnsViewId,
}: CsmIssuesViewProps): JSX.Element {
  const [searchParams, setSearchParams] = useSearchParams();
  const filters = useMemo<CasesFilters>(
    () => readCasesFiltersFromUrl(searchParams),
    [searchParams],
  );

  // Writes `defaultCaseTypes` into the URL at most once per mount, the first
  // time it finds no `types` param at all. Done as a real URL write (in an
  // effect, gated by a ref) rather than an in-memory override inside the
  // `filters` memo above: a memo can't tell "fresh visit" apart from "the
  // user cleared the type filter back to every type" -- both just look like
  // "no `types` param" -- so an in-memory default would keep reasserting
  // itself every time the user broadened back, making that choice
  // impossible to keep. Writing it into the URL once, right after mount,
  // means every later render (including a later clear) is driven purely by
  // `searchParams` like every other filter, with no special-casing.
  const defaultCaseTypesAppliedRef = useRef(false);
  useEffect(() => {
    if (defaultCaseTypesAppliedRef.current || !defaultCaseTypes?.length || hideTypeFilter) {
      return;
    }
    // Marks the default "consumed" on the very first render regardless of
    // whether it actually got written -- an initial URL that already names
    // an explicit `types` (e.g. a direct link to `?types=service_request`)
    // must permanently disarm the default too, or clearing that explicit
    // type back to "every type" later would look identical to "no types
    // param yet" and wrongly reassert `defaultCaseTypes` at that point.
    defaultCaseTypesAppliedRef.current = true;
    if (searchParams.has("types")) return;
    const next = new URLSearchParams(searchParams);
    next.set("types", defaultCaseTypes.join(","));
    setSearchParams(next, { replace: true });
  }, [searchParams, defaultCaseTypes, hideTypeFilter, setSearchParams]);

  const location = useLocation();
  const navigate = useNavTransition();
  // Set by DashboardWidgetTile's count/pie/bar click-throughs, since this
  // view has no dashboard context of its own (unlike the dashboard's
  // list-shape widget, whose embedded CasesList sets the same `from` shape
  // pointing at the dashboard itself). `location.state` belongs to the
  // *route*, not this component, so when this view is embedded as a
  // project's Work items sub-tab it still sees whatever `from` the project
  // page itself was reached with -- callers embedding it that way must pass
  // `hideBackButton`, since the outer page already renders its own Back
  // button to the same destination.
  const backTo = (location.state as { from?: string } | null)?.from;

  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const [isFiltersOpen, setIsFiltersOpen] = useState(true);
  const [sortField, setSortField] = useState<CasesSortField>(
    DEFAULT_CASES_SORT.field,
  );
  const [sortOrder, setSortOrder] = useState<CasesSortOrder>(
    DEFAULT_CASES_SORT.order,
  );

  const setFilters = useCallback(
    (next: CasesFilters) => {
      setPage(0);
      // Preserve any non-filter params (e.g. the active project-detail tab).
      const merged = new URLSearchParams(searchParams);
      FILTER_PARAM_KEYS.forEach((k) => merged.delete(k));
      writeCasesFiltersToUrl(next).forEach((v, k) => merged.set(k, v));
      setSearchParams(merged, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  // Severity (S1-S4) is a support-case concept, so the severity filter is by
  // default only shown on the support-cases list — i.e. when the surrounding
  // view locks the record type to `case`. Every other list (service
  // requests, engagements, security reports) hides it, unless the caller
  // overrides with `showSeverityFilter` (see its own doc comment — the
  // project Work items tab does this since its type filter is unlocked and
  // visible rather than fixed via `lockedFilters`).
  const showSeverityFilter =
    showSeverityFilterOverride ??
    (lockedFilters?.caseTypes?.length === 1 &&
      lockedFilters.caseTypes[0] === "case");
  // Service Requests don't carry a severity, so the column is redundant there.
  const isServiceRequestOnly =
    lockedFilters?.caseTypes?.length === 1 &&
    lockedFilters.caseTypes[0] === "service_request";

  const debouncedSearch = useDebouncedValue(filters.search, 300);
  // User filters (debounced search) with the locked overrides applied last so
  // the fixed type/project can't be widened by a stale URL value.
  const queryFilters = useMemo<CasesFilters>(
    () => {
      const merged: CasesFilters = {
        ...filters,
        search: debouncedSearch,
        // The severity control is hidden for non-case lists, so don't let a
        // stale `severities` value from a shared URL silently filter those
        // results.
        ...(showSeverityFilter ? {} : { severities: [] }),
        ...lockedFilters,
        // `lockedFilters.caseTypes` still drives the severity-filter/column
        // hints above (both keyed off the raw `lockedFilters` prop, not this
        // merged query) even when the type control itself is visible
        // (Support/`CsmCasesPage` — see its own doc comment) — but the
        // *query* must not be pinned to that lock once the user has a real,
        // working control to pick a different type from, or the control
        // would be visible yet functionally inert (picking a type would
        // silently have no effect on the results). Every other caller pairs
        // a `caseTypes` lock with `hideTypeFilter`, so this only changes
        // behavior for the one caller that doesn't.
        ...(hideTypeFilter ? {} : { caseTypes: filters.caseTypes }),
      };
      // An unlocked, empty type selection means "no type filter applied" from
      // the FE's perspective (every issue type shown — this is the only
      // unlocked, multi-type view; every other CsmIssuesView caller locks
      // `caseTypes` to a single value and hides the control). But
      // `useGetCsmCases` omits an empty `caseTypes` from the search payload
      // entirely, and the entity-service defaults an absent/empty `types`
      // filter to support cases only (`default_case`) rather than "no
      // restriction" — so an omitted filter silently narrows the result to
      // one type instead of returning all of them. Send every known type
      // explicitly in that case so the BE default can't kick in.
      if (merged.caseTypes.length === 0) {
        merged.caseTypes = ALL_CASE_TYPES;
      }
      return merged;
    },
    [filters, debouncedSearch, showSeverityFilter, lockedFilters, hideTypeFilter],
  );

  const {
    data,
    isLoading,
    isFetching,
    isError,
    error,
    refetch,
    dataUpdatedAt,
  } = useGetCsmCases(queryFilters, page, rowsPerPage, true, sortOrder, sortField);

  const handleSortFieldChange = (field: CasesSortField): void => {
    setSortField(field);
    setPage(0);
  };

  const handleSortOrderChange = (order: CasesSortOrder): void => {
    setSortOrder(order);
    setPage(0);
  };

  const { data: directoryUsers } = useDirectoryUsers();
  const { showError } = useErrorBanner();
  const hasShownErrorRef = useRef(false);

  // Export CSV: pages `/cases/search` with the exact same filters/sort as the
  // listing above (`buildCaseSearchFilters`/`resolveAssignedUserIds` are the
  // same helpers `useGetCsmCases` uses), independent of the table's own
  // page/rowsPerPage — the export always fetches the *whole* filtered result
  // set, not whatever page happens to be on screen. See
  // `useFilteredCsvExport`/`fetchAllPages` for the paging + truncation logic.
  const api = useBackendApi();
  const currentUserEmail = useIdTokenClaims()?.email;
  const currentUserId = useCurrentUser().user?.id;

  // "Customise columns" — off unless a caller opts in (see `enableColumnCustomization`'s
  // doc). `showSeverityColumn` mirrors the exact gate `CasesList` itself is given below
  // (`isServiceRequestOnly || hideSeverityColumn`) so the picker can never offer a
  // Severity column that would just render "—" for every row (service requests have no
  // severity concept at all).
  const showSeverityColumn = !(isServiceRequestOnly || hideSeverityColumn);
  // Every current caller of this view locks the case type via `lockedFilters` (Case, SR,
  // SRA, Engagements each fix `caseTypes` to one value) — the Type column would then show
  // the same chip on every row, so it's still offered (a legacy row that predates type
  // tagging renders "—" there, which is a genuine signal) but left off by default. An
  // unlocked, multi-type view (none exists among today's callers) would want it on by
  // default, hence the `isLockedToSingleType` check rather than hard-coding this off.
  const isLockedToSingleType = lockedFilters?.caseTypes?.length === 1;
  const availableOptionalColumns: CaseOptionalColumnId[] = [
    "product",
    "type",
    "issueType",
    ...(showSeverityColumn ? (["severity"] as const) : []),
    "assignee",
    "createdBy",
    "customer",
    "createdAt",
    "escalationLevel",
  ];
  const defaultVisibleOptionalColumns: CaseOptionalColumnId[] = [
    "product",
    ...(isLockedToSingleType ? [] : (["type"] as const)),
    ...(showSeverityColumn ? (["severity"] as const) : []),
  ];
  const columnPrefs = useColumnPreferences({
    viewId: `case-list:${columnsViewId ?? entityNoun}`,
    userKey: getColumnPreferencesUserKey({ id: currentUserId, email: currentUserEmail }),
    columns: availableOptionalColumns.map((id) => ({ id, label: CASE_OPTIONAL_COLUMNS[id].label })),
    defaultVisibleIds: defaultVisibleOptionalColumns,
  });
  const exportSearch = queryFilters.search.trim();
  const fetchCasesExportPage = useCallback(
    async (offset: number, limit: number) => {
      // Re-resolved per page when an assignee filter is active — a small,
      // deterministic, targeted `/users/search` by email, not a directory
      // scan — rather than caching it across pages, so the export can never
      // serve a stale resolution if this closure is reused across a filter
      // change mid-export (it isn't today, but this keeps that safe by
      // construction rather than by care).
      let assignedUserIds: string[] | undefined;
      if (queryFilters.assignees.length > 0) {
        const resolved = await resolveAssignedUserIds(
          api,
          queryFilters.assignees,
          currentUserId,
        );
        if (resolved === ASSIGNEE_FILTER_RESOLVED_EMPTY) {
          return { items: [] as CsmCaseRow[], total: 0 };
        }
        assignedUserIds = resolved;
      }
      const res = await api.post<BeCaseSearchPayload, BeCaseSearchResponse>(
        "/cases/search",
        {
          pagination: { offset, limit },
          sortBy: { field: sortField, order: sortOrder },
          filters: buildCaseSearchFilters(queryFilters, exportSearch, assignedUserIds),
        },
      );
      const items = (res.cases ?? []).map((c) => mapCaseSearchViewToRow(c, currentUserEmail));
      return { items, total: res.total };
    },
    [api, queryFilters, exportSearch, sortField, sortOrder, currentUserId, currentUserEmail],
  );

  // Same gate `CasesList` is given (`hideSeverityColumn={isServiceRequestOnly
  // || hideSeverityColumn}`) so the export's column set can never drift from
  // what's actually rendered.
  const showExportSeverityColumn = !(isServiceRequestOnly || hideSeverityColumn);
  const caseToCsvRow = useCallback(
    (c: CsmCaseRow): string[] => {
      const caseId =
        c.wso2CaseId && c.caseNumber
          ? `${c.wso2CaseId}/${c.caseNumber}`
          : c.wso2CaseId || c.caseNumber || "";
      const updated = formatBackendTimestampForDisplay(c.updatedAt, {
        year: "numeric",
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
      const stateText =
        stateLabel(c.state) +
        (c.state === "work_in_progress" && c.workState
          ? ` (${WORK_STATE_LABEL[c.workState]})`
          : "");
      const row = [caseId, c.subject, c.projectName, c.product];
      if (showExportSeverityColumn) row.push(c.severity);
      row.push(stateText, updated ?? "");
      return row;
    },
    [showExportSeverityColumn],
  );
  const exportHeader = [
    "Case ID",
    "Subject",
    "Project",
    "Product",
    ...(showExportSeverityColumn ? ["Severity"] : []),
    "State",
    "Updated",
  ];

  useEffect(() => {
    if (isError && !hasShownErrorRef.current) {
      hasShownErrorRef.current = true;
      showError(`Could not load ${entityNoun}.`, error);
    }
    if (!isError) hasShownErrorRef.current = false;
  }, [isError, error, showError, entityNoun]);

  const cases = data?.cases ?? [];

  const availableAssigneeUsers = useMemo(() => {
    const list = (directoryUsers ?? [])
      .filter((u) => u.name)
      .map((u) => ({ name: u.name, email: u.email }));
    list.sort((a, b) => a.name.localeCompare(b.name));
    return list;
  }, [directoryUsers]);

  const availableProjects = useMemo(() => {
    const byId = new Map<string, string>();
    (data?.cases ?? []).forEach((c) => {
      if (c.projectId) byId.set(c.projectId, c.projectName || c.projectId);
    });
    return Array.from(byId, ([id, name]) => ({ id, name }));
  }, [data?.cases]);

  const total = data?.total ?? 0;
  const lastPage = total === 0 ? 0 : Math.ceil(total / rowsPerPage) - 1;
  // Clamp to the last valid page when the loaded set shrinks (filter change, rows
  // closing). React's documented pattern for adjusting state from changed inputs
  // is a guarded set during render — not an effect (the lint rule forbids
  // setState in effects); React re-renders before committing, so it's not a
  // user-visible extra paint.
  if (data !== undefined && !data.hasMore && page > lastPage) {
    setPage(lastPage);
  }
  const paginationCount = data === undefined || data.hasMore ? -1 : total;

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  const breachedCount = cases.filter(
    (c) => c.minutesToBreach < 0 && c.state !== "closed",
  ).length;
  const rangeStart = total === 0 ? 0 : page * rowsPerPage + 1;
  const rangeEnd = page * rowsPerPage + cases.length;

  const subtitle =
    isLoading
      ? null
      : total === 0
        ? `No ${entityNoun}`
        : `Showing ${rangeStart}–${rangeEnd} of ${total}`;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {backTo && !hideBackButton && (
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate(backTo)}
          sx={{ alignSelf: "flex-start" }}
        >
          Back
        </Button>
      )}
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 2,
          flexWrap: "wrap",
        }}
      >
        <Box>
          {title && <Typography variant="h5">{title}</Typography>}
          {subtitle != null && (
            <Typography variant="body2" color="text.secondary">
              {subtitle}
            </Typography>
          )}
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          {breachedCount > 0 && (
            <Chip size="small" color="error" label={`${breachedCount} breached`} />
          )}
          <RefreshButton
            onRefresh={() => void refetch()}
            isFetching={isFetching}
            updatedAt={dataUpdatedAt}
            label={`Refresh ${entityNoun}`}
          />
          <FilteredCsvExportButton<CsmCaseRow>
            entityName={entityNoun.replace(/\s+/g, "-")}
            entityNounPlural={entityNoun}
            header={exportHeader}
            toRow={caseToCsvRow}
            fetchPage={fetchCasesExportPage}
            disabled={isError || total === 0}
          />
          {actions}
        </Box>
      </Box>

      <CasesFilterBar
        filters={filters}
        onChange={setFilters}
        onReset={() => setFilters(DEFAULT_CASES_FILTERS)}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen((v) => !v)}
        availableAssigneeUsers={availableAssigneeUsers}
        availableProjects={availableProjects}
        showSeverityFilter={showSeverityFilter}
        hideTypeFilter={hideTypeFilter}
        typeFilterLabel={typeFilterLabel}
        hideProjectFilter={hideProjectFilter}
        hideOnboardingStatusFilter={hideOnboardingStatusFilter}
        hideCreTeamFilter={hideCreTeamFilter}
        showEngagementTypeFilter={showEngagementTypeFilter}
      />

      <CasesList
        cases={cases}
        isLoading={isLoading || isFetching}
        skeletonCount={rowsPerPage}
        detailBasePath={detailBasePath}
        hideSeverityColumn={isServiceRequestOnly || hideSeverityColumn}
        optionalColumns={
          enableColumnCustomization
            ? columnPrefs.visibleColumns.map((c) => c.id as CaseOptionalColumnId)
            : undefined
        }
        columnCustomizer={
          enableColumnCustomization ? (
            <ColumnCustomizerButton
              allColumns={columnPrefs.allColumns}
              isVisible={columnPrefs.isVisible}
              onToggle={columnPrefs.toggleColumn}
              onMove={columnPrefs.moveColumn}
              onReorder={columnPrefs.reorderColumn}
              onReset={columnPrefs.resetToDefault}
              label={`Customise ${entityNoun} columns`}
            />
          ) : undefined
        }
        sortField={sortField}
        onSortFieldChange={handleSortFieldChange}
        sortOrder={sortOrder}
        onSortOrderChange={handleSortOrderChange}
      />

      <TablePagination
        component="div"
        count={paginationCount}
        page={page}
        onPageChange={(_, newPage) => setPage(newPage)}
        rowsPerPage={rowsPerPage}
        onRowsPerPageChange={handleChangeRowsPerPage}
        rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
        labelRowsPerPage={`${entityNoun[0].toUpperCase()}${entityNoun.slice(1)} per page`}
        showFirstButton
        showLastButton
      />
    </Box>
  );
}
