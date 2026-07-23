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
  Divider,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import {
  formatDateTimeLocal,
  parseDateTimeLocal,
  resolveDisplayTimeZone,
  zonedInputToUtcIso,
} from "@utils/dateTime";

const { DateTimePicker, LocalizationProvider } = DatePickers;

/** One of the four independent fix-ETA fields this dialog can set. */
export type FixEtaField =
  | "fixEta"
  | "bestCaseFixEta"
  | "mostLikelyFixEta"
  | "worstCaseFixEta";

interface SetFixEtaDialogProps {
  /** Current customer-facing fix-commitment date/time, if any (ISO). */
  currentFixEta?: string | null;
  /** Current internal-only best-case estimate, if any (ISO). */
  currentBestCaseFixEta?: string | null;
  /** Current internal-only most-likely estimate, if any (ISO). */
  currentMostLikelyFixEta?: string | null;
  /** Current internal-only worst-case estimate, if any (ISO). */
  currentWorstCaseFixEta?: string | null;
  /** True while any of the four PATCHes is in flight; disables every field's Save. */
  isSaving: boolean;
  onClose: () => void;
  /** Apply one field's new value (`PATCH { [field]: valueIso }`), as a UTC ISO string. */
  onSave: (field: FixEtaField, valueIso: string) => void;
}

const FIELD_LABEL: Record<FixEtaField, string> = {
  fixEta: "Fix ETA",
  bestCaseFixEta: "Best case",
  mostLikelyFixEta: "Most likely",
  worstCaseFixEta: "Worst case",
};

interface FixEtaFieldRowProps {
  field: FixEtaField;
  currentValue?: string | null;
  timeZone: string;
  isSaving: boolean;
  onSave: (field: FixEtaField, valueIso: string) => void;
}

/**
 * One independently-saved date/time field. Each row owns its own draft value
 * and Save action — the four fields are unrelated PATCH variants (see
 * `BeCaseUpdatePayload`), so saving one never requires the others to be filled.
 */
function FixEtaFieldRow({
  field,
  currentValue,
  timeZone,
  isSaving,
  onSave,
}: FixEtaFieldRowProps): JSX.Element {
  const [value, setValue] = useState<string>(
    currentValue ? formatDateTimeLocal(new Date(currentValue)) : "",
  );
  const parsed = parseDateTimeLocal(value);
  const canSubmit = !!parsed;

  const handleSave = (): void => {
    if (!canSubmit) return;
    const iso = zonedInputToUtcIso(value, timeZone);
    if (!iso) return;
    onSave(field, iso);
  };

  return (
    <Box sx={{ display: "flex", gap: 1, alignItems: "flex-start" }}>
      <LocalizationProvider dateAdapter={AdapterDateFns}>
        <DateTimePicker
          label={`${FIELD_LABEL[field]} (${timeZone})`}
          value={parsed}
          onChange={(next) =>
            setValue(
              next instanceof Date && !Number.isNaN(next.getTime())
                ? formatDateTimeLocal(next)
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
 * Set the case's four independent fix-ETA fields: the single customer-facing
 * commitment (`fixEta` on `PATCH /cases/{id}`) plus three internal-only
 * estimates (`bestCaseFixEta` / `mostLikelyFixEta` / `worstCaseFixEta`) never
 * shared with the customer. Each field is its own single-field PATCH variant
 * (see `BeCaseUpdatePayload`), so every row saves independently — filling in
 * one estimate never requires the others. ServiceNow-source only for
 * `fixEta`; the caller surfaces a rejection on another source.
 */
export default function SetFixEtaDialog({
  currentFixEta,
  currentBestCaseFixEta,
  currentMostLikelyFixEta,
  currentWorstCaseFixEta,
  isSaving,
  onClose,
  onSave,
}: SetFixEtaDialogProps): JSX.Element {
  const timeZone = resolveDisplayTimeZone();

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Set fix ETA</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          <Typography variant="body2" color="text.secondary">
            Each field below is saved independently — you don't need to fill
            in every estimate to save one.
          </Typography>

          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75 }}>
            <Typography variant="caption" color="text.secondary">
              Customer-facing fix-commitment date/time. Shared with the
              customer — distinct from the backend-computed SLA clocks shown
              on the SLAs tab.
            </Typography>
            <FixEtaFieldRow
              field="fixEta"
              currentValue={currentFixEta}
              timeZone={timeZone}
              isSaving={isSaving}
              onSave={onSave}
            />
          </Box>

          <Divider />

          <Typography variant="subtitle2">Internal-only estimates</Typography>
          <Typography variant="caption" color="text.secondary" sx={{ mt: -1.5 }}>
            Never shared with the customer.
          </Typography>

          <FixEtaFieldRow
            field="bestCaseFixEta"
            currentValue={currentBestCaseFixEta}
            timeZone={timeZone}
            isSaving={isSaving}
            onSave={onSave}
          />
          <FixEtaFieldRow
            field="mostLikelyFixEta"
            currentValue={currentMostLikelyFixEta}
            timeZone={timeZone}
            isSaving={isSaving}
            onSave={onSave}
          />
          <FixEtaFieldRow
            field="worstCaseFixEta"
            currentValue={currentWorstCaseFixEta}
            timeZone={timeZone}
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
