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
  AdapterDateFns,
  Box,
  Button,
  DatePickers,
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useRef, useState, type JSX } from "react";
import MultiSelectField from "@components/MultiSelectField";
import AsyncCreatedByMultiSelect from "@features/csm-cases/components/AsyncCreatedByMultiSelect";
import AsyncAssigneeMultiSelect from "@features/csm-cases/components/AsyncAssigneeMultiSelect";
import AsyncProjectMultiSelect from "@features/csm-cases/components/AsyncProjectMultiSelect";
import AsyncAccountMultiSelect from "@features/csm-cases/components/AsyncAccountMultiSelect";
import ProductNameMultiSelect from "@features/csm-cases/components/ProductNameMultiSelect";
import AsyncTagMultiSelect from "@features/csm-cases/components/AsyncTagMultiSelect";
import { INTERNAL_USER_ROLES } from "@features/csm-users/types/csmUsers";
import {
  ADVANCED_FILTER_FIELDS,
  RELATIVE_DATE_PRESETS,
  getAdvancedFilterFieldMeta,
  getAdvancedFilterOpMeta,
  type AdvancedFilterField,
  type AdvancedFilterRow,
} from "@features/csm-cases/utils/advancedFilters";
import type { UnifiedFilterRow } from "@features/csm-cases/utils/filterFieldAdapters";

const { DatePicker, LocalizationProvider } = DatePickers;

interface AdvancedFiltersBuilderProps {
  /** The unified row list — one row per non-empty typed field, plus every
   * untyped ad-hoc row, see `filtersToAdvancedRows`. */
  rows: UnifiedFilterRow[];
  onUpdateRow: (row: UnifiedFilterRow, next: AdvancedFilterRow) => void;
  onRemoveRow: (row: UnifiedFilterRow) => void;
  onAddRow: () => void;
  /** CS team options (`creGroupId` → display name) for the `creTeam` row —
   * fetched data, not part of the static catalogue. */
  creTeamOptions: { value: string; label: string }[];
  /** SRE team options (`sreGroupId` → display name) for the `sreTeam` row —
   * same reasoning as `creTeamOptions`. */
  sreTeamOptions: { value: string; label: string }[];
  /** Known email → name pairs for the `assignedUserId` row's value input
   * (`AsyncAssigneeMultiSelect`), so already-selected chips are labelled
   * before any search has run — same seed the Simple grid's own "Assignee"
   * control uses. */
  assigneeNameSeed?: Map<string, string>;
  /** Known id → name pairs for the `projectId` row's value input
   * (`AsyncProjectMultiSelect`) — same seed the Simple grid's own "Project"
   * control uses. */
  projectNameSeed?: Map<string, string>;
  /** Known id → name pairs for the `accountId` row's value input
   * (`AsyncAccountMultiSelect`) — same shape as `projectNameSeed`, but
   * `accountId` has no Simple-grid control of its own to seed it from, so
   * this is currently always empty in practice; kept for parity/future use. */
  accountNameSeed?: Map<string, string>;
}

/** "YYYY-MM-DD" to a local-midnight Date (avoids the UTC-parse day-shift
 * `new Date(dateString)` can cause depending on the viewer's timezone) —
 * same helper `DateRangeFilter`/`ChangeRequestsFilterBar` each keep locally
 * for their own date-only fields; duplicated here for the same reason
 * `DateRangeFilter` duplicates it rather than importing across features. */
function parseDateOnly(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Local-midnight Date back to "YYYY-MM-DD". */
function formatDateOnly(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

const CUSTOM_DATE_SENTINEL = "__custom_date__";

function isRelativeDatePreset(value: string): boolean {
  return RELATIVE_DATE_PRESETS.some((p) => p.value === value);
}

interface DateOrPresetValueInputProps {
  /** Unique per rendered row, so the label/`labelId` pair stays unambiguous
   * when several date rows are open at once. */
  labelId: string;
  /** A relative-date placeholder (one of `RELATIVE_DATE_PRESETS`), a literal
   * `YYYY-MM-DD`, or `""` (nothing chosen yet). */
  value: string;
  onChange: (next: string) => void;
}

/** How long the custom date picker waits after the last edit before
 * committing a *complete, valid* date upstream — see `DateOrPresetValueInput`'s
 * own doc comment for why this exists and why it only applies to that one
 * path (never to a preset pick or an explicit clear, both already instant,
 * discrete actions with nothing to coalesce). */
const CUSTOM_DATE_COMMIT_DEBOUNCE_MS = 300;

/**
 * The `createdOn`/`updatedOn`/`closedOn` row's value input: a preset
 * dropdown (human labels for the common relative-date placeholders — see
 * `RELATIVE_DATE_PRESETS`) plus an actual calendar date picker for an exact
 * day, so neither the placeholder grammar (`__daysAgo:N__`, ...) nor a raw
 * `YYYY-MM-DD` ever has to be hand-typed. Mode (`preset` vs `custom`) is
 * local state seeded from the incoming value, since a bare string can't
 * distinguish "no date chosen yet" from "chose Custom, haven't picked a day
 * yet" — the caller should key this component by `field-op` (see
 * `AdvancedFiltersBuilder`) so switching to a different date row/op resets
 * that local state instead of carrying it over.
 *
 * The calendar's own `value` is local state (`localDate`), not derived
 * directly from the incoming `value` prop on every render — found live as a
 * real bug otherwise: MUI's `DatePicker` reports an invalid (`NaN`) `Date`
 * on every keystroke while a masked `MM/DD/YYYY` field is still incomplete
 * (e.g. month and day typed, year not finished yet). Reading `value`
 * straight off `parseDateOnly(value)` and treating "not a valid complete
 * date" as "clear it" committed `""` upstream on every one of those
 * keystrokes, which then round-tripped back down through `value` and reset
 * the field to empty — wiping out the month/day the user had already typed
 * the moment they started on the year. Keeping the displayed date as local
 * state, updated on every keystroke regardless of completeness, and only
 * ever pushing a value upstream for a complete, valid `Date` fixes this
 * without losing anything: MUI never fights a controlled value that hasn't
 * itself changed, so the field keeps whatever's been typed so far during
 * every intermediate, incomplete render.
 *
 * `null` from the picker's own `onChange` is deliberately never treated as
 * "clear it," here or anywhere else in this component — MUI reports `null`
 * both for the field's clear button AND for removing a single section while
 * editing (e.g. backspacing just the day), confirmed against this app's own
 * pinned `@mui/x-date-pickers` version. Committing a clear for the second
 * case would wipe the filter in the middle of an edit the user never meant
 * to abandon. The field's own dedicated `onClear` slot (wired in
 * `slotProps.field` below) is the one unambiguous signal for an actual,
 * deliberate clear, and is the only place one is committed.
 *
 * A complete, valid date is also debounced before being committed upstream
 * (`CUSTOM_DATE_COMMIT_DEBOUNCE_MS`), the same technique `useDebouncedValue`
 * applies elsewhere in this app, but implemented with a cancellable
 * `setTimeout` here rather than that hook directly: a rapid run of valid
 * intermediate dates (e.g. holding the calendar's day/month stepper, or
 * quickly clicking several days in the popup) would otherwise commit a
 * value — and, through this row's own `onUpdateRow`, likely re-run a
 * search — once per intermediate value instead of once the user actually
 * settles. Every picker change cancels whatever commit was previously
 * scheduled, before deciding whether to schedule a new one: an in-progress
 * edit (an incomplete date, or a section the user just removed) must never
 * let an earlier, now-superseded valid date commit out from under it a
 * moment later. An explicit clear is committed immediately, never debounced:
 * a deliberate, discrete action reads as unresponsive if delayed the same
 * way a mid-typing keystroke is. A preset pick (the dropdown below) is a
 * separate, already-discrete action and was never debounced; picking one
 * also cancels any still-pending custom-date commit, so a quick switch away
 * from a half-typed custom date can never have that stale value land after
 * the preset the user actually chose.
 *
 * This component also stays in sync with an externally-driven change to its
 * own `value` — e.g. a "clear all filters" action resetting this exact
 * field/op's value while the row stays mounted (same key, so the `useState`
 * initializers above don't re-run). `lastCommittedValueRef` tracks what this
 * component itself last told the parent; when `value` changes to something
 * else, that can only have come from outside, so the displayed date is
 * re-synced from it and any of this component's own still-pending commits —
 * which would otherwise silently overwrite the external change a moment
 * later — are dropped. The debounced commit itself always calls the latest
 * `onChange` prop (via `onChangeRef`), not the one captured when the timer
 * was scheduled: `AdvancedFiltersBuilder` builds a fresh closure over the
 * current row/filter state on every render, so a commit firing after the
 * caller has since edited a *different* filter must still go through
 * whatever callback is current, not a stale one closing over stale state.
 */
function DateOrPresetValueInput({
  labelId,
  value,
  onChange,
}: DateOrPresetValueInputProps): JSX.Element {
  const [mode, setMode] = useState<"preset" | "custom">(
    value && !isRelativeDatePreset(value) ? "custom" : "preset",
  );
  const [localDate, setLocalDate] = useState<Date | null>(() => parseDateOnly(value));
  const commitTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Always the latest `onChange` prop -- read by the debounced commit below
  // instead of closing over the callback from the render that scheduled it.
  // `AdvancedFiltersBuilder` builds a fresh `onChange` closure over the
  // current `row`/filter state on every render (one per row, not shared),
  // so if the caller edits a *different* filter while this row's commit is
  // still pending, the debounced timer must call whatever `onChange` is
  // current when it actually fires, not the one captured when it was
  // scheduled -- otherwise it would commit through a stale callback closing
  // over stale filter state, discarding the intervening edit.
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  // What this component itself last told the parent, so the effect below
  // can tell "the parent changed `value` out from under us" (e.g. a
  // clear-all-filters action resetting this exact field/op's value while
  // the row stays mounted) apart from "`value` just caught up with our own
  // last commit" (the ordinary round trip after any commit below).
  const lastCommittedValueRef = useRef(value);

  const cancelPendingCommit = (): void => {
    if (commitTimeoutRef.current) {
      clearTimeout(commitTimeoutRef.current);
      commitTimeoutRef.current = null;
    }
  };
  // Cancel a still-pending debounced commit if this row is removed (or
  // re-keyed to a different field/op) while the timer is in flight, so it
  // can never fire `onChange` against a row that's no longer this one.
  useEffect(() => cancelPendingCommit, []);

  // Adopt an externally-driven value change (not our own echo) and drop
  // anything of our own still pending, which would otherwise overwrite it
  // moments later.
  useEffect(() => {
    if (value !== lastCommittedValueRef.current) {
      lastCommittedValueRef.current = value;
      cancelPendingCommit();
      setLocalDate(parseDateOnly(value));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  const commitNow = (next: string): void => {
    cancelPendingCommit();
    lastCommittedValueRef.current = next;
    onChangeRef.current(next);
  };

  const commitDateDebounced = (next: string): void => {
    cancelPendingCommit();
    commitTimeoutRef.current = setTimeout(() => {
      commitTimeoutRef.current = null;
      lastCommittedValueRef.current = next;
      onChangeRef.current(next);
    }, CUSTOM_DATE_COMMIT_DEBOUNCE_MS);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <FormControl size="small" fullWidth>
        {/* `displayEmpty` always renders *something* in the value area (the
            "Choose…" placeholder item when nothing's picked yet) — MUI's
            default shrink-on-value logic treats an empty value as "nothing
            to shrink for" though, so the floating label sits in its
            unshrunk, centered position right on top of that placeholder
            text. Force it shrunk always, same fix as MultiSelectField's own
            `displayEmpty`-style empty state. */}
        <InputLabel id={labelId} shrink>
          Date
        </InputLabel>
        <Select
          labelId={labelId}
          label="Date"
          notched
          value={mode === "custom" ? CUSTOM_DATE_SENTINEL : value}
          displayEmpty
          onChange={(e) => {
            const next = e.target.value;
            if (next === CUSTOM_DATE_SENTINEL) {
              setMode("custom");
              setLocalDate(null);
              commitNow("");
            } else {
              setMode("preset");
              commitNow(next);
            }
          }}
        >
          <MenuItem value="">Choose…</MenuItem>
          {RELATIVE_DATE_PRESETS.map((p) => (
            <MenuItem key={p.value} value={p.value}>
              {p.label}
            </MenuItem>
          ))}
          <MenuItem value={CUSTOM_DATE_SENTINEL}>Custom date…</MenuItem>
        </Select>
      </FormControl>
      {mode === "custom" && (
        <LocalizationProvider dateAdapter={AdapterDateFns}>
          <DatePicker
            label="Exact date"
            value={localDate}
            onChange={(date) => {
              setLocalDate(date);
              // Cancel whatever was previously scheduled on *every* change,
              // before deciding whether to schedule a new one -- an
              // in-progress edit (an incomplete date, or a section the user
              // just removed) must never let an earlier, now-superseded
              // valid date commit out from under it a moment later.
              cancelPendingCommit();
              if (date instanceof Date && !Number.isNaN(date.getTime())) {
                // A complete, valid date -- debounce the actual commit.
                commitDateDebounced(formatDateOnly(date));
              }
              // `null` and an incomplete date are both left alone otherwise.
              // `null` is deliberately NOT treated as "clear it" here: MUI
              // reports it both for the field's own clear button AND for
              // removing a single section while editing (e.g. backspacing
              // just the day) -- treating the second as a full clear would
              // wipe the committed filter mid-edit. The clear button's own
              // dedicated `onClear` slot below is the one unambiguous
              // signal for an actual, deliberate clear.
            }}
            slotProps={{
              textField: { size: "small", fullWidth: true },
              field: {
                clearable: true,
                onClear: () => {
                  setLocalDate(null);
                  commitNow("");
                },
              },
            }}
          />
        </LocalizationProvider>
      )}
    </Box>
  );
}

/** Splits a comma-separated free-text entry into a trimmed, non-empty array. */
function splitCsv(raw: string): string[] {
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

/**
 * Strips a {@link UnifiedFilterRow}'s `origin`/`arrayIndex` bookkeeping back
 * down to a plain {@link AdvancedFilterRow} before spreading it into an edit.
 * Every `onUpdateRow(row, { ...row, ... })`-shaped call site below needs
 * this — `UnifiedFilterRow extends AdvancedFilterRow`, so `{ ...row, ... }`
 * type-checks fine but silently carries `origin`/`arrayIndex` into what's
 * supposed to be a clean row, and (via the "stays in the untyped array"
 * branch of `updateUnifiedRow`) that cruft would otherwise land inside a
 * `filters.advancedFilters` entry — state that round-trips through the URL
 * and the `/cases/search` payload builder, neither of which expects it.
 */
function asRow(row: AdvancedFilterRow): AdvancedFilterRow {
  return { field: row.field, op: row.op, values: row.values };
}

/**
 * The unified "Advanced filters" field/op/value row builder — every field
 * `/cases/search` accepts (see `advancedFilters.ts`'s catalogue), including
 * the ones that also have a dedicated Simple-grid control. A row's field is
 * itself a pickable dropdown (not fixed per row): picking, say, "Severity"
 * here edits the exact same `filters.severities` the Simple grid's own
 * "Severity" control does (see `filterFieldAdapters.ts`'s typed-adapter
 * registry) — there is only ever one place a given predicate lives, this
 * builder just offers a second, more flexible way to edit it.
 */
export default function AdvancedFiltersBuilder({
  rows,
  onUpdateRow,
  onRemoveRow,
  onAddRow,
  creTeamOptions,
  sreTeamOptions,
  assigneeNameSeed,
  projectNameSeed,
  accountNameSeed,
}: AdvancedFiltersBuilderProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="subtitle2" color="text.secondary">
        Advanced filters
      </Typography>
      {rows.map((row, index) => {
        const fieldMeta = getAdvancedFilterFieldMeta(row.field);
        const opMeta = getAdvancedFilterOpMeta(row.field, row.op);
        // Stable-ish key: typed rows are keyed by field+op (there is only
        // ever one row per field+op, whether typed or not), array rows by
        // their array index — matches how `updateUnifiedRow` addresses them.
        const rowKey =
          row.origin === "typed"
            ? `typed-${row.field}-${row.op}`
            : `array-${row.arrayIndex}`;
        return (
          <Box
            key={rowKey}
            sx={{ display: "flex", gap: 1, alignItems: "flex-start", flexWrap: "wrap" }}
          >
            <FormControl size="small" sx={{ minWidth: 200 }}>
              <InputLabel id={`advanced-filter-field-${index}-label`}>Field</InputLabel>
              <Select
                labelId={`advanced-filter-field-${index}-label`}
                label="Field"
                value={row.field}
                onChange={(e) => {
                  const nextField = e.target.value as AdvancedFilterField;
                  const nextFieldMeta = getAdvancedFilterFieldMeta(nextField);
                  const nextOp = nextFieldMeta?.ops[0]?.op ?? row.op;
                  onUpdateRow(row, { field: nextField, op: nextOp, values: [] });
                }}
              >
                {ADVANCED_FILTER_FIELDS.map((m) => (
                  <MenuItem key={m.field} value={m.field}>
                    {m.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            <FormControl size="small" sx={{ minWidth: 160 }}>
              <InputLabel id={`advanced-filter-op-${index}-label`}>Operator</InputLabel>
              <Select
                labelId={`advanced-filter-op-${index}-label`}
                label="Operator"
                value={row.op}
                onChange={(e) => {
                  onUpdateRow(row, { ...asRow(row), op: e.target.value as typeof row.op, values: [] });
                }}
              >
                {(fieldMeta?.ops ?? []).map((o) => (
                  <MenuItem key={o.op} value={o.op}>
                    {o.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            <Box sx={{ minWidth: 220, maxWidth: 320, flex: "1 1 220px" }}>
              {opMeta?.valueKind === "multiText" && (
                <TextField
                  size="small"
                  fullWidth
                  label="Value(s)"
                  placeholder={fieldMeta?.placeholder ?? "Comma-separated values"}
                  value={row.values.join(", ")}
                  onChange={(e) =>
                    onUpdateRow(row, { ...asRow(row), values: splitCsv(e.target.value) })
                  }
                  helperText={
                    fieldMeta?.suggestions?.length
                      ? `Suggestions: ${fieldMeta.suggestions.join(", ")}`
                      : "Comma-separated"
                  }
                />
              )}
              {opMeta?.valueKind === "multiSelect" && (
                <MultiSelectField
                  id={`advanced-filter-value-${index}`}
                  label="Value(s)"
                  values={row.values}
                  // `creTeam`/`sreTeam` options are fetched data (the team
                  // registry), not part of the static catalogue -- see
                  // `creTeamOptions`/`sreTeamOptions`'s own doc comments.
                  options={
                    row.field === "creTeam"
                      ? creTeamOptions
                      : row.field === "sreTeam"
                        ? sreTeamOptions
                        : (fieldMeta?.options ?? [])
                  }
                  onChange={(next) => onUpdateRow(row, { ...asRow(row), values: next })}
                />
              )}
              {opMeta?.valueKind === "asyncEmailMultiSelect" && (
                <AsyncCreatedByMultiSelect
                  values={row.values}
                  onChange={(next) => onUpdateRow(row, { ...asRow(row), values: next })}
                  roleIds={INTERNAL_USER_ROLES}
                  active
                />
              )}
              {opMeta?.valueKind === "asyncAssigneeMultiSelect" && (
                <AsyncAssigneeMultiSelect
                  id={`advanced-filter-value-${index}`}
                  label="Value(s)"
                  values={row.values}
                  onChange={(next) => onUpdateRow(row, { ...asRow(row), values: next })}
                  nameSeed={assigneeNameSeed}
                  roleIds={INTERNAL_USER_ROLES}
                  active
                />
              )}
              {opMeta?.valueKind === "asyncProjectMultiSelect" && (
                <AsyncProjectMultiSelect
                  id={`advanced-filter-value-${index}`}
                  label="Value(s)"
                  values={row.values}
                  onChange={(next) => onUpdateRow(row, { ...asRow(row), values: next })}
                  nameSeed={projectNameSeed}
                />
              )}
              {opMeta?.valueKind === "asyncAccountMultiSelect" && (
                <AsyncAccountMultiSelect
                  id={`advanced-filter-value-${index}`}
                  label="Value(s)"
                  values={row.values}
                  onChange={(next) => onUpdateRow(row, { ...asRow(row), values: next })}
                  nameSeed={accountNameSeed}
                />
              )}
              {opMeta?.valueKind === "asyncProductMultiSelect" && (
                <ProductNameMultiSelect
                  id={`advanced-filter-value-${index}`}
                  label="Value(s)"
                  values={row.values}
                  onChange={(next) => onUpdateRow(row, { ...asRow(row), values: next })}
                />
              )}
              {opMeta?.valueKind === "asyncTagMultiSelect" && (
                <AsyncTagMultiSelect
                  id={`advanced-filter-value-${index}`}
                  values={row.values}
                  onChange={(next) => onUpdateRow(row, { ...asRow(row), values: next })}
                />
              )}
              {opMeta?.valueKind === "text" && (
                <TextField
                  size="small"
                  fullWidth
                  label="Value"
                  placeholder={fieldMeta?.placeholder}
                  value={row.values[0] ?? ""}
                  onChange={(e) =>
                    onUpdateRow(row, {
                      ...asRow(row),
                      values: e.target.value ? [e.target.value] : [],
                    })
                  }
                />
              )}
              {opMeta?.valueKind === "number" && (
                <TextField
                  size="small"
                  fullWidth
                  type="number"
                  label="Value"
                  value={row.values[0] ?? ""}
                  onChange={(e) =>
                    onUpdateRow(row, {
                      ...asRow(row),
                      values: e.target.value ? [e.target.value] : [],
                    })
                  }
                />
              )}
              {opMeta?.valueKind === "dateOrPreset" && (
                <DateOrPresetValueInput
                  key={`${row.field}-${row.op}`}
                  labelId={`advanced-filter-date-mode-${index}-label`}
                  value={row.values[0] ?? ""}
                  onChange={(next) => onUpdateRow(row, { ...asRow(row), values: next ? [next] : [] })}
                />
              )}
              {(opMeta?.valueKind === "none" || opMeta?.valueKind === "currentUser") && (
                <Typography variant="caption" color="text.secondary" sx={{ lineHeight: "40px" }}>
                  {opMeta.valueKind === "currentUser" ? "The signed-in user" : "No value needed"}
                </Typography>
              )}
            </Box>

            <IconButton
              size="small"
              aria-label="Remove filter row"
              onClick={() => onRemoveRow(row)}
              sx={{ mt: 0.5 }}
            >
              <Trash2 size={16} />
            </IconButton>
          </Box>
        );
      })}
      <Box>
        <Button size="small" variant="outlined" startIcon={<Plus size={16} />} onClick={onAddRow}>
          Add filter
        </Button>
      </Box>
    </Box>
  );
}
