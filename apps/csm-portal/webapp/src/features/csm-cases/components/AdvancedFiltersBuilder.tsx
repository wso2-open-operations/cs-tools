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
import { useState, type JSX } from "react";
import MultiSelectField from "@components/MultiSelectField";
import AsyncCreatedByMultiSelect from "@features/csm-cases/components/AsyncCreatedByMultiSelect";
import AsyncAssigneeMultiSelect from "@features/csm-cases/components/AsyncAssigneeMultiSelect";
import AsyncProjectMultiSelect from "@features/csm-cases/components/AsyncProjectMultiSelect";
import ProductNameMultiSelect from "@features/csm-cases/components/ProductNameMultiSelect";
import AsyncTagMultiSelect from "@features/csm-cases/components/AsyncTagMultiSelect";
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
 */
function DateOrPresetValueInput({
  labelId,
  value,
  onChange,
}: DateOrPresetValueInputProps): JSX.Element {
  const [mode, setMode] = useState<"preset" | "custom">(
    value && !isRelativeDatePreset(value) ? "custom" : "preset",
  );

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
              onChange("");
            } else {
              setMode("preset");
              onChange(next);
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
            value={parseDateOnly(value)}
            onChange={(date) =>
              onChange(
                date instanceof Date && !Number.isNaN(date.getTime())
                  ? formatDateOnly(date)
                  : "",
              )
            }
            slotProps={{
              textField: { size: "small", fullWidth: true },
              field: { clearable: true },
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
                />
              )}
              {opMeta?.valueKind === "asyncAssigneeMultiSelect" && (
                <AsyncAssigneeMultiSelect
                  id={`advanced-filter-value-${index}`}
                  label="Value(s)"
                  values={row.values}
                  onChange={(next) => onUpdateRow(row, { ...asRow(row), values: next })}
                  nameSeed={assigneeNameSeed}
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
