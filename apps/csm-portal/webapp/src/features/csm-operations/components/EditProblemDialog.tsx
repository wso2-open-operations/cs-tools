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
  Typography,
  TextField,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useSearchGroups } from "@api/useSearchGroups";
import { useSearchUsersByName } from "@api/useSearchUsersByName";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import { userLabel } from "@features/csm-operations/utils/incidentFormOptions";
import { formatDateTimeLocal, parseDateTimeLocal } from "@utils/dateTime";
import type { BeGroup, BeProblemDetail, BeUpdateProblemPayload, BeUser } from "@api/backend/types";

const { DateTimePicker, LocalizationProvider } = DatePickers;

interface EditProblemDialogProps {
  problem: BeProblemDetail;
  /** True while the PATCH is in flight; disables the actions. */
  isSaving: boolean;
  /** User-facing message for the most recent failed save, if any. */
  saveError?: string | null;
  onClose: () => void;
  /** Submit only the changed/filled-in fields (`PATCH /problems/{id}`). */
  onSave: (patch: BeUpdateProblemPayload) => void;
}

/** Convert a backend timestamp (`YYYY-MM-DD HH:MM:SS`) to `YYYY-MM-DDTHH:MM`. */
function toDateTimeLocal(raw?: string | null): string {
  const m = /^(\d{4}-\d{2}-\d{2})[ T](\d{2}:\d{2})/.exec(raw?.trim() ?? "");
  return m ? `${m[1]}T${m[2]}` : "";
}

/** Convert a `datetime-local` value back to the BE's `YYYY-MM-DD HH:MM:SS`. */
function toBackendDateTime(local: string): string {
  return `${local.replace("T", " ")}:00`;
}

/**
 * Assignment/tracking edit dialog for a problem: `assignedToId`,
 * `assignmentGroupId`, `workaround`, and `targetResolutionDate`. Deliberately
 * narrower than `EditIncidentDialog`/`EditChangeRequestDialog` — no
 * classification fields (category/subcategory/priority aren't
 * Update-writable per `CHANGES-problem-update.md` §3) and no state field
 * (state moves only through `ProblemActionBar`'s modeled forward
 * transitions, never an arbitrary jump here).
 *
 * `assignmentGroupId` and `targetResolutionDate` start blank on every open,
 * not prefilled from `problem` — `GET /problems/{id}` doesn't return either
 * field today (confirmed only via a direct ServiceNow table read-back, not
 * through the Problem-detail response shape; see `BeProblemDetail`'s own
 * scope). Leaving a value in place across dialog opens would risk a false
 * "already set to X" impression the portal can't actually verify.
 */
export default function EditProblemDialog({
  problem,
  isSaving,
  saveError,
  onClose,
  onSave,
}: EditProblemDialogProps): JSX.Element {
  const [assignedToId, setAssignedToId] = useState(problem.assignedTo?.id ?? "");
  const [assignmentGroupId, setAssignmentGroupId] = useState("");
  const [workaround, setWorkaround] = useState(problem.workaround ?? "");
  const [targetResolutionDate, setTargetResolutionDate] = useState("");

  const initialAssignedToId = problem.assignedTo?.id ?? "";
  const initialWorkaround = problem.workaround ?? "";

  const dateValue = useMemo(() => parseDateTimeLocal(toDateTimeLocal(targetResolutionDate)), [
    targetResolutionDate,
  ]);

  const patch = useMemo(() => {
    const next: BeUpdateProblemPayload = {};
    if (assignedToId !== initialAssignedToId) next.assignedToId = assignedToId || null;
    if (assignmentGroupId) next.assignmentGroupId = assignmentGroupId;
    if (workaround !== initialWorkaround) next.workaround = workaround;
    if (targetResolutionDate) next.targetResolutionDate = targetResolutionDate;
    return next;
  }, [assignedToId, initialAssignedToId, assignmentGroupId, workaround, initialWorkaround, targetResolutionDate]);

  const hasChanges = Object.keys(patch).length > 0;

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Edit problem</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {saveError && <Alert severity="error">{saveError}</Alert>}

          <Typography variant="subtitle2">Assignment</Typography>
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ flex: "1 1 220px" }}>
              <AsyncEntitySelect<BeUser>
                id="edit-problem-assigned-to"
                label="Assigned to"
                placeholder="Search people…"
                value={assignedToId}
                onChange={setAssignedToId}
                disabled={isSaving}
                useSearch={useSearchUsersByName}
                getId={(u) => u.id!}
                getLabel={userLabel}
                knownLabel={problem.assignedTo?.name}
                helperText="Setting this on a New problem with no existing owner moves it to Assess automatically (a ServiceNow business rule)."
              />
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              <AsyncEntitySelect<BeGroup>
                id="edit-problem-assignment-group"
                label="Assignment group"
                placeholder="Search groups…"
                value={assignmentGroupId}
                onChange={setAssignmentGroupId}
                disabled={isSaving}
                useSearch={useSearchGroups}
                getId={(g) => g.id}
                getLabel={(g) => g.name}
                helperText="Not shown pre-filled — the portal can't yet read a problem's current assignment group back."
              />
            </Box>
          </Box>

          <Typography variant="subtitle2">Tracking</Typography>
          <TextField
            label="Workaround"
            value={workaround}
            onChange={(e) => setWorkaround(e.target.value)}
            disabled={isSaving}
            multiline
            minRows={2}
            fullWidth
          />
          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DateTimePicker
              label="Target resolution date"
              value={dateValue}
              onChange={(next) =>
                setTargetResolutionDate(
                  next instanceof Date && !Number.isNaN(next.getTime())
                    ? toBackendDateTime(formatDateTimeLocal(next))
                    : "",
                )
              }
              slotProps={{
                textField: {
                  size: "small",
                  fullWidth: true,
                  helperText:
                    "Not on the native ServiceNow Problem form — this is a generic due-date column exposed here for internal tracking only. Not shown pre-filled for the same reason as Assignment group.",
                },
              }}
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
          onClick={() => onSave(patch)}
          disabled={isSaving || !hasChanges}
        >
          {isSaving ? "Saving…" : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
