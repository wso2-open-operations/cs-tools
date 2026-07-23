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
import { formatDateOnly, parseDateOnly, zonedInputToUtcIso } from "@utils/dateTime";

const { DesktopDatePicker: DatePicker, LocalizationProvider } = DatePickers;

interface SetAutocloseHoldDialogProps {
  /** Current hold-until date-time, if any (ISO), shown as the picker's initial value. */
  currentHoldUntil?: string;
  /** True while a PATCH is in flight; disables the actions. */
  isSaving: boolean;
  onClose: () => void;
  /** Apply the new hold-until date (`PATCH { autocloseHoldUntil }`), as a UTC ISO string. */
  onSave: (holdUntilIso: string) => void;
}

/**
 * Place a case on hold in the backing data source's staged auto-closure
 * sequence until a picked date, mirroring the real ServiceNow UX (see
 * `autocloseHoldUntil` on `UpdateCaseRequest`). Single-value shape, modeled on
 * {@link ChangeSeverityDialog} — a date picker instead of a radio group.
 * ServiceNow-source only; the caller surfaces a rejection on another source.
 */
export default function SetAutocloseHoldDialog({
  currentHoldUntil,
  isSaving,
  onClose,
  onSave,
}: SetAutocloseHoldDialogProps): JSX.Element {
  const [dateValue, setDateValue] = useState<string>(
    currentHoldUntil ? formatDateOnly(new Date(currentHoldUntil)) : "",
  );

  const parsed = parseDateOnly(dateValue);
  const isPast = !!parsed && parsed.getTime() < new Date().setHours(0, 0, 0, 0);
  const canSubmit = !!parsed && !isPast;

  const handleSubmit = (): void => {
    if (!canSubmit) return;
    // Hold until end-of-day local time, converted to UTC for the wire.
    const iso = zonedInputToUtcIso(`${dateValue}T23:59:00`);
    if (!iso) return;
    onSave(iso);
  };

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Hold auto-closure</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5, pt: 0.5 }}>
          <Typography variant="body2" color="text.secondary">
            Places the case on hold until the picked date. It won't be
            auto-closed until then — this is the only supported write against
            the auto-closure sequence.
          </Typography>
          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DatePicker
              label="Hold until"
              value={parseDateOnly(dateValue)}
              onChange={(next) =>
                setDateValue(
                  next instanceof Date && !Number.isNaN(next.getTime())
                    ? formatDateOnly(next)
                    : "",
                )
              }
              disabled={isSaving}
              slotProps={{
                textField: {
                  fullWidth: true,
                  required: true,
                  size: "small",
                  error: isPast,
                  helperText: isPast ? "Pick a date in the future." : undefined,
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
          Hold
        </Button>
      </DialogActions>
    </Dialog>
  );
}
