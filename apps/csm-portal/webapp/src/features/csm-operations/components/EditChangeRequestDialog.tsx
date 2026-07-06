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
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Switch,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import type {
  BeChangeRequestDetail,
  BePatchChangeRequestPayload,
} from "@api/backend/types";

interface EditChangeRequestDialogProps {
  cr: BeChangeRequestDetail;
  /** True while the PATCH is in flight; disables the actions. */
  isSaving: boolean;
  onClose: () => void;
  /** Submit only the changed fields (`PATCH /change-requests/{id}`). */
  onSave: (patch: BePatchChangeRequestPayload) => void;
}

/**
 * Convert a backend timestamp (`YYYY-MM-DD HH:MM:SS`, or ISO `T`-separated) to
 * the `YYYY-MM-DDTHH:MM` shape an `<input type="datetime-local">` expects. The
 * value is treated as plain wall-clock text so no timezone shift is applied.
 */
function toDateTimeLocal(raw?: string | null): string {
  const m = /^(\d{4}-\d{2}-\d{2})[ T](\d{2}:\d{2})/.exec(raw?.trim() ?? "");
  return m ? `${m[1]}T${m[2]}` : "";
}

/** Convert a `datetime-local` value back to the BE's `YYYY-MM-DD HH:MM:SS`. */
function toBackendDateTime(local: string): string {
  return `${local.replace("T", " ")}:00`;
}

/**
 * Edit the change-request fields the BE allows updating: planned start, and the
 * customer approved / reviewed flags. Only changed fields are sent, and the BE
 * requires at least one, so Save is disabled until something differs.
 */
export default function EditChangeRequestDialog({
  cr,
  isSaving,
  onClose,
  onSave,
}: EditChangeRequestDialogProps): JSX.Element {
  const initialPlannedStart = useMemo(
    () => toDateTimeLocal(cr.plannedStartOn),
    [cr.plannedStartOn],
  );
  const [plannedStart, setPlannedStart] = useState(initialPlannedStart);
  const [approved, setApproved] = useState(!!cr.hasCustomerApproved);
  const [reviewed, setReviewed] = useState(!!cr.hasCustomerReviewed);

  const patch = useMemo<BePatchChangeRequestPayload>(() => {
    const next: BePatchChangeRequestPayload = {};
    if (plannedStart !== initialPlannedStart && plannedStart) {
      next.plannedStartOn = toBackendDateTime(plannedStart);
    }
    if (approved !== !!cr.hasCustomerApproved) next.isCustomerApproved = approved;
    if (reviewed !== !!cr.hasCustomerReviewed) next.isCustomerReviewed = reviewed;
    return next;
  }, [
    plannedStart,
    initialPlannedStart,
    approved,
    reviewed,
    cr.hasCustomerApproved,
    cr.hasCustomerReviewed,
  ]);

  const hasChanges = Object.keys(patch).length > 0;

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Edit change request</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          <TextField
            label="Planned start"
            type="datetime-local"
            size="small"
            fullWidth
            value={plannedStart}
            onChange={(e) => setPlannedStart(e.target.value)}
            InputLabelProps={{ shrink: true }}
          />
          <FormControlLabel
            control={
              <Switch
                checked={approved}
                onChange={(e) => setApproved(e.target.checked)}
              />
            }
            label="Customer approved"
          />
          <FormControlLabel
            control={
              <Switch
                checked={reviewed}
                onChange={(e) => setReviewed(e.target.checked)}
              />
            }
            label="Customer reviewed"
          />
          <Typography variant="caption" color="text.secondary">
            Edits apply to ServiceNow-managed change requests.
          </Typography>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={() => onSave(patch)}
          disabled={isSaving || !hasChanges}
        >
          {isSaving ? "Saving…" : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
