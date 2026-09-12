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
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import WidgetFilterConditionEditor from "@features/csm-admin/dashboards/components/WidgetFilterConditionEditor";
import {
  filterConditionsFromQuery,
  queryFromFilterConditions,
  type FilterCondition,
} from "@features/csm-admin/dashboards/utils/widgetQueryConditions";
import type { PresetDraft } from "@features/csm-admin/dashboards/utils/sharedConfigDraftsStorage";

interface PresetEditorDialogProps {
  /** `undefined` when designing a brand-new preset. */
  preset?: PresetDraft;
  /** Names already taken, so a rename cannot silently overwrite another
   * preset. Excludes the one being edited. */
  takenNames: readonly string[];
  onClose: () => void;
  onSave: (preset: PresetDraft) => void;
}

/**
 * Designs one shared filter preset: a name plus the single filter predicate
 * it stands for.
 *
 * Exactly one predicate, not a list: a preset expands to ONE entry in a
 * widget's `query.filters` (see the backend's own `LoadSharedPresets`, whose
 * value type is a single filter object). Letting an admin build two rows here
 * would produce a file the loader rejects, so the second row is refused with
 * a reason rather than accepted and then failed on deploy.
 *
 * The condition row itself is the same editor a widget's filters use — the
 * predicate shape is identical, so there is no second filter UI to keep in
 * step. Presets are not offered inside it: a preset referencing another
 * preset is rejected by the loader (`validatePresetsNotRecursive`).
 */
export default function PresetEditorDialog({
  preset,
  takenNames,
  onClose,
  onSave,
}: PresetEditorDialogProps): JSX.Element {
  const [name, setName] = useState(preset?.name ?? "");
  const [conditions, setConditions] = useState<FilterCondition[]>(() =>
    // A preset's body is a single case-DSL predicate, so it is read through
    // the case reader by wrapping it in the `filters` array that reader
    // expects — the preset file stores the bare predicate, not the wrapper.
    preset ? filterConditionsFromQuery("case", { filters: [preset.filter] }) : [],
  );

  const trimmedName = name.trim();
  const duplicate = takenNames.some((n) => n === trimmedName);
  const tooManyConditions = conditions.length > 1;
  const canSave =
    trimmedName.length > 0 && !duplicate && conditions.length === 1 && !tooManyConditions;

  const handleSave = (): void => {
    const query = queryFromFilterConditions("case", conditions);
    const filters = query.filters;
    if (!Array.isArray(filters) || filters.length !== 1) return;
    onSave({ name: trimmedName, filter: filters[0] as Record<string, unknown> });
  };

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{preset ? "Edit preset" : "New preset"}</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
          <TextField
            size="small"
            label="Preset name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            error={duplicate}
            helperText={
              duplicate
                ? "Another preset already uses this name."
                : "The name a dashboard definition references this preset by, e.g. activeCaseStates."
            }
            slotProps={{ htmlInput: { "aria-label": "Preset name" } }}
          />
          <Box>
            <Typography variant="subtitle2" gutterBottom>
              Condition
            </Typography>
            <WidgetFilterConditionEditor
              resourceType="case"
              conditions={conditions}
              onChange={setConditions}
            />
          </Box>
          {tooManyConditions && (
            <Alert severity="error">
              A preset stands for exactly one condition. Remove the extra rows,
              or make two presets — the config loader rejects a preset whose
              body is a list.
            </Alert>
          )}
          {conditions.length === 0 && (
            <Alert severity="info">
              Add the one condition this preset should stand for.
            </Alert>
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={!canSave} onClick={handleSave}>
          Save preset
        </Button>
      </DialogActions>
    </Dialog>
  );
}
