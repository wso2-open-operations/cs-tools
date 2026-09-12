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
  Chip,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  TablePagination,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import type { BeWidgetResourceType } from "@api/backend/types";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import {
  getColumnPreferencesUserKey,
  useColumnPreferences,
} from "@hooks/useColumnPreferences";
import { useWidgetData } from "@features/csm-dashboard/api/useWidgetData";
import { useTeams } from "@features/csm-dashboard/api/useTeams";
import ColumnCustomizerButton from "@components/column-customizer/ColumnCustomizerButton";
import RefreshButton from "@components/RefreshButton";
import { WIDGET_LIST_RENDERERS } from "@features/csm-dashboard/config/widgetListConfig";
import {
  CASE_OPTIONAL_COLUMNS,
  type CaseOptionalColumnId,
} from "@features/csm-cases/utils/caseListColumns";
import {
  callRequestWidgetFiltersToQuery,
  DEFAULT_CALL_REQUEST_WIDGET_FILTERS,
  resourceTypeForPreviewSlug,
  translateCallRequestDashboardFilters,
  translateCaseDashboardFilters,
  type CallRequestWidgetFilters,
} from "@features/csm-dashboard/config/widgetResourceConfig";
import {
  describeWidgetFilters,
  isAnyOfBranchArray,
  parseWidgetPreviewFilters,
  resolveCurrentUserSentinels,
} from "@features/csm-dashboard/utils/widgetPreviewUrl";
import CasesFilterBar, {
  type CasesFilters,
} from "@features/csm-cases/components/CasesFilterBar";
import CasesList from "@features/csm-cases/components/CasesList";
import { useGetCsmCases } from "@features/csm-cases/api/useGetCsmCases";
import { DEFAULT_CASES_FILTERS } from "@features/csm-cases/utils/casesFiltersUrl";
import DateRangeFilter, {
  type DateRangeFilterValue,
} from "@features/csm-dashboard/components/DateRangeFilter";
import MultiSelectField from "@components/MultiSelectField";
import TriStateMultiSelectField from "@components/TriStateMultiSelectField";
import AsyncUserIdMultiSelect from "@features/csm-cases/components/AsyncUserIdMultiSelect";
import { STATE_OPTIONS } from "@features/csm-cases/utils/caseFilterOptions";
import {
  ALL_CALL_REQUEST_STATES,
  CALL_REQUEST_STATE_LABEL,
} from "@features/csm-cases/utils/callRequestState";

const DEFAULT_ROWS_PER_PAGE = 10;
const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];
const SEARCH_DEBOUNCE_MS = 300;

/**
 * Every widget `resourceType` that ultimately queries `/cases/search` (see
 * `WIDGET_RESOURCE_CONFIG` in `widgetResourceConfig.ts` — they all share the
 * same response shape and the same `CaseWidgetList` renderer). Reported
 * live: for these, a static "Filtered by:" chip summary read wrong next to
 * every other list page in the app, which uses a real, editable filter bar
 * — so this "View more" destination stays its own dedicated, bookmarkable
 * route (that part was correct), but its filter UI is now the actual
 * `CasesFilterBar` + `useGetCsmCases` + `CasesList`, the same trio the Cases
 * tab itself uses, seeded from the widget's own filters via
 * `translateCaseDashboardFilters` and then fully editable from there.
 *
 * Exception: a widget carrying `anyOf` (cross-field OR — see
 * `isAnyOfBranchArray`) skips this editable path entirely and falls through
 * to the plain `DashboardWidgetPreviewContent` below instead (see the
 * `!isAnyOfBranchArray(...)` check at the call site) — `CasesFilters` has no
 * OR construct to seed `CasesFilterBar` with, so `translateCaseDashboardFilters`
 * would have to silently drop `anyOf` the same way `casesHref`'s own
 * click-through used to (the bug `caseFamilyBuildHref` in
 * `widgetResourceConfig.ts` exists to close). `DashboardWidgetPreviewContent`
 * posts the widget's raw, un-translated filters straight to `/cases/search`
 * via `useWidgetData` — the same request shape the tile's own count used —
 * so its result set is guaranteed to match, at the cost of a plain search box
 * instead of a fully editable filter bar for just this one case.
 */
const CASE_FAMILY_RESOURCE_TYPES = new Set<BeWidgetResourceType>([
  "case",
  "service_request",
  "security_report_analysis",
  "announcement",
  "engagement",
]);

/**
 * "View more" landing for a dashboard `shape: "list"` widget tile — the same
 * per-resourceType table the tile itself renders (see `widgetListConfig.tsx`;
 * e.g. cases render through the identical `CasesList` the Cases tab uses),
 * paginated (real `TablePagination`, not just a bigger fixed fetch) so a
 * viewer can browse the widget's whole matching set from here without
 * leaving to the resource's own tab, plus a free-text search box merged
 * into the widget's own filters (`searchQuery` — the same field every other
 * resource search in this app already uses).
 *
 * Fully URL-driven (see `buildWidgetPreviewHref` in `widgetPreviewUrl.ts`)
 * rather than router-state-based, so the page is bookmarkable/shareable and
 * survives a refresh: the resource type is `:previewSlug` in the path, the
 * widget's own id/display name are `w`/`n` query params, and each filter
 * field is its own readable query param (the signed-in user's own id, where
 * present, is masked to `@me` rather than embedded verbatim). A URL with no
 * recognizable `previewSlug` or missing required params falls back to a
 * "go to the dashboard" prompt instead of crashing.
 */
export default function DashboardWidgetPreviewPage(): JSX.Element {
  const { previewSlug } = useParams<{ previewSlug: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { user, isLoading: isLoadingUser } = useCurrentUser();

  const resourceType = resourceTypeForPreviewSlug(previewSlug);
  const widgetId = searchParams.get("w");
  const displayName = searchParams.get("n");
  const { filters: rawFilters, needsCurrentUser } =
    parseWidgetPreviewFilters(searchParams);

  const backButton = (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={() => navigate("/dashboard")}
      sx={{ alignSelf: "flex-start" }}
    >
      Back
    </Button>
  );

  if (!resourceType || !widgetId || !displayName) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {backButton}
        <Typography variant="body2" color="text.secondary">
          Open this page from a dashboard widget&rsquo;s &ldquo;View
          more&rdquo; link.
        </Typography>
      </Box>
    );
  }

  // A widget filtered to "assigned to me" carries the `@me` sentinel until
  // the signed-in user's own id is known — hold off resolving/querying
  // until then rather than ever sending the literal placeholder upstream.
  if (needsCurrentUser && !user?.id) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {backButton}
        <Typography variant="h5">{displayName}</Typography>
        <Typography variant="body2" color="text.secondary">
          {isLoadingUser
            ? "Loading…"
            : "Could not resolve the signed-in user for this widget."}
        </Typography>
      </Box>
    );
  }

  const filters = resolveCurrentUserSentinels(rawFilters, user?.id);

  if (CASE_FAMILY_RESOURCE_TYPES.has(resourceType) && !isAnyOfBranchArray(filters.anyOf)) {
    return (
      <CaseFamilyWidgetPreview
        displayName={displayName}
        filters={filters}
        backButton={backButton}
        resourceType={resourceType}
      />
    );
  }

  if (resourceType === "call_request") {
    return (
      <CallRequestWidgetPreview
        widgetId={widgetId}
        displayName={displayName}
        filters={filters}
        backButton={backButton}
      />
    );
  }

  if (resourceType === "case_feedback") {
    return (
      <CaseFeedbackWidgetPreview
        widgetId={widgetId}
        displayName={displayName}
        filters={filters}
        backButton={backButton}
      />
    );
  }

  return (
    <DashboardWidgetPreviewContent
      widgetId={widgetId}
      displayName={displayName}
      resourceType={resourceType}
      filters={filters}
      backButton={backButton}
    />
  );
}

interface CaseFamilyWidgetPreviewProps {
  displayName: string;
  filters: Record<string, unknown>;
  backButton: JSX.Element;
  resourceType: BeWidgetResourceType;
}

/**
 * The `useColumnPreferences` `viewId` this preview's "Customise columns"
 * picker should share with the corresponding main list page, for a
 * `resourceType` whose main page routes through `CsmIssuesView`/`CasesList`
 * (the exact same `CaseOptionalColumnId` column vocabulary this preview's
 * own `CasesList` renders) — so a viewer's column choices carry over between
 * "View more" and the real tab. `announcement` is deliberately excluded: its
 * main page (`CsmAnnouncementsPage.tsx`) has its own, unrelated column set
 * (`ANNOUNCEMENT_COLUMNS`, a different bespoke table), so reusing its
 * `"announcements"` viewId here would misapply a saved preference built for
 * an entirely different set of column ids. Any resourceType not listed here
 * (including `announcement`) falls back to its own dedicated bucket.
 */
const CASE_FAMILY_COLUMNS_VIEW_ID: Partial<Record<BeWidgetResourceType, string>> = {
  case: "cases",
  service_request: "service-requests",
  security_report_analysis: "security-reports",
  engagement: "engagements",
};

function columnsViewIdForResourceType(resourceType: BeWidgetResourceType): string {
  return CASE_FAMILY_COLUMNS_VIEW_ID[resourceType] ?? `dashboard-preview-${resourceType}`;
}

/**
 * The case-family "View more" landing: a real, editable `CasesFilterBar`
 * seeded from the widget's own filters (translated once via
 * `translateCaseDashboardFilters`, the same function that already builds
 * the tile's own click-through URL), feeding the actual `useGetCsmCases` +
 * `CasesList` the Cases tab itself uses — not a read-only render of
 * whatever the widget happened to be configured with.
 *
 * `CasesFilterBar`'s tag control is tri-state (digiops-cs#2907) — a tag can
 * be included, excluded, or left unselected, shown as its own `+`/`-` chip
 * — so a widget's `tag notIn [X]` now seeds `excludeTags` directly, the
 * same value the dashboard tile itself queries. No approximation needed:
 * what's shown and what's queried are the same `CasesFilters` value from
 * the start, kept in one piece of state.
 */
function CaseFamilyWidgetPreview({
  displayName,
  filters,
  backButton,
  resourceType,
}: CaseFamilyWidgetPreviewProps): JSX.Element {
  // Frozen once at mount (a fresh "View more"/slice click is a fresh mount)
  // so a later Reset restores what the widget actually linked here with,
  // not whatever the viewer has since edited.
  const [initial] = useState<CasesFilters>(() => ({
    ...DEFAULT_CASES_FILTERS,
    ...translateCaseDashboardFilters(filters),
  }));

  const [casesFilters, setCasesFilters] = useState<CasesFilters>(initial);
  const [isFiltersOpen, setIsFiltersOpen] = useState(true);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const handleFiltersChange = (next: CasesFilters): void => {
    setCasesFilters(next);
    setPage(0);
  };

  const handleReset = (): void => {
    setCasesFilters(initial);
    setPage(0);
  };

  const handleRowsPerPageChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  // "Customise columns" — mirrors `CsmIssuesView`'s own wiring exactly (same
  // severity gate, same "exclude Type by default" reasoning) since this
  // preview is, like every `CsmIssuesView` caller, locked to one case type
  // (the widget's own `resourceType`). Sharing a `viewId` with the matching
  // main list page (see `columnsViewIdForResourceType`) means a viewer's
  // column choices carry over between "View more" and the real tab, rather
  // than starting from scratch here.
  const currentUserId = useCurrentUser().user?.id;
  const currentUserEmail = useIdTokenClaims()?.email;
  const showSeverityColumn = resourceType === "case";
  const availableOptionalColumns: CaseOptionalColumnId[] = [
    "product",
    "type",
    "issueType",
    ...(showSeverityColumn ? (["severity"] as const) : []),
    "assignee",
    "createdBy",
    "customer",
    "createdAt",
  ];
  const defaultVisibleOptionalColumns: CaseOptionalColumnId[] = [
    "product",
    ...(showSeverityColumn ? (["severity"] as const) : []),
  ];
  const columnPrefs = useColumnPreferences({
    viewId: `case-list:${columnsViewIdForResourceType(resourceType)}`,
    userKey: getColumnPreferencesUserKey({ id: currentUserId, email: currentUserEmail }),
    columns: availableOptionalColumns.map((id) => ({ id, label: CASE_OPTIONAL_COLUMNS[id].label })),
    defaultVisibleIds: defaultVisibleOptionalColumns,
  });

  const { data, isLoading, isError, isFetching, refetch, dataUpdatedAt } = useGetCsmCases(
    casesFilters,
    page,
    rowsPerPage,
    true,
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {backButton}
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="h5">{displayName}</Typography>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label={`Refresh ${displayName}`}
        />
      </Box>
      <CasesFilterBar
        filters={casesFilters}
        onChange={handleFiltersChange}
        onReset={handleReset}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen((prev) => !prev)}
        availableAssigneeUsers={[]}
        availableProjects={[]}
      />
      {isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load this widget.
        </Typography>
      ) : (
        <>
          <CasesList
            cases={data?.cases ?? []}
            isLoading={isLoading}
            skeletonCount={rowsPerPage}
            hideSeverityColumn={!showSeverityColumn}
            optionalColumns={columnPrefs.visibleColumns.map((c) => c.id as CaseOptionalColumnId)}
            columnCustomizer={
              <ColumnCustomizerButton
                allColumns={columnPrefs.allColumns}
                isVisible={columnPrefs.isVisible}
                onToggle={columnPrefs.toggleColumn}
                onMove={columnPrefs.moveColumn}
                onReorder={columnPrefs.reorderColumn}
                onReset={columnPrefs.resetToDefault}
                label={`Customise ${displayName} columns`}
              />
            }
          />
          <TablePagination
            component="div"
            count={data?.total ?? 0}
            page={page}
            onPageChange={(_, newPage) => setPage(newPage)}
            rowsPerPage={rowsPerPage}
            onRowsPerPageChange={handleRowsPerPageChange}
            rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
            showFirstButton
            showLastButton
          />
        </>
      )}
    </Box>
  );
}

interface CallRequestWidgetPreviewProps {
  widgetId: string;
  displayName: string;
  filters: Record<string, unknown>;
  backButton: JSX.Element;
}

/**
 * The call-requests "View more" landing: a real, editable filter bar seeded
 * from the widget's own filters (`translateCallRequestDashboardFilters` —
 * see `widgetResourceConfig.ts`), feeding the same `useWidgetData` +
 * `CallRequestWidgetList` renderer the widget tile itself already uses —
 * not the generic, read-only "Filtered by:" chip summary + search box
 * every other flat-filter resourceType falls back to (`call_request` isn't
 * in `CASE_FAMILY_RESOURCE_TYPES`: it queries `/call-requests/search`, a
 * different, much smaller field set than the case-search DSL).
 *
 * Deliberately Simple-only — no Simple/Advanced toggle like
 * `CaseFamilyWidgetPreview`'s. Confirmed directly against
 * `apps/csm-portal/backend/openapi.yaml`'s `SearchAllCallRequestsPayload`:
 * this endpoint's filters are `assignedUserIds` (plain UUID list, no op
 * choice), `states` (plain enum list, no op choice), `assignmentTeamIds`
 * (plain UUID list, no op choice), and `caseStates`/`excludeCaseStates` —
 * the ONE field here with an in/notIn distinction at all, and that's
 * already fully expressible via the Case state control's own tri-state
 * (include/exclude) cycle, the same `TriStateMultiSelectField` pattern
 * `CasesFilterBar`'s State control uses for the identical shape. With zero
 * fields left over that a toggle would add anything for, an Advanced mode
 * here would just be an empty second tab — busywork, not a feature, for a
 * 4-field contract this much smaller than the case-search DSL
 * `AdvancedFiltersBuilder` exists for.
 */
function CallRequestWidgetPreview({
  widgetId,
  displayName,
  filters,
  backButton,
}: CallRequestWidgetPreviewProps): JSX.Element {
  // Frozen once at mount, same rationale as `CaseFamilyWidgetPreview`'s own
  // `initial` — a Reset must restore what the widget actually linked here
  // with, not whatever the viewer has since edited.
  const [initial] = useState<CallRequestWidgetFilters>(() => ({
    ...DEFAULT_CALL_REQUEST_WIDGET_FILTERS,
    ...translateCallRequestDashboardFilters(filters),
  }));
  const [crFilters, setCrFilters] = useState<CallRequestWidgetFilters>(initial);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const handleReset = (): void => {
    setCrFilters(initial);
    setPage(0);
  };

  // So the Assignee control can render a selected value equal to the
  // signed-in user's own id as "Me" instead of its raw UUID — see
  // `AsyncUserIdMultiSelect`'s `currentUserId` doc comment. `filters` (this
  // component's own prop) already had any `__current_user__`/`@me`
  // sentinel resolved to this same id upstream, before this component ever
  // saw it (`resolveCurrentUserSentinels`, above) — this is only needed to
  // recognize that resolved id again for display/re-selection.
  const currentUserId = useCurrentUser().user?.id;

  // Same CRE-team sourcing/scoping `CasesFilterBar`'s own "CRE Team" control
  // uses (`useTeams(true)`, filtered to `family === "cre-abt"`, keyed by
  // `creGroupId` — what `assignmentTeamIds` actually matches on, not the
  // registry `id`). This endpoint's `assignmentTeamIds` is CRE-only per
  // digiops-cs#2732 ("Calls To Attend") — no SRE-team equivalent exists on
  // this contract, so there is no second team control to add here.
  const { data: teams } = useTeams(true);
  const creTeamOptions = useMemo(
    () =>
      (teams ?? [])
        .filter(
          (t): t is typeof t & { creGroupId: string } =>
            Boolean(t.creGroupId) && t.family === "cre-abt",
        )
        .map((t) => ({ value: t.creGroupId, label: t.name })),
    [teams],
  );

  const callStateOptions = useMemo(
    () =>
      ALL_CALL_REQUEST_STATES.map((s) => ({ value: s, label: CALL_REQUEST_STATE_LABEL[s] })),
    [],
  );

  const queriedFilters = useMemo(
    () => callRequestWidgetFiltersToQuery(crFilters),
    [crFilters],
  );

  const { data, isLoading, isError, isFetching, refetch, dataUpdatedAt } = useWidgetData({
    widgetId,
    resourceType: "call_request",
    filters: queriedFilters,
    shape: "list",
    listLimit: rowsPerPage,
    offset: page * rowsPerPage,
  });
  const ListRenderer = WIDGET_LIST_RENDERERS.call_request;

  const handleRowsPerPageChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {backButton}
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="h5">{displayName}</Typography>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label={`Refresh ${displayName}`}
        />
      </Box>
      <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "flex-end", gap: 2 }}>
        <Box sx={{ minWidth: 220 }}>
          <MultiSelectField
            id="call-request-filter-state"
            label="Call state"
            values={crFilters.states}
            options={callStateOptions}
            onChange={(next) => {
              setCrFilters({ ...crFilters, states: next });
              setPage(0);
            }}
          />
        </Box>
        <Box sx={{ minWidth: 220 }}>
          <TriStateMultiSelectField
            id="call-request-filter-case-state"
            label="Case state"
            includedValues={crFilters.caseStates}
            excludedValues={crFilters.excludeCaseStates}
            options={STATE_OPTIONS}
            onChange={(next) => {
              setCrFilters({
                ...crFilters,
                caseStates: next.included,
                excludeCaseStates: next.excluded,
              });
              setPage(0);
            }}
          />
        </Box>
        <Box sx={{ minWidth: 220 }}>
          <AsyncUserIdMultiSelect
            id="call-request-filter-assignee"
            label="Assignee"
            values={crFilters.assignedUserIds}
            onChange={(next) => {
              setCrFilters({ ...crFilters, assignedUserIds: next });
              setPage(0);
            }}
            currentUserId={currentUserId}
          />
        </Box>
        <Box sx={{ minWidth: 220 }}>
          <MultiSelectField
            id="call-request-filter-team"
            label="CRE Team"
            values={crFilters.assignmentTeamIds}
            options={creTeamOptions}
            onChange={(next) => {
              setCrFilters({ ...crFilters, assignmentTeamIds: next });
              setPage(0);
            }}
          />
        </Box>
        <Button variant="text" size="small" onClick={handleReset}>
          Reset
        </Button>
      </Box>
      {isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load this widget.
        </Typography>
      ) : (
        <>
          <ListRenderer
            items={data?.items ?? []}
            isLoading={isLoading}
            resourceType="call_request"
          />
          <TablePagination
            component="div"
            count={data?.total ?? 0}
            page={page}
            onPageChange={(_, newPage) => setPage(newPage)}
            rowsPerPage={rowsPerPage}
            onRowsPerPageChange={handleRowsPerPageChange}
            rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
            showFirstButton
            showLastButton
          />
        </>
      )}
    </Box>
  );
}

interface CaseFeedbackWidgetPreviewProps {
  widgetId: string;
  displayName: string;
  filters: Record<string, unknown>;
  backButton: JSX.Element;
}

/** The 5 case-feedback rating values, in order — same scale ServiceNow's own
 * survey uses (see `useCaseFeedbackTrendData`'s `colorForAvgRating` doc
 * comment for the same 1-5 -> CSAT-label mapping). No shared constant for
 * this exists elsewhere in the app (every other rating display reads the
 * label straight off the record itself), so it's scoped here rather than
 * invented as a new cross-feature export for a single dropdown. */
const FEEDBACK_RATING_OPTIONS: { value: string; label: string }[] = [
  { value: "1", label: "1 — Very Dissatisfied" },
  { value: "2", label: "2 — Dissatisfied" },
  { value: "3", label: "3 — Neutral" },
  { value: "4", label: "4 — Satisfied" },
  { value: "5", label: "5 — Very Satisfied" },
];

/** First string value out of a filter field that's either a bare string (a
 * tile/slice click-through's own scalar filters) or a 1-element string[]
 * (the same field once round-tripped through the preview URL — see
 * `parseWidgetPreviewFilters`, which decodes every param as a comma-split
 * array). Either shape lands here since `filters` (this component's own
 * prop) always came from that URL round trip. */
function asFeedbackFilterValue(v: unknown): string | undefined {
  if (typeof v === "string") return v;
  if (Array.isArray(v) && typeof v[0] === "string") return v[0];
  return undefined;
}

/**
 * The case-feedback "View more" landing: unlike the generic
 * `DashboardWidgetPreviewContent` fallback below (a static "Filtered by:"
 * chip summary plus a free-text search box the feedback search endpoint
 * doesn't even support — `case_feedback` has no `searchQuery` field), this
 * gives a real, editable rating + date-range filter bar, seeded from the
 * widget's own filters (a rating-pie or trend-bar slice click-through) and
 * then freely adjustable from there — the same "seeded then editable"
 * pattern `CaseFamilyWidgetPreview` already uses for the case-family
 * resourceTypes, scoped to this resourceType's own flat
 * `dateFrom`/`dateTo`/`rating` filter shape (see
 * `WIDGET_RESOURCE_CONFIG.case_feedback`'s own `buildSearchRequestBody`
 * doc comment) rather than the case-search DSL that component translates.
 */
function CaseFeedbackWidgetPreview({
  widgetId,
  displayName,
  filters,
  backButton,
}: CaseFeedbackWidgetPreviewProps): JSX.Element {
  // Frozen once at mount (a fresh "View more"/slice click is a fresh mount),
  // same rationale as `CaseFamilyWidgetPreview`'s `resetBaseline` — Reset
  // must restore what the widget/slice actually linked here with, not
  // whatever the viewer has since edited.
  const [initial] = useState(() => ({
    dateFrom: asFeedbackFilterValue(filters.dateFrom),
    dateTo: asFeedbackFilterValue(filters.dateTo),
    rating: asFeedbackFilterValue(filters.rating),
  }));
  const [dateRange, setDateRange] = useState<DateRangeFilterValue>({
    from: initial.dateFrom,
    to: initial.dateTo,
  });
  const [rating, setRating] = useState<string>(initial.rating ?? "");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const queriedFilters = useMemo(() => {
    const out: Record<string, unknown> = { ...filters };
    delete out.dateFrom;
    delete out.dateTo;
    delete out.rating;
    if (dateRange.from) out.dateFrom = dateRange.from;
    if (dateRange.to) out.dateTo = dateRange.to;
    if (rating) out.rating = Number(rating);
    return out;
  }, [filters, dateRange, rating]);

  const handleReset = (): void => {
    setDateRange({ from: initial.dateFrom, to: initial.dateTo });
    setRating(initial.rating ?? "");
    setPage(0);
  };

  const { data, isLoading, isError, isFetching, refetch, dataUpdatedAt } = useWidgetData({
    widgetId,
    resourceType: "case_feedback",
    filters: queriedFilters,
    shape: "list",
    listLimit: rowsPerPage,
    offset: page * rowsPerPage,
  });
  const ListRenderer = WIDGET_LIST_RENDERERS.case_feedback;

  const handleRowsPerPageChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {backButton}
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="h5">{displayName}</Typography>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label={`Refresh ${displayName}`}
        />
      </Box>
      <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "flex-end", gap: 2 }}>
        <DateRangeFilter
          label="Feedback submitted"
          value={dateRange}
          onChange={(next) => {
            setDateRange(next);
            setPage(0);
          }}
        />
        <FormControl size="small" sx={{ minWidth: 220 }}>
          <InputLabel id="feedback-rating-filter-label">Rating</InputLabel>
          <Select
            labelId="feedback-rating-filter-label"
            label="Rating"
            value={rating}
            onChange={(e) => {
              setRating(e.target.value);
              setPage(0);
            }}
          >
            <MenuItem value="">All ratings</MenuItem>
            {FEEDBACK_RATING_OPTIONS.map((option) => (
              <MenuItem key={option.value} value={option.value}>
                {option.label}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <Button variant="text" size="small" onClick={handleReset}>
          Reset
        </Button>
      </Box>
      {isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load this widget.
        </Typography>
      ) : (
        <>
          <ListRenderer
            items={data?.items ?? []}
            isLoading={isLoading}
            resourceType="case_feedback"
          />
          <TablePagination
            component="div"
            count={data?.total ?? 0}
            page={page}
            onPageChange={(_, newPage) => setPage(newPage)}
            rowsPerPage={rowsPerPage}
            onRowsPerPageChange={handleRowsPerPageChange}
            rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
            showFirstButton
            showLastButton
          />
        </>
      )}
    </Box>
  );
}

interface DashboardWidgetPreviewContentProps {
  widgetId: string;
  displayName: string;
  resourceType: BeWidgetResourceType;
  filters: Record<string, unknown>;
  backButton: JSX.Element;
}

function DashboardWidgetPreviewContent({
  widgetId,
  displayName,
  resourceType,
  filters,
  backButton,
}: DashboardWidgetPreviewContentProps): JSX.Element {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, SEARCH_DEBOUNCE_MS);

  const queriedFilters = useMemo(() => {
    const trimmed = debouncedSearch.trim();
    return trimmed ? { ...filters, searchQuery: trimmed } : filters;
  }, [filters, debouncedSearch]);

  // What's actually being queried, made visible rather than trusted
  // silently — the exact filters this page is about to send, in the same
  // already-resolved shape `useWidgetData` below queries with (no
  // `__current_team__`/`@me` placeholders left to decode). Excludes the
  // free-text search term, which the search box right below already shows.
  const filterSummary = useMemo(() => describeWidgetFilters(filters), [filters]);

  const { data, isLoading, isError, isFetching, refetch, dataUpdatedAt } = useWidgetData({
    widgetId,
    resourceType,
    filters: queriedFilters,
    shape: "list",
    listLimit: rowsPerPage,
    offset: page * rowsPerPage,
  });
  const ListRenderer = WIDGET_LIST_RENDERERS[resourceType];

  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setSearch(e.target.value);
    setPage(0);
  };

  const handleRowsPerPageChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {backButton}
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="h5">{displayName}</Typography>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label={`Refresh ${displayName}`}
        />
      </Box>
      {filterSummary.length > 0 && (
        <Box
          role="group"
          aria-label="Active filters"
          sx={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 1 }}
        >
          <Typography variant="caption" color="text.secondary">
            Filtered by:
          </Typography>
          {filterSummary.map((entry) => (
            <Chip
              key={`${entry.field}-${entry.op ?? "in"}`}
              size="small"
              variant="outlined"
              label={`${entry.field}${entry.op ? ` (${entry.op})` : ""}: ${entry.value}`}
            />
          ))}
        </Box>
      )}
      <TextField
        size="small"
        label="Search"
        placeholder="Search…"
        value={search}
        onChange={handleSearchChange}
        slotProps={{ htmlInput: { "aria-label": "Search" } }}
        sx={{ maxWidth: 360 }}
      />
      {isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load this widget.
        </Typography>
      ) : (
        <>
          <ListRenderer
            items={data?.items ?? []}
            isLoading={isLoading}
            resourceType={resourceType}
          />
          <TablePagination
            component="div"
            count={data?.total ?? 0}
            page={page}
            onPageChange={(_, newPage) => setPage(newPage)}
            rowsPerPage={rowsPerPage}
            onRowsPerPageChange={handleRowsPerPageChange}
            rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
            showFirstButton
            showLastButton
          />
        </>
      )}
    </Box>
  );
}
