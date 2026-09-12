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
  IconButton,
  MenuItem,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type {
  BeDashboardFilterPreset,
  BeWidgetResourceType,
} from "@api/backend/types";
import {
  CASE_FIELD_OPTIONS,
  isPresetCondition,
  operatorsForResourceType,
  usesCaseFieldFilterDsl,
  type FilterCondition,
  type FilterConditionOp,
} from "@features/csm-admin/dashboards/utils/widgetQueryConditions";

const OP_LABEL: Record<FilterConditionOp, string> = {
  eq: "is",
  in: "is any of",
  notIn: "is none of",
  gte: "is on/after (≥)",
  lte: "is on/before (≤)",
  isEmpty: "is empty",
  isNotEmpty: "is not empty",
};

const NO_VALUE_OPS = new Set<FilterConditionOp>(["isEmpty", "isNotEmpty"]);

interface WidgetFilterConditionEditorProps {
  resourceType: BeWidgetResourceType;
  conditions: FilterCondition[];
  onChange: (next: FilterCondition[]) => void;
  /**
   * The shared filter presets a row may reference. Omitted/empty hides the
   * "Add preset" affordance entirely rather than offering an empty picker —
   * a deployment with no presets file configured is legitimate.
   */
  presets?: readonly BeDashboardFilterPreset[];
}

/** Human-readable rendering of what a preset expands to, for the picker's
 * helper text — an admin picking "activeCaseStates" from a list of names
 * alone has no way to know which states that actually is. */
function describePreset(preset: BeDashboardFilterPreset | undefined): string {
  if (!preset) return "";
  const { field, op, values } = preset.filter as {
    field?: unknown;
    op?: unknown;
    values?: unknown;
  };
  if (typeof field !== "string" || typeof op !== "string") {
    // A preset whose body is not the field/op/values shape this editor knows
    // is still selectable — the backend, not this list, defines what is
    // valid — it just cannot be summarized.
    return JSON.stringify(preset.filter);
  }
  const label = OP_LABEL[op as FilterConditionOp] ?? op;
  const rendered = Array.isArray(values) ? values.map(String).join(", ") : "";
  return rendered.length > 0 ? `${field} ${label} ${rendered}` : `${field} ${label}`;
}

/**
 * The widget editor's filter builder: one row per condition (field,
 * operator, value(s)), rather than a raw JSON textarea — see
 * `widgetQueryConditions.ts` for how a row round-trips through whichever of
 * the app's two real filter shapes this widget's `resourceType` actually
 * needs.
 */
export default function WidgetFilterConditionEditor({
  resourceType,
  conditions,
  onChange,
  presets,
}: WidgetFilterConditionEditorProps): JSX.Element {
  const isCaseLike = usesCaseFieldFilterDsl(resourceType);
  const fieldOptions = isCaseLike ? CASE_FIELD_OPTIONS : [];
  // Only offer operators this resourceType's own search contract can
  // actually express (see `operatorsForResourceType`'s doc comment) — a
  // non-case resourceType has no generic notIn/gte/lte/isEmpty/isNotEmpty
  // convention, so offering them here would let the admin build a filter
  // this app cannot serialize correctly.
  const availableOps = operatorsForResourceType(resourceType);

  const updateRow = (index: number, patch: Partial<FilterCondition>): void => {
    const next = conditions.map((c, i) => (i === index ? { ...c, ...patch } : c));
    onChange(next);
  };

  const removeRow = (index: number): void => {
    onChange(conditions.filter((_, i) => i !== index));
  };

  const addRow = (): void => {
    onChange([...conditions, { field: "", op: "eq", values: [] }]);
  };

  // Presets only exist inside `query.filters`, which is the case field DSL —
  // no other resourceType's search contract has that array at all, so a
  // preset row there could not be serialized (see
  // `queryFromFilterConditions`). Hidden rather than disabled: an
  // affordance that can never work for this resourceType is noise.
  // Array.isArray rather than a nullish check: this is a network-shaped
  // value, and a 200 carrying something other than an array (a contract
  // violation, or a caller passing the wrong thing) must degrade to "no
  // presets offered" rather than throw and take the whole widget editor down
  // with it. The filter rows themselves still work without a catalogue.
  const presetOptions = isCaseLike && Array.isArray(presets) ? presets : [];
  const canAddPreset = presetOptions.length > 0;
  const presetByName = new Map(presetOptions.map((p) => [p.name, p]));

  const addPresetRow = (): void => {
    onChange([...conditions, { field: "", op: "eq", values: [], preset: "" }]);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
      {conditions.length === 0 && (
        <Typography variant="body2" color="text.secondary">
          No filters — this widget matches every {resourceType.replace(/_/g, " ")} record.
        </Typography>
      )}
      {conditions.map((condition, index) => {
        // A row whose op isn't in this resourceType's own supported list
        // (only possible from data written before that restriction existed,
        // or a resourceType switch elsewhere clearing conditions
        // notwithstanding) still needs its own current value represented in
        // the Select, or MUI renders it blank — offered alongside the real
        // list rather than silently swapped out from under the admin.
        const rowOps = availableOps.includes(condition.op)
          ? availableOps
          : [...availableOps, condition.op];
        const rowSx = {
          display: "flex",
          flexWrap: "wrap",
          alignItems: "flex-start",
          gap: 1,
        } as const;

        if (isPresetCondition(condition) || condition.preset !== undefined) {
          const chosen = presetByName.get(condition.preset ?? "");
          return (
            <Box key={index} sx={rowSx}>
              <Autocomplete
                size="small"
                options={presetOptions.map((p) => p.name)}
                value={condition.preset ?? ""}
                onChange={(_e, next) => updateRow(index, { preset: next ?? "" })}
                sx={{ minWidth: 260, flex: "1 1 260px" }}
                renderInput={(params) => (
                  <TextField
                    {...params}
                    label="Shared preset"
                    // The name alone does not say what the preset filters
                    // on, and getting that wrong silently changes what the
                    // widget counts.
                    helperText={describePreset(chosen)}
                    slotProps={{
                      htmlInput: { ...params.inputProps, "aria-label": "Filter preset" },
                    }}
                  />
                )}
              />
              <Tooltip title="Remove this filter">
                <IconButton
                  size="small"
                  onClick={() => removeRow(index)}
                  aria-label="Remove filter"
                >
                  <Trash2 size={16} />
                </IconButton>
              </Tooltip>
            </Box>
          );
        }

        return (
          <Box key={index} sx={rowSx}>
            <Autocomplete
              freeSolo
              size="small"
              options={fieldOptions}
              value={condition.field}
              onInputChange={(_e, value) => updateRow(index, { field: value })}
              sx={{ minWidth: 180, flex: "1 1 180px" }}
              renderInput={(params) => (
                <TextField
                  {...params}
                  label="Field"
                  slotProps={{ htmlInput: { ...params.inputProps, "aria-label": "Filter field" } }}
                />
              )}
            />
            <TextField
              select
              size="small"
              label="Operator"
              value={condition.op}
              onChange={(e) => updateRow(index, { op: e.target.value as FilterConditionOp })}
              sx={{ minWidth: 160 }}
              slotProps={{
                // `condition.op` always holds a real value (no empty
                // option), so the label is always shrunk -- see
                // MultiSelectField.tsx's doc comment for why this override
                // is needed at all against oxygen-ui's own theme.
                inputLabel: { shrink: true, sx: { top: "0px !important" } },
                select: { notched: true },
              }}
            >
              {rowOps.map((op) => (
                <MenuItem key={op} value={op}>
                  {OP_LABEL[op]}
                </MenuItem>
              ))}
            </TextField>
            {!NO_VALUE_OPS.has(condition.op) && (
              <Autocomplete
                multiple
                freeSolo
                size="small"
                options={[]}
                value={condition.values}
                onChange={(_e, next) =>
                  updateRow(index, { values: next.map((v) => v.trim()).filter((v) => v.length > 0) })
                }
                sx={{ minWidth: 220, flex: "2 1 220px" }}
                renderInput={(params) => (
                  <TextField
                    {...params}
                    label="Value(s)"
                    placeholder={condition.values.length ? undefined : "Type a value and press Enter…"}
                    slotProps={{ htmlInput: { ...params.inputProps, "aria-label": "Filter value" } }}
                  />
                )}
              />
            )}
            <Tooltip title="Remove this filter">
              <IconButton size="small" onClick={() => removeRow(index)} aria-label="Remove filter">
                <Trash2 size={16} />
              </IconButton>
            </Tooltip>
          </Box>
        );
      })}
      <Box sx={{ display: "flex", gap: 1, alignSelf: "flex-start" }}>
        <Button
          size="small"
          variant="text"
          startIcon={<Plus size={16} />}
          onClick={addRow}
        >
          Add filter
        </Button>
        {canAddPreset && (
          // Deliberately NOT wrapped in a Tooltip: MUI's Tooltip takes over
          // the child's accessible name, so the button would stop being
          // findable as "Add preset" by assistive tech (and by its own
          // tests). The explanation lives in the caption below instead.
          <Button
            size="small"
            variant="text"
            startIcon={<Plus size={16} />}
            onClick={addPresetRow}
          >
            Add preset
          </Button>
        )}
      </Box>
      {canAddPreset && (
        <Typography variant="caption" color="text.secondary">
          A preset references a shared, named condition by name instead of
          spelling it out, so every dashboard using it changes together.
        </Typography>
      )}
    </Box>
  );
}
