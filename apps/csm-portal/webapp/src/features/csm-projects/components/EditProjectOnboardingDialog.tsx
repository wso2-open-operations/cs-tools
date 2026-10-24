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
  AdapterDateFns,
  Box,
  Button,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { formatDateOnly, parseDateOnly, utcDateOnlyValue } from "@utils/dateTime";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import {
  PROJECT_ONBOARDING_STATUSES,
  type ProjectOnboardingPatch,
  type ProjectOnboardingStatusValue,
} from "@features/csm-projects/api/usePatchProjectOnboarding";
import {
  useSearchInternalUsers,
  type InternalUserOption,
} from "@features/csm-projects/api/useSearchInternalUsers";

const { DesktopDatePicker: DatePicker, LocalizationProvider } = DatePickers;

const ONBOARDING_STATUS_LABELS: Record<ProjectOnboardingStatusValue, string> = {
  "Not-Started": "Not Started",
  "In-Progress": "In Progress",
  "On-Hold": "On Hold",
  Completed: "Completed",
  Cancelled: "Cancelled",
  Expired: "Expired",
  "Not-Applicable": "Not Applicable",
};

/** The backend matches ignoring case, hyphens, underscores and spaces, and an
 *  older source spells On Hold `OnHold`; normalize the same way so the picker
 *  opens on the project's real current value. `""` when absent or unknown. */
function matchCurrentStatus(current?: string | null): ProjectOnboardingStatusValue | "" {
  const norm = (v: string): string => v.toLowerCase().replace(/[-_\s]/g, "");
  const target = current ? norm(current) : "";
  return PROJECT_ONBOARDING_STATUSES.find((s) => norm(s) === target) ?? "";
}

/** Current onboarding dates as the read API returns them (midnight-UTC
 *  timestamps, or null). Read names differ from the PATCH names for plan and
 *  go-live; the dialog maps between them. */
export interface ProjectOnboardingDates {
  onboardingStartDate?: string | null;
  goLivePlanDate?: string | null;
  goLiveDate?: string | null;
  onboardingExpiryDate?: string | null;
}

type DateField = "start" | "plan" | "goLive" | "expiry";

const DATE_FIELDS: ReadonlyArray<{
  key: DateField;
  label: string;
  read: keyof ProjectOnboardingDates;
  write: keyof ProjectOnboardingPatch;
}> = [
  { key: "start", label: "Onboarding start date", read: "onboardingStartDate", write: "onboardingStartDate" },
  { key: "plan", label: "Go-live plan date", read: "goLivePlanDate", write: "onboardingGoLivePlanDate" },
  { key: "goLive", label: "Go-live date", read: "goLiveDate", write: "onboardingGoLiveDate" },
  { key: "expiry", label: "Onboarding expiry date", read: "onboardingExpiryDate", write: "onboardingExpiryDate" },
];

interface EditProjectOnboardingDialogProps {
  /** The project's current `onboardingStatus`, if any. */
  currentStatus?: string | null;
  /** The project's current onboarding owner, if any. */
  currentOwner?: { id: string; name: string } | null;
  /** The project's current onboarding dates (read-API shape). */
  currentDates?: ProjectOnboardingDates;
  /** True while the PATCH is in flight; disables the pickers/actions. */
  isSaving: boolean;
  /** User-facing message for the most recent failed save, if any. */
  saveError?: string | null;
  onClose: () => void;
  /** Submit only the changed field(s) (`PATCH /projects/{id}`). */
  onSave: (patch: ProjectOnboardingPatch) => void;
}

/**
 * Edit a project's onboarding status, owner and four onboarding dates
 * independently -- any may be changed on its own, and only changed fields are
 * submitted. Dates are calendar days (`YYYY-MM-DD`, seeded from the UTC day of
 * the stored value, never the viewer's local day) and cannot be cleared: the
 * backend has no way to unset one, so an emptied date is treated as unchanged.
 * Plan/go-live/expiry earlier than the start date are blocked client-side; the
 * backend stays authoritative. The owner picker lists internal, active
 * users only (see `useSearchInternalUsers`). There is no way to clear the
 * owner back to "none": the backend does not support it, so an emptied owner
 * is never submitted. Mirrors `EditProjectClosureStateDialog`/
 * `EditAccountTeamsDialog`: a plain controlled dialog whose caller owns the
 * mutation and passes `isSaving`/`saveError` down.
 */
export default function EditProjectOnboardingDialog({
  currentStatus,
  currentOwner,
  currentDates,
  isSaving,
  saveError,
  onClose,
  onSave,
}: EditProjectOnboardingDialogProps): JSX.Element {
  // Baseline captured once per open session (the caller mounts this dialog
  // only while open). The live project query can refresh underneath an open
  // dialog; comparing against refreshed props would desync "changed" from the
  // editable state and could send a stale value.
  const [baseline] = useState(() => ({
    status: matchCurrentStatus(currentStatus),
    ownerId: currentOwner?.id ?? "",
    dates: {
      start: utcDateOnlyValue(currentDates?.onboardingStartDate) ?? "",
      plan: utcDateOnlyValue(currentDates?.goLivePlanDate) ?? "",
      goLive: utcDateOnlyValue(currentDates?.goLiveDate) ?? "",
      expiry: utcDateOnlyValue(currentDates?.onboardingExpiryDate) ?? "",
    } as Record<DateField, string>,
  }));
  const {
    status: initialStatus,
    ownerId: initialOwnerId,
    dates: initialDates,
  } = baseline;
  const [status, setStatus] = useState<ProjectOnboardingStatusValue | "">(initialStatus);
  const [ownerId, setOwnerId] = useState(initialOwnerId);
  const [dates, setDates] = useState<Record<DateField, string>>(initialDates);

  // A date counts as changed only when non-empty and different from the stored
  // value; an emptied picker means "keep", since a date can't be cleared.
  const isDateChanged = (k: DateField): boolean =>
    dates[k] !== "" && dates[k] !== initialDates[k];

  const patch = useMemo<ProjectOnboardingPatch>(() => {
    const next: ProjectOnboardingPatch = {};
    if (status && status !== initialStatus) next.onboardingStatus = status;
    if (ownerId && ownerId !== initialOwnerId) next.onboardingOwner = ownerId;
    for (const f of DATE_FIELDS) {
      if (dates[f.key] !== "" && dates[f.key] !== initialDates[f.key]) {
        (next as Record<string, string>)[f.write] = dates[f.key];
      }
    }
    return next;
  }, [status, initialStatus, ownerId, initialOwnerId, dates, initialDates]);

  // The start date the backend will compare against: the one being sent, else
  // the stored one. A later date is checked when it, or the start date, is
  // being changed, against its effective value (the edit, else the stored
  // one), so moving the start past an untouched stored date is caught here
  // rather than rejected by the backend. Unrelated edits are never blocked by
  // stored data that already violates the order. ISO dates compare correctly
  // as strings.
  const effectiveStart = dates.start || initialDates.start;
  const startChanged = isDateChanged("start");
  const dateErrors: Partial<Record<DateField, string>> = {};
  if (effectiveStart) {
    for (const k of ["plan", "goLive", "expiry"] as const) {
      const changed = isDateChanged(k);
      if (!changed && !startChanged) continue;
      const effective = changed ? dates[k] : initialDates[k];
      if (effective && effective < effectiveStart) {
        dateErrors[k] = changed
          ? "Can't be earlier than the onboarding start date."
          : `The stored date (${effective}) is earlier than the new onboarding start date.`;
      }
    }
  }
  const hasDateError = Object.keys(dateErrors).length > 0;

  const hasChanges = Object.keys(patch).length > 0;

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Edit onboarding</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {saveError && <Alert severity="error">{saveError}</Alert>}

          <FormControl size="small" fullWidth>
            <InputLabel
              id="edit-project-onboarding-status-label"
              shrink={status !== ""}
              sx={{ top: "0px !important" }}
            >
              Onboarding status
            </InputLabel>
            <Select
              labelId="edit-project-onboarding-status-label"
              label="Onboarding status"
              value={status}
              onChange={(e) => setStatus(e.target.value as ProjectOnboardingStatusValue)}
              disabled={isSaving}
              notched={status !== ""}
            >
              {PROJECT_ONBOARDING_STATUSES.map((s) => (
                <MenuItem key={s} value={s}>
                  {ONBOARDING_STATUS_LABELS[s]}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          <AsyncEntitySelect<InternalUserOption>
            id="edit-project-onboarding-owner"
            label="Onboarding owner"
            placeholder="Search internal users…"
            value={ownerId}
            onChange={setOwnerId}
            disabled={isSaving}
            useSearch={useSearchInternalUsers}
            getId={(u) => u.id}
            getLabel={(u) => u.name}
            knownLabel={currentOwner?.name}
            hideClearButton
          />

          <LocalizationProvider dateAdapter={AdapterDateFns}>
            {DATE_FIELDS.map((f) => (
              <DatePicker
                key={f.key}
                label={f.label}
                value={parseDateOnly(dates[f.key])}
                onChange={(next) =>
                  setDates((d) => ({
                    ...d,
                    [f.key]:
                      next instanceof Date && !Number.isNaN(next.getTime())
                        ? formatDateOnly(next)
                        : "",
                  }))
                }
                disabled={isSaving}
                slotProps={{
                  textField: {
                    fullWidth: true,
                    size: "small",
                    error: !!dateErrors[f.key],
                    helperText: dateErrors[f.key],
                  },
                }}
              />
            ))}
          </LocalizationProvider>

          <Typography variant="caption" color="text.secondary">
            Only internal users can be the owner. The owner and the dates can't
            be cleared once set, so an emptied field is left unchanged and won't
            be saved.
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
          disabled={isSaving || !hasChanges || hasDateError}
        >
          {isSaving ? "Saving…" : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
