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
  Card,
  DatePickers,
  FormControl,
  IconButton,
  InputLabel,
  LinearProgress,
  MenuItem,
  Select,
  Typography,
} from "@wso2/oxygen-ui";
import { Check, Pencil, X } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type ReactNode } from "react";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import {
  useSearchInternalUsers,
  type InternalUserOption,
} from "@features/csm-projects/api/useSearchInternalUsers";
import {
  PROJECT_ONBOARDING_STATUSES,
  type ProjectOnboardingPatch,
  type ProjectOnboardingStatusValue,
} from "@features/csm-projects/api/usePatchProjectOnboarding";
import ProjectMetaCell from "@features/csm-projects/components/ProjectMetaCell";
import ProjectOnboardingOwnerValue from "@features/csm-projects/components/ProjectOnboardingOwnerValue";
import ProjectOnboardingStatusChip from "@features/csm-projects/components/ProjectOnboardingStatusChip";
import {
  buildOnboardingPatch,
  DATE_FIELDS,
  initialFormValues,
  ONBOARDING_STATUS_LABELS,
  validateOnboardingDates,
  type OnboardingDateField,
  type OnboardingFormValues,
} from "@features/csm-projects/utils/onboardingEdit";
import { isOnboardingEnabled } from "@features/csm-projects/utils/projectOnboardingEnabled";
import type { ProjectDetails } from "@features/csm-projects/types/csmProjects";
import { formatDateOnly, formatUtcDateForDisplay, parseDateOnly } from "@utils/dateTime";

const { DesktopDatePicker: DatePicker, LocalizationProvider } = DatePickers;

interface ProjectOnboardingTabProps {
  project: ProjectDetails;
  /** Show each field's edit pencil (`canEditProjectOnboarding`); the backend
   *  is the real gate. */
  canEdit: boolean;
  /** True while the PATCH is in flight; disables the open input and Save. */
  isSaving?: boolean;
  /** User-facing message for the most recent failed save, if any. */
  saveError?: string | null;
  /** Submit just the one edited field (`PATCH /projects/{id}`); `onSuccess`
   *  closes the editor. The caller owns the mutation and any error state. */
  onSave: (patch: ProjectOnboardingPatch, onSuccess: () => void) => void;
  /** Called when a field's editor opens or is cancelled, so the caller can
   *  drop a stale error from a previous attempt. */
  onResetError?: () => void;
}

/** The dates are calendar days stored at midnight UTC: show the UTC day, not
 *  the viewer's local one. */
function dateCell(value?: string | null): string {
  return formatUtcDateForDisplay(value) ?? "—";
}

/** Hours with at most two decimals and no trailing zeros ("12", "12.5"). */
function hoursCell(value?: number | null): string {
  if (value == null) return "—";
  return `${Number(value.toFixed(2))} h`;
}

const GRID_COLUMNS = {
  xs: "1fr",
  sm: "repeat(2, minmax(0, 1fr))",
  md: "repeat(3, minmax(0, 1fr))",
};

type FieldKey = "status" | "owner" | OnboardingDateField;

interface EditSession {
  field: FieldKey;
  /** Stored values when the editor opened. The live project query can refresh
   *  underneath an open editor; comparing against refreshed props would desync
   *  "changed" from the draft and could send a stale value. */
  baseline: OnboardingFormValues;
  values: OnboardingFormValues;
}

/**
 * Project detail "Onboarding" tab: onboarding status, owner and the five
 * onboarding dates. Read-only for everyone except an editor (`canEdit`), who
 * sees a pencil on each field. The pencil swaps just that field to its input in
 * place with its own save/cancel (one field at a time, no dialog), and Save
 * sends only that field. Dates are calendar days (`YYYY-MM-DD`, seeded from the
 * UTC day of the stored value, never the viewer's local day) and cannot be
 * cleared: the backend has no way to unset one, so Save stays disabled until a
 * non-empty, different value is picked. A date earlier than the start date (or
 * a start date later than a stored date) is blocked client-side; the backend
 * stays authoritative and its message is shown inline via `saveError`. The
 * owner picker lists internal, active users only and has no clear button. The
 * per-contact onboarding step ledger is not repeated here; it already lives in
 * the Project contacts tab.
 */
export default function ProjectOnboardingTab({
  project,
  canEdit,
  isSaving = false,
  saveError,
  onSave,
  onResetError,
}: ProjectOnboardingTabProps): JSX.Element {
  const { totalOnboardingHours: total, consumedOnboardingHours: consumed } = project;
  const remaining = project.remainingOnboardingHours;
  const hasHours = total != null || consumed != null || remaining != null;
  const progress =
    total != null && total > 0 ? Math.min(100, Math.max(0, ((consumed ?? 0) / total) * 100)) : null;

  const [session, setSession] = useState<EditSession | null>(null);

  const begin = (field: FieldKey): void => {
    if (isSaving) return;
    onResetError?.();
    const baseline = initialFormValues(project.onboardingStatus, project.onboardingOwner?.id, project);
    setSession({ field, baseline, values: baseline });
  };
  const cancel = (): void => {
    if (isSaving) return;
    onResetError?.();
    setSession(null);
  };
  const update = (patch: Partial<OnboardingFormValues>): void => {
    // The old server error described the previous draft value.
    onResetError?.();
    setSession((s) => (s ? { ...s, values: { ...s.values, ...patch } } : s));
  };

  const patch = session ? buildOnboardingPatch(session.values, session.baseline) : {};
  const dateErrors = session ? validateOnboardingDates(session.values, session.baseline) : {};
  const canSave = Object.keys(patch).length > 0 && Object.keys(dateErrors).length === 0;
  const editing = canEdit ? session : null;

  const actions = (): JSX.Element => (
    <Box sx={{ display: "flex", gap: 0.5 }} className="csm-print-hide">
      <IconButton
        size="small"
        color="primary"
        aria-label="Save"
        disabled={isSaving || !canSave}
        onClick={() => onSave(patch, () => setSession(null))}
      >
        <Check size={16} />
      </IconButton>
      <IconButton size="small" aria-label="Cancel" disabled={isSaving} onClick={cancel}>
        <X size={16} />
      </IconButton>
    </Box>
  );

  /** A read-only cell with this field's pencil when the caller can edit. */
  const viewCell = (field: FieldKey, label: string, value: ReactNode): JSX.Element => (
    <ProjectMetaCell key={field} label={label}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
        {value}
        {canEdit && (
          <IconButton
            size="small"
            className="csm-print-hide"
            aria-label={`Edit ${label}`}
            disabled={isSaving}
            onClick={() => begin(field)}
          >
            <Pencil size={14} />
          </IconButton>
        )}
      </Box>
    </ProjectMetaCell>
  );

  /** The open editor: the input, this field's save/cancel, and its error. */
  const editCell = (field: FieldKey, input: ReactNode, helper?: string): JSX.Element => (
    <Box key={field} sx={{ display: "flex", flexDirection: "column", gap: 0.5, minWidth: 0 }}>
      <Box sx={{ display: "flex", alignItems: "flex-start", gap: 0.5 }}>
        <Box sx={{ flex: 1, minWidth: 0 }}>{input}</Box>
        {actions()}
      </Box>
      {helper && (
        <Typography variant="caption" color="text.secondary">
          {helper}
        </Typography>
      )}
      {saveError && <Alert severity="error">{saveError}</Alert>}
    </Box>
  );

  const statusCell = (): JSX.Element => {
    if (editing?.field !== "status") {
      return viewCell(
        "status",
        "Onboarding status",
        <ProjectOnboardingStatusChip
          onboardingStatus={project.onboardingStatus}
          emptyFallback={<Typography variant="body2">—</Typography>}
        />,
      );
    }
    const { status } = editing.values;
    return editCell(
      "status",
      <FormControl size="small" fullWidth>
        <InputLabel id="project-onboarding-status-label" shrink={status !== ""} sx={{ top: "0px !important" }}>
          Onboarding status
        </InputLabel>
        <Select
          labelId="project-onboarding-status-label"
          label="Onboarding status"
          value={status}
          onChange={(e) => update({ status: e.target.value as ProjectOnboardingStatusValue })}
          disabled={isSaving}
          notched={status !== ""}
        >
          {PROJECT_ONBOARDING_STATUSES.map((s) => (
            <MenuItem key={s} value={s}>
              {ONBOARDING_STATUS_LABELS[s]}
            </MenuItem>
          ))}
        </Select>
      </FormControl>,
    );
  };

  const ownerCell = (): JSX.Element => {
    if (editing?.field !== "owner") {
      return viewCell(
        "owner",
        "Onboarding owner",
        <ProjectOnboardingOwnerValue owner={project.onboardingOwner} />,
      );
    }
    return editCell(
      "owner",
      <AsyncEntitySelect<InternalUserOption>
        id="project-onboarding-owner"
        label="Onboarding owner"
        placeholder="Search internal users…"
        value={editing.values.ownerId}
        onChange={(ownerId) => update({ ownerId })}
        disabled={isSaving}
        useSearch={useSearchInternalUsers}
        getId={(u) => u.id}
        getLabel={(u) => u.name}
        knownLabel={project.onboardingOwner?.name}
        hideClearButton
      />,
      "Only internal users can be the owner. It can't be cleared once set.",
    );
  };

  const dateCellFor = (f: (typeof DATE_FIELDS)[number]): JSX.Element => {
    if (editing?.field !== f.key) return viewCell(f.key, f.label, <Typography variant="body2">{dateCell(project[f.read])}</Typography>);
    // Moving the start past a stored date is reported on the start field, the
    // only one open, naming the stored dates it would precede.
    const clashing = DATE_FIELDS.filter((d) => dateErrors[d.key]);
    const error =
      f.key === "start"
        ? clashing.length > 0
          ? `Can't be later than the stored ${clashing.map((d) => d.label.toLowerCase()).join(", ")}.`
          : undefined
        : dateErrors[f.key];
    return editCell(
      f.key,
      <LocalizationProvider dateAdapter={AdapterDateFns}>
        <DatePicker
          label={f.label}
          value={parseDateOnly(editing.values.dates[f.key])}
          onChange={(next) =>
            update({
              dates: {
                ...editing.values.dates,
                [f.key]:
                  next instanceof Date && !Number.isNaN(next.getTime()) ? formatDateOnly(next) : "",
              },
            })
          }
          disabled={isSaving}
          slotProps={{
            textField: { fullWidth: true, size: "small", error: !!error, helperText: error },
          }}
        />
      </LocalizationProvider>,
      "A date can't be cleared once set.",
    );
  };

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="subtitle2">Onboarding</Typography>
      <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: GRID_COLUMNS }}>
        {statusCell()}
        {isOnboardingEnabled(project.onboardingStatus) && ownerCell()}
        {DATE_FIELDS.map(dateCellFor)}
      </Box>
      {hasHours && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Typography variant="subtitle2">Onboarding hours</Typography>
          <Box
            sx={{
              display: "grid",
              gap: 2,
              gridTemplateColumns: {
                xs: "1fr",
                sm: "repeat(2, minmax(0, 1fr))",
                md: "repeat(3, minmax(0, 1fr))",
              },
            }}
          >
            <ProjectMetaCell label="Total">
              <Typography variant="body2">{hoursCell(total)}</Typography>
            </ProjectMetaCell>
            <ProjectMetaCell label="Consumed">
              <Typography variant="body2">{hoursCell(consumed)}</Typography>
            </ProjectMetaCell>
            <ProjectMetaCell label="Remaining">
              <Typography variant="body2">{hoursCell(remaining)}</Typography>
            </ProjectMetaCell>
          </Box>
          {progress !== null && (
            <LinearProgress
              variant="determinate"
              value={progress}
              aria-label="Onboarding hours consumed"
              sx={{ height: 6, borderRadius: 3 }}
            />
          )}
        </Box>
      )}
    </Card>
  );
}
