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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  Grid,
  IconButton,
  InputAdornment,
  ListItemIcon,
  ListItemText,
  ListSubheader,
  Menu,
  MenuItem,
  Paper,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  Bookmark,
  BookmarkPlus,
  Check,
  ChevronDown,
  ChevronUp,
  ListFilter,
  Search,
  Trash2,
  X,
} from "@wso2/oxygen-ui-icons-react";
import { useCallback, useMemo, useState, type JSX } from "react";
import type {
  CaseState,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";
import {
  countActiveFilters,
  DEFAULT_CASES_FILTERS,
  readCasesFiltersFromUrl,
  writeCasesFiltersToUrl,
} from "@features/csm-cases/utils/casesFiltersUrl";
import { useTeams } from "@features/csm-dashboard/api/useTeams";
import {
  deleteFilterView,
  moveFilterView,
  saveFilterView,
  useSavedFilterViews,
} from "@features/csm-cases/utils/savedFilterViews";
import type {
  BeCaseType,
  BeCaseWorkState,
  BeEngagementType,
} from "@api/backend/types";
import {
  ALL_CASE_TYPES,
  CASE_TYPE_LABEL,
} from "@features/csm-cases/utils/caseType";
import AsyncProjectMultiSelect from "@features/csm-cases/components/AsyncProjectMultiSelect";
import MultiSelectField from "@components/MultiSelectField";
import AsyncAssigneeMultiSelect from "@features/csm-cases/components/AsyncAssigneeMultiSelect";
import ProductNameMultiSelect from "@features/csm-cases/components/ProductNameMultiSelect";
import AdvancedFiltersBuilder from "@features/csm-cases/components/AdvancedFiltersBuilder";
import AnyOfGroupsBuilder from "@features/csm-cases/components/AnyOfGroupsBuilder";
import {
  getAdvancedFilterFieldMeta,
  getAdvancedFilterOpMeta,
  isCompleteAdvancedFilterRow,
  type AdvancedFilterRow,
} from "@features/csm-cases/utils/advancedFilters";
import { isCompleteAnyOfBranch, type AnyOfBranch } from "@features/csm-cases/utils/anyOfFilters";
import {
  ENGAGEMENT_TYPE_OPTIONS,
  ONBOARDING_STATUS_OPTIONS,
  SEVERITY_OPTIONS,
  STATE_OPTIONS,
} from "@features/csm-cases/utils/caseFilterOptions";
import {
  addBlankUnifiedRow,
  filtersToAdvancedRows,
  isSimpleRepresentable,
  removeUnifiedRow,
  updateUnifiedRow,
  type UnifiedFilterRow,
} from "@features/csm-cases/utils/filterFieldAdapters";


/**
 * Filter state for the CSM cases list. `severities` / `states` / `caseTypes`
 * are multi-select arrays driven by fixed enums; `projects` is an id-based
 * type-to-search multi-select. `assignees` holds engineer **emails** plus the
 * sentinel `@me`; `useGetCsmCases` resolves these to the engineer UUIDs that
 * `/cases/search` filters on. All are pushed into the `/cases/search` payload
 * server-side.
 */
export interface CasesFilters {
  search: string;
  severities: Severity[];
  states: CaseState[];
  /** States the case must NOT be in (`state` op:notIn). Not the inverse of
   * `states` — a distinct field so `in` and `notIn` can never be conflated
   * on the round trip, same reasoning as `tags`/`excludeTags`. No dedicated
   * Simple-mode bar control of its own (the State field's tri-state
   * include/exclude toggle was removed — Simple mode's "State" control is
   * now a plain include-only multi-select, matching every other Simple
   * field); still settable via the Advanced-mode `state`/`notIn` row or a
   * dashboard click-through, and surfaced as a removable chip when active
   * (see `buildActiveFilterChips`). */
  excludeStates: CaseState[];
  /** Case-type filter (BE `typeKeys`). Empty = all types. */
  caseTypes: BeCaseType[];
  /** Engineer emails (+ the `@me` sentinel) to filter by assigned engineer. */
  assignees: string[];
  /** Work sub-state filter; only meaningful when `states` includes `work_in_progress`. */
  workStates: BeCaseWorkState[];
  projects: string[];
  /** Engagement sub-type filter; only meaningful when `caseTypes` is locked to `engagement`. */
  engagementTypes: BeEngagementType[];
  /** Product family names (e.g. "API Manager"); matches all versions of each. */
  productNames: string[];
  /** CS team group ids (`creTeam` op:in) the case's project is scoped to. */
  csTeams: string[];
  /** SRE team group ids (`sreTeam` op:in) the case's project is scoped to.
   * Independent of `csTeams` -- a case's account may carry both a CRE and
   * an SRE team assignment. */
  sreTeams: string[];
  /** Tags the case must carry (`tag` op:in). Independent of `excludeTags` —
   * both may be set at once (the backend ANDs them). */
  tags: string[];
  /** Tags the case must NOT carry (`tag` op:notIn). Not the inverse of
   * `tags` — a distinct field so `in` and `notIn` can never be conflated on
   * the round trip (see `casesFiltersUrl.ts`'s codec doc comment). */
  excludeTags: string[];
  /** Project onboarding status values (`projectOnboardingStatus` op:in).
   * Unlike `states`/`tags`, this field has no `notIn` counterpart of its
   * own: the domain is exactly the 4 fixed values in `ALL_ONBOARDING_STATUSES`
   * (`onboardingStatus.ts`), so "not X" and "in every value except X" are
   * equivalent, and `translateCaseDashboardFilters` (`widgetResourceConfig.ts`)
   * folds a dashboard widget's `notIn` filter into this field's complement at
   * the translation boundary rather than carrying a separate exclude field
   * (and URL param) through the rest of the app. */
  onboardingStatuses: string[];
  /** Inclusive lower bound on the case's active task's SLA business-elapsed
   * percent (`taskSLABusinessElapsedPercent` op:gte). `null` = unset. */
  slaElapsedPctGte: number | null;
  /** Inclusive upper bound, same field, op:lte. `null` = unset. */
  slaElapsedPctLte: number | null;
  /** Escalation presence (`escalation` field): `true` = has an active
   * escalation (op:isNotEmpty), `false` = has none (op:isEmpty), `null` =
   * unfiltered. Deliberately not string-typed on the ops themselves — the
   * value-less op IS the whole predicate here, so a tri-state is the
   * accurate shape rather than an op name a caller could typo. */
  hasEscalation: boolean | null;
  /** Escalation level values (`escalationLevel` op:in). */
  escalationLevels: string[];
  /** Project-type ids (`projectType` op:in). */
  projectTypes: string[];
  /** `createdOn` range bounds (op:gte / op:lte respectively); RFC3339 or
   * `YYYY-MM-DD`. `null` = unbounded on that side. */
  createdOnGte: string | null;
  createdOnLte: string | null;
  /** `updatedOn` range bounds — same shape as `createdOnGte`/`createdOnLte`. */
  updatedOnGte: string | null;
  updatedOnLte: string | null;
  /** `closedOn` range bounds — same shape as `createdOnGte`/`createdOnLte`. */
  closedOnGte: string | null;
  closedOnLte: string | null;
  /**
   * Ad-hoc field/op/value rows from the "Advanced filters" builder — the
   * escape hatch for `/cases/search` fields the dedicated bar controls above
   * don't cover (`projectType`, `sreTeam`, `deploymentId`, `number`,
   * `internalId`, `resolutionNotes`, `parentId`,
   * `taskSLABusinessElapsedPercent`, `escalationLevel`, `escalation`,
   * `createdBy`, and the `createdOn`/`updatedOn`/`closedOn` date ranges) —
   * see `advancedFilters.ts`'s field catalogue. Fields that already have a
   * dedicated bar control of their own (`tag`, `projectOnboardingStatus`,
   * `creTeam`) are deliberately NOT offered here — see that catalogue's own
   * doc comment. Each row maps to one extra `BeCaseFieldFilter` entry
   * (`caseSearchPayload.ts`); an incomplete row (a field/op picked but no
   * value where one is required) is never emitted — see
   * `isCompleteAdvancedFilterRow`.
   */
  advancedFilters: AdvancedFilterRow[];
  /**
   * Cross-field OR groups from the "OR groups" builder — each branch's own
   * rows are ANDed, the branches themselves are OR'd, and the whole `anyOf`
   * result is ANDed with everything else (`filters.anyOf`, see
   * `anyOfFilters.ts`). Only a restricted field subset may appear inside a
   * branch — a real, backend-enforced allowlist distinct from (and narrower
   * than) `advancedFilters`' own field catalogue. A branch with no complete
   * conditions is never emitted into the request payload — see
   * `isCompleteAnyOfBranch`.
   */
  anyOfBranches: AnyOfBranch[];
}

/**
 * Lightweight user-directory entry surfaced in the assignee picker. The filter
 * stores the `email` as the value; the `name` is shown as the option label.
 */
export interface AssigneeUser {
  name: string;
  email: string;
}

interface CasesFilterBarProps {
  filters: CasesFilters;
  onChange: (next: CasesFilters) => void;
  onReset: () => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
  /** Full user directory shown in the assignee picker. */
  availableAssigneeUsers: AssigneeUser[];
  /** Projects for the (id-based) project filter — value is the id, label the name. */
  availableProjects: { id: string; name: string }[];
  /**
   * Show the severity control. Severity (S1-S4) is a support-case concept, so
   * this is only meaningful when the list is scoped to support cases; other
   * record types (service requests, engagements, etc.) hide it.
   */
  showSeverityFilter?: boolean;
  /** Hide the case-type control when the surrounding view locks the type. */
  hideTypeFilter?: boolean;
  /**
   * Label for the case-type control. Defaults to "Case type"; a view that
   * mixes every record type under a broader umbrella term (e.g. a project's
   * Work items tab, which spans cases/service requests/security reports/
   * engagements/announcements) can override it to "Work item type" so the
   * label matches what the surrounding page calls these records, without
   * changing the control's behavior or its `caseTypes` value shape.
   */
  typeFilterLabel?: string;
  /** Hide the project control when the surrounding view is project-scoped. */
  hideProjectFilter?: boolean;
  /** Show the engagement-type multi-select (only relevant when type is locked to engagement). */
  showEngagementTypeFilter?: boolean;
  /**
   * Hide the "Onboarding status" Simple-mode control. `onboardingStatuses` is
   * a per-project attribute, not a per-case one — on a view already scoped to
   * a single project (e.g. that project's own Work items tab), every case
   * shown shares the same value, so the control is a no-op that only adds
   * clutter. The field itself stays in the Advanced-mode catalogue; this only
   * hides the dedicated Simple control.
   */
  hideOnboardingStatusFilter?: boolean;
  /**
   * Hide the "CRE Team" Simple-mode control. Same reasoning as
   * {@link hideOnboardingStatusFilter}: the CS team a case's project is
   * scoped to is a per-project attribute, a no-op filter on a
   * single-project-scoped view. Advanced mode still offers the field.
   */
  hideCreTeamFilter?: boolean;
}

// Work state has no bar control of its own (see `buildActiveFilterChips`'s
// doc comment) -- this only labels its chip now.
const WORK_STATE_LABEL: Record<BeCaseWorkState, string> = {
  ongoing: "Ongoing",
  paused: "Paused",
};

/** Formats a `createdOn`/`updatedOn`/`closedOn` bound for a chip label — a
 * locale date when parseable, the raw string otherwise (never throws on a
 * malformed value; this is a display fallback, not validation). */
function formatDateBound(raw: string): string {
  // A bare `YYYY-MM-DD` is parsed as UTC midnight by `Date`, which renders as
  // the previous day for any locale behind UTC — pin it to local midnight.
  const d = /^\d{4}-\d{2}-\d{2}$/.test(raw)
    ? new Date(`${raw}T00:00:00`)
    : new Date(raw);
  return Number.isNaN(d.getTime()) ? raw : d.toLocaleDateString();
}

/** One removable "active filter" chip. */
interface ActiveFilterChip {
  key: string;
  label: string;
  onRemove: (filters: CasesFilters) => CasesFilters;
}

/**
 * Fields extended onto `CasesFilters` for lossless dashboard click-through
 * (SLA %, escalation, project type, the three date ranges) get no dedicated
 * bar control of their own — that many new fields would overwhelm the bar,
 * and most of them only ever get set by a widget click-through, not
 * hand-picked in the bar. They must still be visible and individually
 * removable, though, or a user landing on a dashboard-filtered cases list
 * has no way to see (or undo) *why* it's filtered — hence one chip per
 * active value here, shown regardless of whether the filter grid itself is
 * expanded. `sreTeams`/`excludeTags`/`workStates` are included here too:
 * their bar controls were removed as clutter (they are advanced, rarely
 * hand-picked, and a better home for advanced filters is still to be
 * designed), so a chip is now the ONLY way a user can see or clear them
 * after arriving from a dashboard click-through. `excludeStates` joins this
 * group too: the State field's tri-state include/exclude toggle was removed
 * (Simple mode's "State" control is now a plain include-only multi-select),
 * so a chip is the only way to see/clear an exclusion that arrived via a
 * saved view, a shared URL, or a dashboard click-through. `csTeams`/
 * `onboardingStatuses` has its own bar control (see the filter grid below)
 * and is deliberately NOT chipped here — every other bar-controlled field
 * (`states`, `severities`, ...) shows its selection inside its own control,
 * not as a second, redundant chip. `tags`/`excludeTags` moved out of Simple
 * mode entirely (Tags is Advanced-only now — see the mode toggle below): any
 * value in either one forces `isSimpleRepresentable` to false, so the Tag
 * row itself is what a user sees (in Advanced mode), never a chip alongside
 * an invisible control.
 */
// All of the reasoning above is Simple-mode only. In Advanced mode every
// field this function chips (`sreTeams`, `workStates`, the SLA/escalation/
// project-type/date-range fields, plus `advancedFilters` and
// `anyOfBranches`) already has its own always-visible, always-editable,
// always-clearable row: `filtersToAdvancedRows` (`filterFieldAdapters.ts`)
// turns every typed field, and every `advancedFilters` entry, into a row
// `AdvancedFiltersBuilder` renders; `anyOfBranches` gets its own bordered
// box per branch from `AnyOfGroupsBuilder`, unconditionally, whenever it's
// mounted. Advanced mode only ever shows this function's output *alongside*
// those two builders (see the call site below), so a chip there would be a
// redundant, un-synced second rendering of a value the row/branch above it
// already owns -- hence the caller only invokes this function in Simple
// mode now. Simple mode has no row list at all, so a chip stays the only
// way to see/clear an Advanced-only value that arrived via URL/dashboard
// click-through while looking at the Simple grid -- that part is unchanged.
function buildActiveFilterChips(
  filters: CasesFilters,
  /** groupId -> team display name, so a team chip never shows a raw UUID.
   * Falls back to the id when the lookup has not resolved (or the team is
   * unknown) rather than hiding the chip — an unlabelled filter the user can
   * still see and remove beats an invisible one. Only feeds the `sreTeams`
   * chip now (`csTeams` has its own bar control), but still covers both
   * `creGroupId` and `sreGroupId` keys since the caller passes one merged
   * map either way. */
  teamLabels: Record<string, string> = {},
): ActiveFilterChip[] {
  const chips: ActiveFilterChip[] = [];

  filters.sreTeams.forEach((groupId) => {
    chips.push({
      key: `sreTeam-${groupId}`,
      label: `SRE team: ${teamLabels[groupId] ?? groupId}`,
      onRemove: (f) => ({ ...f, sreTeams: f.sreTeams.filter((t) => t !== groupId) }),
    });
  });

  // `states` still has its own bar control (a plain include-only
  // multi-select on the State field -- see the filter grid below), so it is
  // not chipped, same as `csTeams`/`onboardingStatuses`. `excludeStates` no
  // longer has a bar control of its own (the tri-state include/exclude
  // toggle that used to live on the State field was removed), so it gets a
  // chip here -- the only way to see/clear it once it arrives via a saved
  // view, a shared URL, or a dashboard click-through. `tags`/`excludeTags`
  // are Advanced-mode-only now (see the mode toggle below) -- also not
  // chipped, since any non-empty value forces Advanced mode, where the Tag
  // row itself is the visible/removable UI.
  filters.excludeStates.forEach((state) => {
    chips.push({
      key: `excludeState-${state}`,
      label: `Excluding state: ${STATE_OPTIONS.find((o) => o.value === state)?.label ?? state}`,
      onRemove: (f) => ({
        ...f,
        excludeStates: f.excludeStates.filter((s) => s !== state),
      }),
    });
  });

  filters.workStates.forEach((workState) => {
    chips.push({
      key: `workState-${workState}`,
      label: `Work state: ${WORK_STATE_LABEL[workState] ?? workState}`,
      onRemove: (f) => ({
        ...f,
        workStates: f.workStates.filter((w) => w !== workState),
      }),
    });
  });

  // `onboardingStatuses` has its own "Onboarding status" bar control now
  // (see the filter grid below) -- not chipped here, same as
  // `csTeams`/`productNames`/every other bar-controlled field.

  if (filters.slaElapsedPctGte !== null) {
    chips.push({
      key: "sla-gte",
      label: `SLA ≥ ${filters.slaElapsedPctGte}%`,
      onRemove: (f) => ({ ...f, slaElapsedPctGte: null }),
    });
  }
  if (filters.slaElapsedPctLte !== null) {
    chips.push({
      key: "sla-lte",
      label: `SLA ≤ ${filters.slaElapsedPctLte}%`,
      onRemove: (f) => ({ ...f, slaElapsedPctLte: null }),
    });
  }

  if (filters.hasEscalation !== null) {
    chips.push({
      key: "escalation",
      label: filters.hasEscalation ? "Escalated" : "No escalation",
      onRemove: (f) => ({ ...f, hasEscalation: null }),
    });
  }

  filters.escalationLevels.forEach((level) => {
    chips.push({
      key: `escalation-level-${level}`,
      label: `Escalation level: ${level}`,
      onRemove: (f) => ({
        ...f,
        escalationLevels: f.escalationLevels.filter((l) => l !== level),
      }),
    });
  });

  filters.projectTypes.forEach((projectType) => {
    chips.push({
      key: `project-type-${projectType}`,
      // No project-type name lookup exists in the frontend yet (the backend
      // filter is keyed by an opaque id, not a slug) — shows the raw id
      // rather than guessing at a label.
      label: `Project type: ${projectType}`,
      onRemove: (f) => ({
        ...f,
        projectTypes: f.projectTypes.filter((t) => t !== projectType),
      }),
    });
  });

  const dateRanges: [string, keyof CasesFilters, keyof CasesFilters][] = [
    ["Created", "createdOnGte", "createdOnLte"],
    ["Updated", "updatedOnGte", "updatedOnLte"],
    ["Closed", "closedOnGte", "closedOnLte"],
  ];
  for (const [labelPrefix, gteKey, lteKey] of dateRanges) {
    const gte = filters[gteKey] as string | null;
    if (gte !== null) {
      chips.push({
        key: `${gteKey}`,
        label: `${labelPrefix} after ${formatDateBound(gte)}`,
        onRemove: (f) => ({ ...f, [gteKey]: null }),
      });
    }
    const lte = filters[lteKey] as string | null;
    if (lte !== null) {
      chips.push({
        key: `${lteKey}`,
        label: `${labelPrefix} before ${formatDateBound(lte)}`,
        onRemove: (f) => ({ ...f, [lteKey]: null }),
      });
    }
  }

  // Advanced-filter rows (see `AdvancedFiltersBuilder`) each get their own
  // chip too — same reasoning as the SLA/escalation/date-range group above:
  // they're an ad-hoc escape hatch with no bar control of their own, so a
  // chip is the only way to see or clear one once the filter grid is
  // collapsed (e.g. after a saved view or a shared URL sets one). Only
  // *complete* rows are chipped; an in-progress row (no value yet) is only
  // ever visible inside the open builder itself.
  filters.advancedFilters.forEach((row, index) => {
    if (!isCompleteAdvancedFilterRow(row)) return;
    const fieldMeta = getAdvancedFilterFieldMeta(row.field);
    const opMeta = getAdvancedFilterOpMeta(row.field, row.op);
    const valueText = row.values.length > 0 ? ` ${row.values.join(", ")}` : "";
    chips.push({
      key: `advanced-${row.field}-${row.op}-${index}`,
      label: `${fieldMeta?.label ?? row.field} ${opMeta?.label ?? row.op}${valueText}`,
      onRemove: (f) => ({
        ...f,
        advancedFilters: f.advancedFilters.filter((_, i) => i !== index),
      }),
    });
  });

  // Each OR-branch is a distinct predicate too — chipped by its position
  // (not its contents, which can be several conditions long) so it stays
  // visible/removable even once the "OR groups" builder is collapsed. Only
  // a branch with at least one complete condition is chipped; a branch
  // that's still being edited (no complete rows yet) is only ever visible
  // inside the open builder itself, same as an in-progress advanced-filter
  // row.
  filters.anyOfBranches.forEach((branch, index) => {
    if (!isCompleteAnyOfBranch(branch)) return;
    chips.push({
      key: `any-of-branch-${index}`,
      label: `OR group ${index + 1}`,
      onRemove: (f) => ({
        ...f,
        anyOfBranches: f.anyOfBranches.filter((_, i) => i !== index),
      }),
    });
  });

  return chips;
}

export default function CasesFilterBar({
  filters,
  onChange,
  onReset,
  isFiltersOpen,
  onFiltersToggle,
  availableAssigneeUsers,
  availableProjects,
  showSeverityFilter = true,
  hideTypeFilter = false,
  hideOnboardingStatusFilter = false,
  hideCreTeamFilter = false,
  typeFilterLabel = "Case type",
  hideProjectFilter = false,
  showEngagementTypeFilter = false,
}: CasesFilterBarProps): JSX.Element {
  const activeCount = countActiveFilters(filters);
  const hasActive = activeCount > 0;

  // Simple/Advanced mode. Lazily initialized from the *filters this
  // component mounted with* — mount-time-only, deliberately never
  // recomputed on every `filters` change (that would silently flip the user
  // out of the mode they're actively editing in, e.g. the instant they add
  // an Advanced-only field while in Advanced mode, or clear the last
  // Advanced-only field while reviewing a URL in Simple-representable
  // territory). `CasesFilterBar` remounts on real navigation (a fresh
  // `/cases?...` load, a saved view's own page), which is the only time this
  // should re-derive.
  const [mode, setMode] = useState<"simple" | "advanced">(() =>
    isSimpleRepresentable(filters) ? "simple" : "advanced",
  );
  const canShowSimple = isSimpleRepresentable(filters);
  // A filter Simple mode cannot render must never sit silently active behind
  // the Simple grid with nothing on screen showing it — e.g. applying a
  // saved view that carries `tags` while already in Simple mode: `onChange`
  // updates `filters` without remounting this component, so the mount-time
  // `mode` state above never re-derives on its own. This only ever pushes
  // Simple -> Advanced (never the reverse), so it doesn't touch the
  // mount-time-only guarantee `mode`'s own comment describes for the
  // Advanced-authoring case.
  const effectiveMode = mode === "simple" && !canShowSimple ? "advanced" : mode;

  // Holds the Advanced-only filter state a "Quick filters" click had to move
  // out of the way (see handleSwitchToSimple below) so it can be restored if
  // the user comes right back to Advanced without having changed anything in
  // Quick filters meanwhile — protects against a stray/accidental click
  // losing real filter criteria, without reintroducing the "Simple mode
  // silently still filtering by something it can't display" bug the
  // disabled-button guard above was originally written to prevent.
  const [stashedAdvancedFilters, setStashedAdvancedFilters] =
    useState<CasesFilters | null>(null);

  // Every Quick-filters-grid field routes its edits through this instead of
  // `onChange` directly, so making a real edit while a stash is pending
  // drops it — restoring stale Advanced criteria over a filter the user just
  // deliberately changed would be its own kind of surprising data loss.
  const handleSimpleFieldChange = useCallback(
    (next: CasesFilters) => {
      if (stashedAdvancedFilters) setStashedAdvancedFilters(null);
      onChange(next);
    },
    [onChange, stashedAdvancedFilters],
  );

  const handleSwitchToSimple = (): void => {
    if (!canShowSimple) {
      // Preserve search text (not part of what Simple/Advanced toggles)
      // while resetting every actual filter field to empty — Quick filters
      // must never silently keep an Advanced-only criterion applied behind
      // a grid that can't show it.
      setStashedAdvancedFilters(filters);
      onChange({ ...DEFAULT_CASES_FILTERS, search: filters.search });
    }
    setMode("simple");
  };

  const handleSwitchToAdvanced = (): void => {
    if (stashedAdvancedFilters) {
      // Preserve whatever search text is current, not the stash's own
      // (possibly now-stale) copy — search stays live across both modes, so
      // it can have changed while the stash sat unused.
      onChange({ ...stashedAdvancedFilters, search: filters.search });
      setStashedAdvancedFilters(null);
    }
    setMode("advanced");
  };

  const unifiedRows: UnifiedFilterRow[] = useMemo(
    () => filtersToAdvancedRows(filters),
    [filters],
  );

  // Team is a fixed, small enough list to fetch in full (same endpoint/hook
  // the team-based dashboards use -- see AbtDashboardHeader) rather than a
  // type-to-search async picker, and doubles as the source for the "CS
  // team" bar control below and the SRE-team chip label (SRE team has no
  // bar control of its own -- see `buildActiveFilterChips`).
  const { data: teams } = useTeams(true);
  const teamLabels = useMemo(() => {
    const labels: Record<string, string> = {};
    for (const t of teams ?? []) {
      if (t.creGroupId) labels[t.creGroupId] = t.name;
      if (t.sreGroupId) labels[t.sreGroupId] = t.name;
    }
    return labels;
  }, [teams]);
  // `creGroupId` (not the registry `id`) is what a `creTeam`/`csTeams` filter
  // entry actually matches on; only teams with one configured are
  // selectable here (an id-less team has nothing such a filter could hold).
  // Also scoped to the `cre-abt` family, matching `abtFamilyForDashboardType`
  // -- a team of a different family (e.g. plain `cre`) may still carry a
  // `creGroupId` but isn't one of the CRE Team filter's intended options.
  const teamOptions = useMemo(
    () =>
      (teams ?? [])
        .filter(
          (t): t is typeof t & { creGroupId: string } =>
            Boolean(t.creGroupId) && t.family === "cre-abt",
        )
        .map((t) => ({ value: t.creGroupId, label: t.name })),
    [teams],
  );
  // Same shape, keyed off `sreGroupId` instead — feeds the "Advanced
  // filters" builder's `sreTeam` row (a real multi-select now, not
  // hand-typed team ids/UUIDs). Scoped to `cre-abt` family per explicit
  // product instruction, not `sre-abt` -- see the "SRE Team" filter's
  // family-scoping note in `advancedFilters.ts` for the caveat.
  const sreTeamOptions = useMemo(
    () =>
      (teams ?? [])
        .filter(
          (t): t is typeof t & { sreGroupId: string } =>
            Boolean(t.sreGroupId) && t.family === "cre-abt",
        )
        .map((t) => ({ value: t.sreGroupId, label: t.name })),
    [teams],
  );

  // Suppressed only when Advanced mode's own row list/OR-groups are ALSO on
  // screen (`effectiveMode === "advanced" && isFiltersOpen`) -- that's the
  // only situation where a chip would duplicate something already visible
  // and editable (via `AdvancedFiltersBuilder`/`AnyOfGroupsBuilder`). While
  // the panel is collapsed, neither the Simple grid nor the Advanced builder
  // renders, so chips are the ONLY way to see what's active regardless of
  // mode -- this list itself renders outside the `isFiltersOpen` gate below,
  // by design (see that render site's own doc comment), so this memo must
  // not silently empty itself out just because the mode happens to be
  // Advanced. Uses `effectiveMode`, not `mode`, so a non-simple-representable
  // value that arrives without a remount (e.g. an applied saved view)
  // doesn't sit chip-less behind a Simple grid that no longer matches
  // reality.
  const activeFilterChips = useMemo(
    () =>
      effectiveMode === "simple" || !isFiltersOpen
        ? buildActiveFilterChips(filters, teamLabels)
        : [],
    [filters, teamLabels, effectiveMode, isFiltersOpen],
  );

  // ── Saved views ──────────────────────────────────────────────────────────
  // A saved view is just a name pointing at a serialized filter query string;
  // applying one feeds the parsed filters back through onChange (which the page
  // writes to the URL), so the URL stays the source of truth.
  const savedViews = useSavedFilterViews();
  const currentQs = writeCasesFiltersToUrl(filters).toString();
  // Canonicalize a query string (normalize comma encoding, param order, and
  // drop unknown params) so the "active view" check matches regardless of how a
  // view's qs was authored — suggested presets use literal commas, while
  // writeCasesFiltersToUrl emits %2C.
  const canonicalQs = (qs: string): string =>
    writeCasesFiltersToUrl(
      readCasesFiltersFromUrl(new URLSearchParams(qs)),
    ).toString();
  const currentCanonical = canonicalQs(currentQs);
  const isActiveView = (qs: string): boolean => canonicalQs(qs) === currentCanonical;
  const [savedAnchor, setSavedAnchor] = useState<HTMLElement | null>(null);
  const [saveDialogOpen, setSaveDialogOpen] = useState(false);
  const [newViewName, setNewViewName] = useState("");

  const applyView = (qs: string): void => {
    setSavedAnchor(null);
    // A pending stash is about to be replaced wholesale by this saved view —
    // if left in place, clicking Advanced afterward would resurrect the
    // pre-Quick-filters criteria instead of the view the user just applied.
    setStashedAdvancedFilters(null);
    onChange(readCasesFiltersFromUrl(new URLSearchParams(qs)));
  };

  // Same reasoning as applyView above: a reset must not be silently undoable
  // via a later "Advanced" click restoring what was just cleared.
  const handleReset = (): void => {
    setStashedAdvancedFilters(null);
    onReset();
  };

  const handleSaveView = (): void => {
    if (!newViewName.trim()) return;
    saveFilterView(newViewName, currentQs);
    setNewViewName("");
    setSaveDialogOpen(false);
    setSavedAnchor(null);
  };

  // Fixed enums — shared with `advancedFilters.ts`'s catalogue
  // (`caseFilterOptions.ts` is the one source of truth for each), so a value
  // picked in either mode renders identically in the other. Plain constants,
  // not `useMemo`'d, since they're static imports, not derived from props.
  const severityOptions = SEVERITY_OPTIONS;
  const stateOptions = STATE_OPTIONS;
  const caseTypeOptions = useMemo(
    () => ALL_CASE_TYPES.map((t) => ({ value: t, label: CASE_TYPE_LABEL[t] })),
    [],
  );
  const engagementTypeOptions = ENGAGEMENT_TYPE_OPTIONS;

  // Project filter loads the first page of projects on open and pages through
  // the rest on scroll (and narrows as you type) rather than loading the whole
  // catalogue at once. `availableProjects` (projects on the loaded cases) only
  // seeds chip labels for already-selected ids before any page loads.
  const projectNameSeed = useMemo(
    () => new Map(availableProjects.map((p) => [p.id, p.name])),
    [availableProjects],
  );

  // The assignee filter searches the user directory from the backend as you
  // type (see AsyncAssigneeMultiSelect), so anyone is findable — not just the
  // first page of users. `availableAssigneeUsers` (the directory prefetch /
  // owners on loaded cases) only seeds chip labels for already-selected emails
  // before any search has run.
  const assigneeNameSeed = useMemo(() => {
    const m = new Map<string, string>();
    availableAssigneeUsers.forEach((u) => {
      if (u.email) m.set(u.email, u.name);
    });
    return m;
  }, [availableAssigneeUsers]);

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      {/* Search + saved views + filters toggle. */}
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Box sx={{ position: "relative", flex: 1, minWidth: 240 }}>
          <TextField
            fullWidth
            size="small"
            placeholder="Search by case #, subject or internal ID…"
            value={filters.search}
            onChange={(e) => onChange({ ...filters, search: e.target.value })}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
                endAdornment: filters.search ? (
                  <InputAdornment position="end">
                    <IconButton
                      size="small"
                      edge="end"
                      onClick={() => onChange({ ...filters, search: "" })}
                      aria-label="Clear search"
                    >
                      <X size={16} />
                    </IconButton>
                  </InputAdornment>
                ) : undefined,
              },
            }}
          />
        </Box>

        <Button
          variant="outlined"
          size="small"
          color="inherit"
          onClick={(e) => setSavedAnchor(e.currentTarget)}
          startIcon={<Bookmark size={16} />}
          endIcon={<ChevronDown size={16} />}
          aria-haspopup="true"
          aria-expanded={Boolean(savedAnchor)}
        >
          Saved views
        </Button>
        <Menu
          anchorEl={savedAnchor}
          open={Boolean(savedAnchor)}
          onClose={() => setSavedAnchor(null)}
          anchorOrigin={{ vertical: "bottom", horizontal: "left" }}
        >
          <MenuItem
            onClick={() => {
              setSavedAnchor(null);
              setSaveDialogOpen(true);
            }}
          >
            <ListItemIcon>
              <BookmarkPlus size={16} />
            </ListItemIcon>
            <ListItemText primary="Save current view…" />
          </MenuItem>
          <Divider />
          <ListSubheader sx={{ lineHeight: "32px" }}>Saved</ListSubheader>
          {savedViews.length === 0 ? (
            <MenuItem disabled>
              <ListItemText
                primary="No saved views yet"
                slotProps={{ primary: { variant: "body2" } }}
              />
            </MenuItem>
          ) : (
            savedViews.map((v, i) => (
              <MenuItem
                key={`saved-${v.name}`}
                selected={isActiveView(v.qs)}
                onClick={() => applyView(v.qs)}
              >
                <ListItemIcon>
                  {isActiveView(v.qs) ? <Check size={16} /> : null}
                </ListItemIcon>
                <ListItemText primary={v.name} />
                <IconButton
                  size="small"
                  edge="end"
                  aria-label={`Move saved view ${v.name} up`}
                  disabled={i === 0}
                  onClick={(e) => {
                    e.stopPropagation();
                    moveFilterView(v.name, "up");
                  }}
                  sx={{ ml: 1 }}
                >
                  <ChevronUp size={15} />
                </IconButton>
                <IconButton
                  size="small"
                  edge="end"
                  aria-label={`Move saved view ${v.name} down`}
                  disabled={i === savedViews.length - 1}
                  onClick={(e) => {
                    e.stopPropagation();
                    moveFilterView(v.name, "down");
                  }}
                >
                  <ChevronDown size={15} />
                </IconButton>
                <IconButton
                  size="small"
                  edge="end"
                  aria-label={`Delete saved view ${v.name}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    deleteFilterView(v.name);
                  }}
                >
                  <Trash2 size={15} />
                </IconButton>
              </MenuItem>
            ))
          )}
        </Menu>

        <Button
          variant="outlined"
          size="small"
          onClick={hasActive ? handleReset : onFiltersToggle}
          startIcon={hasActive ? <X size={16} /> : <ListFilter size={16} />}
          endIcon={
            !hasActive &&
            (isFiltersOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />)
          }
        >
          {hasActive ? `Clear filters (${activeCount})` : "Filters"}
        </Button>
      </Box>

      {/* Fields with no bar control of their own (see `buildActiveFilterChips`'s
          doc comment) — shown regardless of `isFiltersOpen` so a
          dashboard-filtered arrival is self-explanatory even with the filter
          grid collapsed, and each is individually removable right here. */}
      {activeFilterChips.length > 0 && (
        <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
          {activeFilterChips.map((chip) => (
            <Chip
              key={chip.key}
              size="small"
              label={chip.label}
              onDelete={() => onChange(chip.onRemove(filters))}
            />
          ))}
        </Box>
      )}

      <Dialog
        open={saveDialogOpen}
        onClose={() => setSaveDialogOpen(false)}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>Save current view</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            size="small"
            margin="dense"
            label="View name"
            placeholder="e.g. My open S1/S2"
            value={newViewName}
            onChange={(e) => setNewViewName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                handleSaveView();
              }
            }}
            helperText={
              activeCount === 0
                ? "Tip: no filters are active — this view will show all cases."
                : `Captures the ${activeCount} active filter${activeCount === 1 ? "" : "s"}.`
            }
          />
        </DialogContent>
        <DialogActions>
          <Button color="inherit" onClick={() => setSaveDialogOpen(false)}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleSaveView}
            disabled={!newViewName.trim()}
          >
            Save
          </Button>
        </DialogActions>
      </Dialog>

      {/* Collapsible filter grid. Severity / state / case type are fixed
          multi-selects; assignee / project are type-to-search Autocompletes. */}
      {isFiltersOpen && (
        <>
          <Divider />
          <Box
            role="group"
            aria-label="Filter mode"
            sx={{ display: "flex", alignSelf: "flex-start" }}
          >
            <Tooltip
              title={
                canShowSimple
                  ? ""
                  : "One or more active filters is only representable in Advanced mode — " +
                    "switching to Quick filters clears them (Advanced remembers them until you change anything here)."
              }
            >
              <span>
                <Button
                  size="small"
                  variant={effectiveMode === "simple" ? "contained" : "outlined"}
                  color={effectiveMode === "simple" ? "primary" : "inherit"}
                  aria-pressed={effectiveMode === "simple"}
                  onClick={handleSwitchToSimple}
                  sx={{ borderTopRightRadius: 0, borderBottomRightRadius: 0 }}
                >
                  Quick filters
                </Button>
              </span>
            </Tooltip>
            <Button
              size="small"
              variant={effectiveMode === "advanced" ? "contained" : "outlined"}
              color={effectiveMode === "advanced" ? "primary" : "inherit"}
              aria-pressed={effectiveMode === "advanced"}
              onClick={handleSwitchToAdvanced}
              sx={{ borderTopLeftRadius: 0, borderBottomLeftRadius: 0, ml: "-1px" }}
            >
              Advanced
            </Button>
          </Box>
          {effectiveMode === "simple" ? (
          <Grid container spacing={2} sx={{ mt: 0 }}>
            {showSeverityFilter && (
              <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
                <MultiSelectField
                  id="cases-filter-severity"
                  label="Severity"
                  values={filters.severities}
                  options={severityOptions}
                  onChange={(next) => handleSimpleFieldChange({ ...filters, severities: next })}
                />
              </Grid>
            )}
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              {/* Plain include-only multi-select, same as every other
                  Simple-mode field. `state`/`notIn` is still expressible via
                  the Advanced-mode "State" row (or a dashboard click-through
                  seeding `excludeStates`, surfaced as a removable chip — see
                  `buildActiveFilterChips`) — this control only ever writes
                  `states`. */}
              <MultiSelectField
                id="cases-filter-state"
                label="State"
                values={filters.states}
                options={stateOptions}
                // Work sub-state only applies when `work_in_progress` is the
                // *sole* selected state — with any other state also
                // selected, the work-state filter can't be applied
                // server-side, so drop any selected work states as soon as
                // the selection stops being exactly that one state.
                onChange={(next) =>
                  handleSimpleFieldChange({
                    ...filters,
                    states: next,
                    workStates:
                      next.length === 1 && next[0] === "work_in_progress"
                        ? filters.workStates
                        : [],
                  })
                }
              />
            </Grid>
            {!hideCreTeamFilter && (
              <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
                {/* CS team the case's project is scoped to (`creTeam`). Options
                    are `creGroupId`s (what the filter actually matches on);
                    labels are team display names, never the raw group-id
                    UUID. `workStates` has no bar control of its own now (it's
                    a narrow, rarely hand-picked sub-filter of "state") -- it
                    still round-trips losslessly via the URL/a saved view/a
                    dashboard click-through, surfaced as a removable chip
                    instead (see `buildActiveFilterChips`). */}
                <MultiSelectField
                  id="cases-filter-cs-team"
                  label="CRE Team"
                  values={filters.csTeams}
                  options={teamOptions}
                  onChange={(next) => handleSimpleFieldChange({ ...filters, csTeams: next })}
                />
              </Grid>
            )}
            {showEngagementTypeFilter && (
              <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
                <MultiSelectField
                  id="cases-filter-engagement-type"
                  label="Engagement type"
                  values={filters.engagementTypes}
                  options={engagementTypeOptions}
                  onChange={(next) => handleSimpleFieldChange({ ...filters, engagementTypes: next })}
                />
              </Grid>
            )}
            {!hideTypeFilter && (
              <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
                <MultiSelectField
                  id="cases-filter-type"
                  label={typeFilterLabel}
                  values={filters.caseTypes}
                  options={caseTypeOptions}
                  onChange={(next) => handleSimpleFieldChange({ ...filters, caseTypes: next })}
                />
              </Grid>
            )}
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              {/* Email/`@me`-based picker; `useGetCsmCases` resolves the
                  selection to the UUIDs `/cases/search` expects (`@me` via the
                  app-wide current-user context, named engineers via
                  `/users/search`). Searches the directory as you type. */}
              <AsyncAssigneeMultiSelect
                values={filters.assignees}
                onChange={(next) => handleSimpleFieldChange({ ...filters, assignees: next })}
                nameSeed={assigneeNameSeed}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              {/* Product family filter; the selected names map straight to
                  `productNames` (SN matches product.name, all versions). */}
              <ProductNameMultiSelect
                values={filters.productNames}
                onChange={(next) => handleSimpleFieldChange({ ...filters, productNames: next })}
              />
            </Grid>
            {!hideOnboardingStatusFilter && (
              <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
                <MultiSelectField
                  id="cases-filter-onboarding-status"
                  label="Onboarding status"
                  values={filters.onboardingStatuses}
                  options={ONBOARDING_STATUS_OPTIONS}
                  onChange={(next) => handleSimpleFieldChange({ ...filters, onboardingStatuses: next })}
                />
              </Grid>
            )}
            {!hideProjectFilter && (
              // Last, and wider than every other control: selected project
              // names render as one ellipsized line (see
              // `AsyncProjectMultiSelect`'s `renderTags`), not a wrap, but at
              // the same `lg: 2` width as everything else that single line
              // was cramped enough to ellipsize almost immediately with more
              // than one project picked. Moved to the end of the grid and
              // widened so it has room to actually show a project name or two
              // before truncating.
              <Grid size={{ xs: 12, sm: 12, md: 6, lg: 4 }}>
                <AsyncProjectMultiSelect
                  values={filters.projects}
                  onChange={(next) => handleSimpleFieldChange({ ...filters, projects: next })}
                  nameSeed={projectNameSeed}
                />
              </Grid>
            )}
          </Grid>
          ) : (
            <>
              <AdvancedFiltersBuilder
                rows={unifiedRows}
                onUpdateRow={(row, next) => onChange(updateUnifiedRow(filters, row, next))}
                onRemoveRow={(row) => onChange(removeUnifiedRow(filters, row))}
                onAddRow={() => onChange(addBlankUnifiedRow(filters))}
                creTeamOptions={teamOptions}
                sreTeamOptions={sreTeamOptions}
                assigneeNameSeed={assigneeNameSeed}
                projectNameSeed={projectNameSeed}
              />
              <Divider />
              <AnyOfGroupsBuilder
                branches={filters.anyOfBranches}
                onChange={(next) => onChange({ ...filters, anyOfBranches: next })}
              />
            </>
          )}
          {activeCount > 0 && (
            <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
              <Typography variant="caption" color="text.secondary">
                {activeCount} {activeCount === 1 ? "filter" : "filters"} active
              </Typography>
            </Box>
          )}
        </>
      )}
    </Paper>
  );
}
