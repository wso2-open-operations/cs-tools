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
  Alert,
  Box,
  Button,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import {
  formatDateTimeLocal,
  parseBackendTimestamp,
  parseDateTimeLocal,
  utcMsToZonedInputValue,
  zonedInputToBackendUtc,
} from "@utils/dateTime";

const { DateTimePicker, LocalizationProvider } = DatePickers;

interface CloseOutageDialogProps {
  /** The outage's current `begin`, raw backend timestamp — the chosen `end`
   * must not be earlier than this (the backend rejects it with a `422`, but
   * this catches the obvious case before that round trip). */
  begin: string;
  isSaving: boolean;
  onClose: () => void;
  /** `end` as a backend `YYYY-MM-DD HH:mm:ss` string. */
  onConfirm: (end: string) => void;
}

/**
 * End an outage — `PATCH { end: … }` is the only close mechanism (there is no
 * separate state field or close verb; see `BePatchOutagePayload`'s doc
 * comment).
 *
 * Named for ServiceNow's own verb: its outage form pairs "Begin Outage" with
 * ending the record, and the create page now carries the matching action.
 *
 * The end time still defaults to "now" and stays editable, for the same
 * reason the create page keeps its Begin field: an outage is routinely
 * noticed as over some minutes after it actually recovered, and stamping
 * "now" there overstates the duration on the public status page.
 */
export default function CloseOutageDialog({
  begin,
  isSaving,
  onClose,
  onConfirm,
}: CloseOutageDialogProps): JSX.Element {
  // Wall-clock in the user's timezone, like the picker's own value.
  const [end, setEnd] = useState(() => utcMsToZonedInputValue(Date.now()));
  const endDate = parseDateTimeLocal(end);
  // Compare real instants: `begin` is UTC, `end` is the user's wall-clock.
  const endUtc = zonedInputToBackendUtc(end);
  const beginInstant = parseBackendTimestamp(begin);
  const endBeforeBegin =
    !!beginInstant && !!endUtc && parseBackendTimestamp(endUtc)!.getTime() < beginInstant.getTime();
  const canConfirm = !!endDate && !!endUtc && !endBeforeBegin && !isSaving;

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>End outage</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {endBeforeBegin && (
            <Alert severity="error">End must not be before the outage's begin time.</Alert>
          )}
          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DateTimePicker
              label="End"
              value={endDate}
              onChange={(next) =>
                setEnd(
                  next instanceof Date && !Number.isNaN(next.getTime())
                    ? formatDateTimeLocal(next)
                    : "",
                )
              }
              slotProps={{ textField: { size: "small", fullWidth: true, required: true } }}
            />
          </LocalizationProvider>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="success"
          onClick={() => endUtc && onConfirm(endUtc)}
          disabled={!canConfirm}
        >
          {isSaving ? "Ending…" : "End outage"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
