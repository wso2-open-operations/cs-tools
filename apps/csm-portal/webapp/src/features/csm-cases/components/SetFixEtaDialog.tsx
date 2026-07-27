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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import { formatDateOnly, parseDateOnly } from "@utils/dateTime";

const { DesktopDatePicker: DatePicker, LocalizationProvider } = DatePickers;

/** One of the three independent internal-only fix-ETA estimate fields this dialog can set. */
export type FixEtaField = "bestCaseFixEta" | "mostLikelyFixEta" | "worstCaseFixEta";

interface SetFixEtaDialogProps {
  /** Current internal-only best-case estimate, if any (date-only "YYYY-MM-DD"). */
  currentBestCaseFixEta?: string | null;
  /** Current internal-only most-likely estimate, if any (date-only "YYYY-MM-DD"). */
  currentMostLikelyFixEta?: string | null;
  /** Current internal-only worst-case estimate, if any (date-only "YYYY-MM-DD"). */
  currentWorstCaseFixEta?: string | null;
  /** True while any of the three PATCHes is in flight; disables every field's Save. */
  isSaving: boolean;
  onClose: () => void;
  /** Apply one field's new value (`PATCH { [field]: "YYYY-MM-DD" }`). */
  onSave: (field: FixEtaField, valueDateOnly: string) => void;
}

const FIELD_LABEL: Record<FixEtaField, string> = {
  bestCaseFixEta: "Best case",
  mostLikelyFixEta: "Most likely",
  worstCaseFixEta: "Worst case",
};

interface FixEtaFieldRowProps {
  field: FixEtaField;
  currentValue?: string | null;
  isSaving: boolean;
  onSave: (field: FixEtaField, valueDateOnly: string) => void;
}

/**
 * One independently-saved date field. Each row owns its own draft value and
 * Save action — the three fields are unrelated PATCH variants (see
 * `BeCaseUpdatePayload`), so saving one never requires the others to be
 * filled. Date-only, no time-of-day component, matching this codebase's
 * `SetAutocloseHoldDialog` picker convention.
 */
function FixEtaFieldRow({
  field,
  currentValue,
  isSaving,
  onSave,
}: FixEtaFieldRowProps): JSX.Element {
  const [value, setValue] = useState<string>(currentValue ?? "");
  const parsed = parseDateOnly(value);
  const canSubmit = !!parsed;

  const handleSave = (): void => {
    if (!canSubmit || !value) return;
    onSave(field, value);
  };

  return (
    <Box sx={{ display: "flex", gap: 1, alignItems: "flex-start" }}>
      <LocalizationProvider dateAdapter={AdapterDateFns}>
        <DatePicker
          label={FIELD_LABEL[field]}
          value={parsed}
          onChange={(next) =>
            setValue(
              next instanceof Date && !Number.isNaN(next.getTime())
                ? formatDateOnly(next)
                : "",
            )
          }
          disabled={isSaving}
          slotProps={{
            textField: { fullWidth: true, size: "small" },
            field: { clearable: true },
          }}
        />
      </LocalizationProvider>
      <Button
        variant="outlined"
        size="small"
        disabled={!canSubmit || isSaving}
        loading={isSaving}
        onClick={handleSave}
        sx={{ flexShrink: 0, mt: 0.25 }}
      >
        Save
      </Button>
    </Box>
  );
}

/**
 * Set the case's three independent internal-only fix-ETA estimates
 * (`bestCaseFixEta` / `mostLikelyFixEta` / `worstCaseFixEta`), never shared
 * with the customer. Each field is its own single-field, date-only PATCH
 * variant (see `BeCaseUpdatePayload`), so every row saves independently —
 * filling in one estimate never requires the others. ServiceNow-source only;
 * the caller surfaces a rejection on another source.
 */
export default function SetFixEtaDialog({
  currentBestCaseFixEta,
  currentMostLikelyFixEta,
  currentWorstCaseFixEta,
  isSaving,
  onClose,
  onSave,
}: SetFixEtaDialogProps): JSX.Element {
  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Set fix ETA</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          <Typography variant="body2" color="text.secondary">
            Internal-only estimates, never shared with the customer. Each
            field below is saved independently — you don't need to fill in
            every estimate to save one.
          </Typography>

          <FixEtaFieldRow
            field="bestCaseFixEta"
            currentValue={currentBestCaseFixEta}
            isSaving={isSaving}
            onSave={onSave}
          />
          <FixEtaFieldRow
            field="mostLikelyFixEta"
            currentValue={currentMostLikelyFixEta}
            isSaving={isSaving}
            onSave={onSave}
          />
          <FixEtaFieldRow
            field="worstCaseFixEta"
            currentValue={currentWorstCaseFixEta}
            isSaving={isSaving}
            onSave={onSave}
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSaving}>
          Close
        </Button>
      </DialogActions>
    </Dialog>
  );
}
