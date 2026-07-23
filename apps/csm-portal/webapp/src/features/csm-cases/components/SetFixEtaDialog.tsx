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
import {
  formatDateTimeLocal,
  parseDateTimeLocal,
  resolveDisplayTimeZone,
  zonedInputToUtcIso,
} from "@utils/dateTime";

const { DateTimePicker, LocalizationProvider } = DatePickers;

interface SetFixEtaDialogProps {
  /** Current fix-ETA, if any (ISO), shown as the picker's initial value. */
  currentFixEta?: string | null;
  /** True while a PATCH is in flight; disables the actions. */
  isSaving: boolean;
  onClose: () => void;
  /** Apply the new fix-ETA (`PATCH { fixEta }`), as a UTC ISO string. */
  onSave: (fixEtaIso: string) => void;
}

/**
 * Set the case's single customer-facing fix-commitment date/time
 * (`fixEta` on `PATCH /cases/{id}`). Single-value date/time shape, modeled on
 * {@link SetAutocloseHoldDialog} but with a date **and** time component (like
 * {@link ScheduleCallDialog}'s custom-time picker) since a fix commitment is
 * a specific moment, not an end-of-day date. ServiceNow-source only; the
 * caller surfaces a rejection on another source.
 */
export default function SetFixEtaDialog({
  currentFixEta,
  isSaving,
  onClose,
  onSave,
}: SetFixEtaDialogProps): JSX.Element {
  const timeZone = resolveDisplayTimeZone();
  const [value, setValue] = useState<string>(
    currentFixEta ? formatDateTimeLocal(new Date(currentFixEta)) : "",
  );

  const parsed = parseDateTimeLocal(value);
  const canSubmit = !!parsed;

  const handleSubmit = (): void => {
    if (!canSubmit) return;
    const iso = zonedInputToUtcIso(value, timeZone);
    if (!iso) return;
    onSave(iso);
  };

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Set fix ETA</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5, pt: 0.5 }}>
          <Typography variant="body2" color="text.secondary">
            Sets the customer-facing fix-commitment date/time for this case.
            This is the only fix-ETA field the platform exposes — it is
            distinct from the backend-computed SLA clocks shown on the SLAs
            tab.
          </Typography>
          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DateTimePicker
              label={`Fix ETA (${timeZone})`}
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
                textField: {
                  fullWidth: true,
                  required: true,
                  size: "small",
                },
                field: { clearable: true },
              }}
            />
          </LocalizationProvider>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!canSubmit || isSaving}
          loading={isSaving}
          onClick={handleSubmit}
        >
          Save
        </Button>
      </DialogActions>
    </Dialog>
  );
}
