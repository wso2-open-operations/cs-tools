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
  Autocomplete,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  MenuItem,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Eye, Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import type {
  BeDashboardFilterPreset,
  BeDashboardPieSlice,
  BeDashboardWidget,
  BeDashboardWidgetColumn,
  BeDashboardWidgetColumnFormat,
  BeWidgetPaletteColor,
  BeWidgetResourceType,
  BeWidgetShape,
} from "@api/backend/types";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import DashboardWidgetTile from "@features/csm-dashboard/components/DashboardWidgetTile";
import { useWidgetData } from "@features/csm-dashboard/api/useWidgetData";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import WidgetFilterConditionEditor from "@features/csm-admin/dashboards/components/WidgetFilterConditionEditor";
import { newWidgetId } from "@features/csm-admin/dashboards/utils/dashboardDraftsStorage";
import { discoverAttributePaths } from "@features/csm-admin/dashboards/utils/discoverAttributePaths";
import {
  filterConditionsFromQuery,
  queryFromFilterConditions,
  type FilterCondition,
} from "@features/csm-admin/dashboards/utils/widgetQueryConditions";

const RESOURCE_TYPES = Object.keys(WIDGET_RESOURCE_CONFIG) as BeWidgetResourceType[];
const SHAPES: BeWidgetShape[] = ["count", "list", "pie", "bar"];
const PALETTE_COLORS: BeWidgetPaletteColor[] = [
  "primary",
  "secondary",
  "success",
  "error",
  "info",
  "warning",
];
const COLUMN_FORMATS: BeDashboardWidgetColumnFormat[] = ["text", "date"];

interface SliceDraft {
  label: string;
  color?: BeWidgetPaletteColor;
  conditions: FilterCondition[];
}

/** A `columns` row while it's being edited — same shape as
 * `BeDashboardWidgetColumn` but `format` is normalized to `""` (rather than
 * `undefined`) so it round-trips cleanly through the `TextField select`
 * below, which needs a defined `value` to stay a controlled input. */
interface ColumnDraft {
  path: string;
  label: string;
  format: BeDashboardWidgetColumnFormat | "";
}

function columnsToDrafts(columns: BeDashboardWidgetColumn[] | undefined): ColumnDraft[] {
  return (columns ?? []).map((c) => ({ path: c.path, label: c.label, format: c.format ?? "" }));
}

// Mirrors `columns?: []` being a no-op on the wire (see
// `BeDashboardWidget.columns`'s own doc comment): a row missing either half
// of its identity (`path`/`label`) can't resolve or label a cell, so it's
// dropped rather than saved as a broken column.
function draftsToColumns(drafts: ColumnDraft[]): BeDashboardWidgetColumn[] {
  return drafts
    .filter((d) => d.path.trim().length > 0 && d.label.trim().length > 0)
    .map((d) => ({
      path: d.path.trim(),
      label: d.label.trim(),
      format: d.format || undefined,
    }));
}

function slicesToDrafts(
  resourceType: BeWidgetResourceType,
  slices: BeDashboardPieSlice[] | undefined,
  presets?: readonly BeDashboardFilterPreset[],
): SliceDraft[] {
  return (slices ?? []).map((s) => ({
    label: s.label,
    color: s.color,
    // Presets are expanded per-slice too (a slice carrying its own `filters`
    // replaces the widget's whole array), so a slice's filters need the same
    // collapse-back as the widget's own or a pie widget silently loses every
    // preset reference its slices had.
    conditions: filterConditionsFromQuery(resourceType, s.query, presets),
  }));
}

function draftsToSlices(resourceType: BeWidgetResourceType, drafts: SliceDraft[]): BeDashboardPieSlice[] {
  return drafts
    .filter((d) => d.label.trim().length > 0)
    .map((d) => ({
      label: d.label,
      color: d.color,
      query: queryFromFilterConditions(resourceType, d.conditions),
    }));
}

interface WidgetEditorDialogProps {
  /**
   * The shared filter-preset catalogue, resolved by the PAGE before this
   * dialog is mounted.
   *
   * It has to arrive as a prop rather than be fetched here: the filter rows
   * are seeded in a `useState` initializer, which runs once on mount, and
   * `filterConditionsFromQuery` needs the catalogue at that moment to show an
   * already-expanded deployed filter as the preset it came from. Fetching it
   * inside this component would seed the rows as literal predicates and then
   * have to overwrite them from an effect — the cascading-render pattern
   * `react-hooks/set-state-in-effect` (correctly) rejects. The page owns the
   * query, and this dialog is mounted only once it has settled, so the
   * initializer always sees the final value.
   *
   * `undefined` means the deployment has no presets (or the catalogue failed
   * to load): rows then render as literal predicates, which is correct
   * behaviour, just without the preset affordance.
   */
  presets?: readonly BeDashboardFilterPreset[];
  /** `undefined` when creating a brand-new widget. */
  widget: BeDashboardWidget | undefined;
  /** Pre-fills the section field for a brand-new widget created via a
   * specific section's own "Add widget" action (see the editor page) —
   * ignored when `widget` is set (editing keeps that widget's own
   * section). */
  defaultSection?: string;
  /** Existing section names on this dashboard draft, offered as
   * autocomplete suggestions (freeform text is still accepted — a widget
   * can also start a brand-new section right here). */
  sectionSuggestions: string[];
  /** The team's `creGroupId` the Preview tile below should scope its data
   * to, threaded through exactly as `DashboardWidgetGrid` threads it to
   * every real tile (see that component's own doc comment) — otherwise a
   * widget using the `__current_team__` filter placeholder in a `creTeam`
   * entry or a `{{currentTeam}}` display-text token previews unfiltered
   * data / an unresolved placeholder instead of what an admin would
   * actually see on the live dashboard. `undefined` for a non-team-based
   * dashboard, or while the team isn't resolved yet — see the editor
   * page's own doc comment for where this comes from. */
  selectedTeamCreGroupId?: string | string[];
  /** See `selectedTeamCreGroupId` above; the `sreTeam`-filter counterpart,
   * resolved independently. */
  selectedTeamSreGroupId?: string | string[];
  /** See `selectedTeamCreGroupId` above; the human-readable counterpart for
   * the `{{currentTeam}}` text token — see `DashboardWidgetGrid`. */
  selectedTeamLabel?: string;
  onClose: () => void;
  onSave: (widget: BeDashboardWidget) => void;
  onDelete?: () => void;
}

/**
 * Modal editor for a single dashboard widget: a form for everything a
 * `BeDashboardWidget` carries (display metadata, resourceType/shape,
 * filters, and shape-specific fields), plus a "Preview" button that renders
 * the in-progress config through the exact same `DashboardWidgetTile` the
 * live dashboard (and this builder's own grid) render with — so a "run the
 * current draft through the real resolution path" preview needs no
 * parallel fetch/render logic of its own.
 */
export default function WidgetEditorDialog({
  presets,
  widget,
  defaultSection,
  sectionSuggestions,
  selectedTeamCreGroupId,
  selectedTeamSreGroupId,
  selectedTeamLabel,
  onClose,
  onSave,
  onDelete,
}: WidgetEditorDialogProps): JSX.Element {
  const isNew = widget === undefined;
  const [widgetId] = useState(() => widget?.widgetId ?? newWidgetId());
  const [displayName, setDisplayName] = useState(widget?.displayName ?? "");
  const [description, setDescription] = useState(widget?.description ?? "");
  const [resourceType, setResourceType] = useState<BeWidgetResourceType>(
    widget?.resourceType ?? "case",
  );
  const [shape, setShape] = useState<BeWidgetShape>(widget?.shape ?? "count");
  const [section, setSection] = useState(widget?.section ?? defaultSection ?? "");
  const [gridWidth, setGridWidth] = useState(widget?.gridWidth ?? 3);
  const [listLimit, setListLimit] = useState<number | undefined>(widget?.listLimit);
  // No authoring UI for `groupBy` yet (it's now a real object config, not
  // the stale unused string this dialog used to expose as a raw text
  // field) — round-trip an existing widget's own `groupBy` verbatim so
  // editing this dialog without touching shape/resourceType doesn't
  // silently drop it. `groupBy.field` is only meaningful for the
  // resourceType it was configured under (there's no per-resourceType
  // groupBy field list to validate against, unlike `slices`' filter
  // conditions, so a resourceType change can't be reconciled — it's
  // cleared outright, same as `conditions`/`columns` below) and only for
  // shape `"pie"`/`"bar"` (the backend enforces `groupBy`/`slices` as
  // mutually exclusive) — cleared on either change so a widget edited away
  // from its original group-by config doesn't save a stale one.
  const [existingGroupBy, setExistingGroupBy] = useState(widget?.groupBy);
  const [conditions, setConditions] = useState<FilterCondition[]>(() =>
    filterConditionsFromQuery(widget?.resourceType ?? "case", widget?.query, presets),
  );
  const [sliceDrafts, setSliceDrafts] = useState<SliceDraft[]>(() =>
    slicesToDrafts(widget?.resourceType ?? "case", widget?.slices, presets),
  );


  const [columnDrafts, setColumnDrafts] = useState<ColumnDraft[]>(() =>
    columnsToDrafts(widget?.columns),
  );
  const [previewSnapshot, setPreviewSnapshot] = useState<BeDashboardWidget | undefined>();

  // A resourceType switch invalidates the previous filter shape entirely
  // (see widgetQueryConditions.ts's own doc comment) — rather than silently
  // reinterpreting stale rows against a contract they were never written
  // for, clear them and let the admin rebuild for the new resourceType.
  // Column `path`s are resource-specific too (e.g. `project.key` only
  // resolves for a case) — a stale path after switching resourceType would
  // render an empty cell under a now-misleading header, so those are
  // cleared right alongside the filter/slice conditions. The previous
  // Preview snapshot is stale for the same reason (its `discoveredColumnPaths`
  // are only valid for the resourceType they were fetched under), so it's
  // cleared too — the admin must re-run Preview for the new resourceType
  // before any column paths are offered again.
  const handleResourceTypeChange = (next: BeWidgetResourceType): void => {
    setResourceType(next);
    setConditions([]);
    setSliceDrafts((prev) => prev.map((d) => ({ ...d, conditions: [] })));
    setColumnDrafts([]);
    setPreviewSnapshot(undefined);
    // See `existingGroupBy`'s own doc comment: its `field` is only valid
    // for the resourceType it was configured under, and there's nothing
    // here to reconcile it against the new one.
    setExistingGroupBy(undefined);
  };

  // See `existingGroupBy`'s own doc comment: `groupBy` only makes sense for
  // shape `"pie"`/`"bar"` — clear it the moment the admin moves off either,
  // so re-selecting "pie" a moment later doesn't accidentally resurrect it
  // (the admin would need to know it was silently retained in state to
  // expect that) and so `buildWidget`'s own shape check below and this
  // state agree, rather than one masking a stale value the other would
  // otherwise expose again.
  const handleShapeChange = (next: BeWidgetShape): void => {
    setShape(next);
    if (next !== "pie" && next !== "bar") setExistingGroupBy(undefined);
  };

  const { user } = useCurrentUser();
  // Only meaningful for a list-shape widget (columns are the only thing
  // that needs real attribute paths) — and gated on `previewSnapshot`
  // rather than fetched eagerly on dialog open, so this piggybacks on the
  // exact same "admin clicked Preview" trigger the `DashboardWidgetTile`
  // below already reacts to instead of firing a second, earlier request.
  // Called with the *same* arguments `DashboardWidgetTile` passes to this
  // same hook internally (see that component's own `useWidgetData` call) —
  // matching args means a matching TanStack Query cache key, so this never
  // costs a second real network request; it just reads the one the Preview
  // tile is already making (or about to make).
  const columnPathSampleEnabled = previewSnapshot?.shape === "list";
  const { data: columnPathSampleData } = useWidgetData({
    widgetId: previewSnapshot?.widgetId ?? widgetId,
    resourceType: previewSnapshot?.resourceType ?? resourceType,
    filters: previewSnapshot?.query ?? {},
    shape: previewSnapshot?.shape ?? shape,
    listLimit: previewSnapshot?.listLimit,
    enabled: columnPathSampleEnabled,
    selectedTeamCreGroupId,
    selectedTeamSreGroupId,
    sortBy: previewSnapshot?.sortBy,
    currentUserId: user?.id,
  });
  // Paths actually reachable in the widget's own real Preview data, offered
  // as autocomplete options for a column's `path` field below — empty until
  // Preview has been run at least once for a list-shape widget, in which
  // case the path field just behaves like a plain text input (see its own
  // helper text).
  const discoveredColumnPaths = useMemo(
    () => (columnPathSampleData ? discoverAttributePaths(columnPathSampleData.items) : []),
    [columnPathSampleData],
  );

  const canSave = displayName.trim().length > 0 && gridWidth >= 1 && gridWidth <= 12;

  const buildWidget = (): BeDashboardWidget => {
    // Absent (not an empty array) when unconfigured — the same "no-op,
    // existing hardcoded renderer applies" convention
    // `BeDashboardWidget.columns`'s own doc comment documents, and the one
    // `DashboardWidgetGrid`'s passthrough (`columns={widget.columns}`) and
    // `DashboardWidgetTile`'s `hasColumns` check both rely on.
    const builtColumns = shape === "list" ? draftsToColumns(columnDrafts) : [];
    return {
      widgetId,
      displayName: displayName.trim(),
      description: description.trim() || undefined,
      resourceType,
      shape,
      gridWidth,
      query: queryFromFilterConditions(resourceType, conditions),
      section: section.trim() || undefined,
      // Belt-and-suspenders alongside `existingGroupBy` already being
      // cleared in state on an incompatible shape/resourceType change
      // (see its own doc comment): `buildWidget` is the actual
      // save-time serialization, so it re-asserts the same "pie"/"bar"
      // gate here rather than trusting state alone stayed in sync.
      groupBy: shape === "pie" || shape === "bar" ? existingGroupBy : undefined,
      listLimit: shape === "list" ? listLimit : undefined,
      // `groupBy` and `slices` are mutually exclusive on the wire (the
      // backend enforces this — see `existingGroupBy`'s own doc comment) —
      // an admin can still have stale rows sitting in `sliceDrafts` (the
      // manual slice editor below is hidden, not cleared, while
      // `existingGroupBy` is set), so this omits `slices` outright rather
      // than trusting the UI having hidden the editor was enough on its
      // own.
      slices:
        (shape === "pie" || shape === "bar") && existingGroupBy === undefined
          ? draftsToSlices(resourceType, sliceDrafts)
          : undefined,
      columns: builtColumns.length > 0 ? builtColumns : undefined,
    };
  };

  const handlePreview = (): void => setPreviewSnapshot(buildWidget());

  const handleSave = (): void => {
    if (!canSave) return;
    onSave(buildWidget());
  };

  const addSlice = (): void => setSliceDrafts((prev) => [...prev, { label: "", conditions: [] }]);
  const removeSlice = (index: number): void =>
    setSliceDrafts((prev) => prev.filter((_, i) => i !== index));
  const updateSlice = (index: number, patch: Partial<SliceDraft>): void =>
    setSliceDrafts((prev) => prev.map((d, i) => (i === index ? { ...d, ...patch } : d)));

  const addColumn = (): void =>
    setColumnDrafts((prev) => [...prev, { path: "", label: "", format: "" }]);
  const removeColumn = (index: number): void =>
    setColumnDrafts((prev) => prev.filter((_, i) => i !== index));
  const updateColumn = (index: number, patch: Partial<ColumnDraft>): void =>
    setColumnDrafts((prev) => prev.map((d, i) => (i === index ? { ...d, ...patch } : d)));

  const isChartShape = shape === "pie" || shape === "bar";
  const isListShape = shape === "list";
  const previewKey = useMemo(
    () => (previewSnapshot ? JSON.stringify(previewSnapshot) : undefined),
    [previewSnapshot],
  );

  return (
    <Dialog open onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>{isNew ? "Add widget" : "Edit widget"}</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <TextField
              label="Display name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              required
              size="small"
              sx={{ flex: "1 1 260px" }}
              slotProps={{ htmlInput: { "aria-label": "Widget display name" } }}
            />
            <TextField
              label="Description (optional)"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              size="small"
              sx={{ flex: "1 1 260px" }}
            />
          </Box>

          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <TextField
              select
              label="Resource type"
              value={resourceType}
              onChange={(e) => handleResourceTypeChange(e.target.value as BeWidgetResourceType)}
              size="small"
              sx={{ minWidth: 200 }}
              slotProps={{
                // `resourceType` always holds a real value (never ""), so
                // the label is always shrunk -- see MultiSelectField.tsx's
                // doc comment for why this override is needed at all.
                inputLabel: { shrink: true, sx: { top: "0px !important" } },
                select: { notched: true },
              }}
            >
              {RESOURCE_TYPES.map((rt) => (
                <MenuItem key={rt} value={rt}>
                  {rt.replace(/_/g, " ")}
                </MenuItem>
              ))}
            </TextField>
            <TextField
              select
              label="Shape"
              value={shape}
              onChange={(e) => handleShapeChange(e.target.value as BeWidgetShape)}
              size="small"
              sx={{ minWidth: 160 }}
              slotProps={{
                // `shape` always holds a real value (never ""), so the
                // label is always shrunk.
                inputLabel: { shrink: true, sx: { top: "0px !important" } },
                select: { notched: true },
              }}
            >
              {SHAPES.map((s) => (
                <MenuItem key={s} value={s}>
                  {s}
                </MenuItem>
              ))}
            </TextField>
            <Autocomplete
              freeSolo
              size="small"
              options={sectionSuggestions}
              value={section}
              onInputChange={(_e, value) => setSection(value)}
              sx={{ minWidth: 200, flex: "1 1 200px" }}
              renderInput={(params) => (
                <TextField {...params} label="Section (optional)" placeholder="Untitled group" />
              )}
            />
            <TextField
              label="Grid width (1–12)"
              type="number"
              value={gridWidth}
              onChange={(e) => setGridWidth(Math.min(12, Math.max(1, Number(e.target.value) || 1)))}
              size="small"
              sx={{ width: 160 }}
              slotProps={{ htmlInput: { min: 1, max: 12 } }}
            />
            {shape === "list" && (
              <TextField
                label="Row limit (optional)"
                type="number"
                value={listLimit ?? ""}
                onChange={(e) => {
                  const raw = e.target.value;
                  if (raw === "") {
                    setListLimit(undefined);
                    return;
                  }
                  const parsed = Number(raw);
                  // Same clamp `gridWidth` above applies, plus a guard
                  // `gridWidth` doesn't need: an invalid (non-numeric, e.g.
                  // pasted text) keystroke is ignored outright rather than
                  // written through as `NaN` — `JSON.stringify`s a `NaN` to
                  // `null` in the deployable widget JSON, silently corrupting
                  // it. Falls back to the previous valid value, not a
                  // default, since "no explicit limit" (`undefined`) is
                  // already reachable via the empty-string case above.
                  if (Number.isFinite(parsed)) setListLimit(Math.max(1, Math.trunc(parsed)));
                }}
                size="small"
                sx={{ width: 180 }}
                slotProps={{ htmlInput: { min: 1 } }}
              />
            )}
          </Box>

          <Divider />

          <Typography variant="subtitle2">Filters</Typography>
          <WidgetFilterConditionEditor
            resourceType={resourceType}
            conditions={conditions}
            onChange={setConditions}
            presets={presets}
          />

          {isListShape && (
            <>
              <Divider />
              <Typography variant="subtitle2">
                Columns — leave empty to use this resource type's default list rendering
              </Typography>
              {columnDrafts.map((column, index) => (
                <Box key={index} sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
                  <Autocomplete
                    freeSolo
                    size="small"
                    options={discoveredColumnPaths}
                    value={column.path}
                    onInputChange={(_e, value) => updateColumn(index, { path: value })}
                    sx={{ flex: "1 1 200px" }}
                    renderInput={(params) => (
                      <TextField
                        {...params}
                        label="Path"
                        placeholder="e.g. project.key"
                        helperText={
                          discoveredColumnPaths.length === 0
                            ? "Preview to see available fields"
                            : undefined
                        }
                        slotProps={{
                          htmlInput: { ...params.inputProps, "aria-label": "Column path" },
                        }}
                      />
                    )}
                  />
                  <TextField
                    label="Label"
                    value={column.label}
                    onChange={(e) => updateColumn(index, { label: e.target.value })}
                    size="small"
                    sx={{ flex: "1 1 160px" }}
                    slotProps={{ htmlInput: { "aria-label": "Column label" } }}
                  />
                  <TextField
                    select
                    label="Format"
                    value={column.format}
                    onChange={(e) =>
                      updateColumn(index, {
                        format: e.target.value as BeDashboardWidgetColumnFormat | "",
                      })
                    }
                    size="small"
                    sx={{ minWidth: 140 }}
                    slotProps={{
                      inputLabel: {
                        shrink: column.format !== "",
                        sx: { top: "0px !important" },
                      },
                      select: { notched: column.format !== "" },
                    }}
                  >
                    <MenuItem value="">Default (text)</MenuItem>
                    {COLUMN_FORMATS.map((f) => (
                      <MenuItem key={f} value={f}>
                        {f}
                      </MenuItem>
                    ))}
                  </TextField>
                  <IconButton
                    size="small"
                    aria-label={`Remove column ${column.label || index + 1}`}
                    onClick={() => removeColumn(index)}
                  >
                    <Trash2 size={16} />
                  </IconButton>
                </Box>
              ))}
              <Button
                size="small"
                variant="text"
                startIcon={<Plus size={16} />}
                onClick={addColumn}
                sx={{ alignSelf: "flex-start" }}
              >
                Add column
              </Button>
            </>
          )}

          {isChartShape && existingGroupBy !== undefined && (
            <>
              <Divider />
              <Typography variant="subtitle2">Slices</Typography>
              <Typography variant="body2" color="text.secondary">
                This widget groups its data by {existingGroupBy.field} instead of manual
                slices — the two are mutually exclusive. There's no authoring UI yet to edit
                or clear a group-by config from here (see this dialog's own doc comment); to
                switch back to manual slices, change the shape away from pie/bar and back,
                which clears it.
              </Typography>
            </>
          )}

          {isChartShape && existingGroupBy === undefined && (
            <>
              <Divider />
              <Typography variant="subtitle2">
                Slices — one search per slice, each merged under the filters above
              </Typography>
              {sliceDrafts.map((slice, index) => (
                <Box
                  key={index}
                  sx={{ border: 1, borderColor: "divider", borderRadius: 1, p: 1.5, display: "flex", flexDirection: "column", gap: 1 }}
                >
                  <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
                    <TextField
                      label="Slice label"
                      value={slice.label}
                      onChange={(e) => updateSlice(index, { label: e.target.value })}
                      size="small"
                      sx={{ flex: "1 1 200px" }}
                    />
                    <TextField
                      select
                      label="Color (optional)"
                      value={slice.color ?? ""}
                      onChange={(e) =>
                        updateSlice(index, {
                          color: (e.target.value || undefined) as BeWidgetPaletteColor | undefined,
                        })
                      }
                      size="small"
                      sx={{ minWidth: 160 }}
                      slotProps={{
                        inputLabel: {
                          shrink: (slice.color ?? "") !== "",
                          sx: { top: "0px !important" },
                        },
                        select: { notched: (slice.color ?? "") !== "" },
                      }}
                    >
                      <MenuItem value="">Default rotation</MenuItem>
                      {PALETTE_COLORS.map((c) => (
                        <MenuItem key={c} value={c}>
                          {c}
                        </MenuItem>
                      ))}
                    </TextField>
                    <IconButton
                      size="small"
                      aria-label={`Remove slice ${slice.label || index + 1}`}
                      onClick={() => removeSlice(index)}
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  </Box>
                  <WidgetFilterConditionEditor
                    resourceType={resourceType}
                    conditions={slice.conditions}
                    onChange={(next) => updateSlice(index, { conditions: next })}
                    presets={presets}
                  />
                </Box>
              ))}
              <Button size="small" variant="text" startIcon={<Plus size={16} />} onClick={addSlice} sx={{ alignSelf: "flex-start" }}>
                Add slice
              </Button>
            </>
          )}

          <Divider />

          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <Typography variant="subtitle2">Preview</Typography>
            <Button size="small" variant="outlined" startIcon={<Eye size={14} />} onClick={handlePreview}>
              Preview
            </Button>
          </Box>
          {previewSnapshot ? (
            // A list-shape widget renders a real multi-column table (same as
            // `DashboardWidgetGrid`'s own `widgetGridColumnSx`, which spans a
            // list tile the full row regardless of its configured
            // `gridWidth`) — it should preview at the dialog's actual
            // content width, not a fixed cap. Count/pie/bar tiles are
            // compact by design (a big number, a small chart); capping them
            // keeps the preview from stretching those shapes edge-to-edge,
            // which would look wrong next to how they actually render on a
            // real dashboard grid cell.
            <Box sx={previewSnapshot.shape === "list" ? { width: "100%" } : { maxWidth: 420 }}>
              <DashboardWidgetTile
                key={previewKey}
                widgetId={previewSnapshot.widgetId}
                displayName={previewSnapshot.displayName}
                description={previewSnapshot.description}
                resourceType={previewSnapshot.resourceType}
                shape={previewSnapshot.shape}
                filters={previewSnapshot.query ?? {}}
                listLimit={previewSnapshot.listLimit}
                slices={previewSnapshot.slices}
                groupBy={previewSnapshot.groupBy}
                columns={previewSnapshot.columns}
                sortBy={previewSnapshot.sortBy}
                selectedTeamCreGroupId={selectedTeamCreGroupId}
                selectedTeamSreGroupId={selectedTeamSreGroupId}
                selectedTeamLabel={selectedTeamLabel}
              />
            </Box>
          ) : (
            <Typography variant="body2" color="text.secondary">
              Click "Preview" to run this widget's current settings against real data before
              saving.
            </Typography>
          )}
        </Box>
      </DialogContent>
      <DialogActions sx={{ justifyContent: "space-between", px: 3, pb: 2 }}>
        <Box>
          {!isNew && onDelete && (
            <Button color="error" onClick={onDelete}>
              Delete widget
            </Button>
          )}
        </Box>
        <Box sx={{ display: "flex", gap: 1 }}>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="contained" disabled={!canSave} onClick={handleSave}>
            {isNew ? "Add widget" : "Save widget"}
          </Button>
        </Box>
      </DialogActions>
    </Dialog>
  );
}
