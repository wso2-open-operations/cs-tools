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
  Card,
  DatePickers,
  FormControl,
  InputLabel,
  LinearProgress,
  MenuItem,
  Select,
  Typography,
} from "@wso2/oxygen-ui";
import { Pencil } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
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
  type OnboardingFormValues,
} from "@features/csm-projects/utils/onboardingEdit";
import { isOnboardingEnabled } from "@features/csm-projects/utils/projectOnboardingEnabled";
import type { ProjectDetails } from "@features/csm-projects/types/csmProjects";
import { formatDateOnly, formatUtcDateForDisplay, parseDateOnly } from "@utils/dateTime";

const { DesktopDatePicker: DatePicker, LocalizationProvider } = DatePickers;

interface ProjectOnboardingTabProps {
  project: ProjectDetails;
  /** Show the Edit button (`canEditProjectOnboarding`); the backend is the
   *  real gate. */
  canEdit: boolean;
  /** True while the PATCH is in flight; disables the inputs and Save. */
  isSaving?: boolean;
  /** User-facing message for the most recent failed save, if any. */
  saveError?: string | null;
  /** Open in edit mode (when `canEdit`), e.g. from the overview's pencil. */
  initialEditing?: boolean;
  /** Submit only the changed field(s) (`PATCH /projects/{id}`); `onSuccess`
   *  leaves edit mode. The caller owns the mutation and any error state. */
  onSave: (patch: ProjectOnboardingPatch, onSuccess: () => void) => void;
  /** Called when an edit session starts or is cancelled, so the caller can
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

interface EditSession {
  /** Stored values when the session started. The live project query can
   *  refresh underneath an open session; comparing against refreshed props
   *  would desync "changed" from the editable state and could send a stale
   *  value. */
  baseline: OnboardingFormValues;
  values: OnboardingFormValues;
}

/**
 * Project detail "Onboarding" tab: onboarding status, owner and the five
 * onboarding dates. Read-only until an editor (`canEdit`) clicks Edit, which
 * turns every field into its input in place with one Save/Cancel pair (the
 * same section-level toggle `ProductTab` uses), no dialog. Only changed fields
 * are submitted. Dates are calendar days (`YYYY-MM-DD`, seeded from the UTC day
 * of the stored value, never the viewer's local day) and cannot be cleared: the
 * backend has no way to unset one, so an emptied date is treated as unchanged.
 * Plan/go-live/expiry/readiness earlier than the start date are blocked
 * client-side; the backend stays authoritative and its message is shown via
 * `saveError`. The owner picker lists internal, active users only and has no
 * clear button. The per-contact onboarding step ledger is not repeated here;
 * it already lives in the Project contacts tab.
 */
export default function ProjectOnboardingTab({
  project,
  canEdit,
  isSaving = false,
  saveError,
  initialEditing = false,
  onSave,
  onResetError,
}: ProjectOnboardingTabProps): JSX.Element {
  const { totalOnboardingHours: total, consumedOnboardingHours: consumed } = project;
  const remaining = project.remainingOnboardingHours;
  const hasHours = total != null || consumed != null || remaining != null;
  const progress =
    total != null && total > 0 ? Math.min(100, Math.max(0, ((consumed ?? 0) / total) * 100)) : null;

  const startSession = (): EditSession => {
    const baseline = initialFormValues(project.onboardingStatus, project.onboardingOwner?.id, project);
    return { baseline, values: baseline };
  };
  const [session, setSession] = useState<EditSession | null>(() =>
    canEdit && initialEditing ? startSession() : null,
  );
  const editing = canEdit && session !== null;

  const begin = (): void => {
    onResetError?.();
    setSession(startSession());
  };
  const cancel = (): void => {
    if (isSaving) return;
    onResetError?.();
    setSession(null);
  };
  const update = (patch: Partial<OnboardingFormValues>): void =>
    setSession((s) => (s ? { ...s, values: { ...s.values, ...patch } } : s));

  const patch = session ? buildOnboardingPatch(session.values, session.baseline) : {};
  const dateErrors = session ? validateOnboardingDates(session.values, session.baseline) : {};
  const canSave = Object.keys(patch).length > 0 && Object.keys(dateErrors).length === 0;

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="subtitle2">Onboarding</Typography>
        {canEdit &&
          (editing ? (
            <Box sx={{ display: "flex", gap: 1 }} className="csm-print-hide">
              <Button color="inherit" size="small" onClick={cancel} disabled={isSaving}>
                Cancel
              </Button>
              <Button
                variant="contained"
                size="small"
                disabled={isSaving || !canSave}
                onClick={() => onSave(patch, () => setSession(null))}
              >
                {isSaving ? "Saving…" : "Save"}
              </Button>
            </Box>
          ) : (
            <Button
              variant="outlined"
              size="small"
              className="csm-print-hide"
              startIcon={<Pencil size={14} />}
              onClick={begin}
            >
              Edit
            </Button>
          ))}
      </Box>
      {editing && saveError && <Alert severity="error">{saveError}</Alert>}
      {editing && session ? (
        <>
          <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: GRID_COLUMNS }}>
            <FormControl size="small" fullWidth>
              <InputLabel
                id="project-onboarding-status-label"
                shrink={session.values.status !== ""}
                sx={{ top: "0px !important" }}
              >
                Onboarding status
              </InputLabel>
              <Select
                labelId="project-onboarding-status-label"
                label="Onboarding status"
                value={session.values.status}
                onChange={(e) => update({ status: e.target.value as ProjectOnboardingStatusValue })}
                disabled={isSaving}
                notched={session.values.status !== ""}
              >
                {PROJECT_ONBOARDING_STATUSES.map((s) => (
                  <MenuItem key={s} value={s}>
                    {ONBOARDING_STATUS_LABELS[s]}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <AsyncEntitySelect<InternalUserOption>
              id="project-onboarding-owner"
              label="Onboarding owner"
              placeholder="Search internal users…"
              value={session.values.ownerId}
              onChange={(ownerId) => update({ ownerId })}
              disabled={isSaving}
              useSearch={useSearchInternalUsers}
              getId={(u) => u.id}
              getLabel={(u) => u.name}
              knownLabel={project.onboardingOwner?.name}
              hideClearButton
            />
            <LocalizationProvider dateAdapter={AdapterDateFns}>
              {DATE_FIELDS.map((f) => (
                <DatePicker
                  key={f.key}
                  label={f.label}
                  value={parseDateOnly(session.values.dates[f.key])}
                  onChange={(next) =>
                    update({
                      dates: {
                        ...session.values.dates,
                        [f.key]:
                          next instanceof Date && !Number.isNaN(next.getTime())
                            ? formatDateOnly(next)
                            : "",
                      },
                    })
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
          </Box>
          <Typography variant="caption" color="text.secondary">
            Only internal users can be the owner. The owner and the dates can&apos;t be cleared once
            set, so an emptied field is left unchanged and won&apos;t be saved.
          </Typography>
        </>
      ) : (
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: GRID_COLUMNS }}>
          <ProjectMetaCell label="Onboarding status">
            <ProjectOnboardingStatusChip
              onboardingStatus={project.onboardingStatus}
              emptyFallback={<Typography variant="body2">{"—"}</Typography>}
            />
          </ProjectMetaCell>
          {isOnboardingEnabled(project.onboardingStatus) && (
            <ProjectMetaCell label="Onboarding owner">
              <ProjectOnboardingOwnerValue owner={project.onboardingOwner} />
            </ProjectMetaCell>
          )}
          {DATE_FIELDS.map((f) => (
            <ProjectMetaCell key={f.key} label={f.label}>
              <Typography variant="body2">{dateCell(project[f.read])}</Typography>
            </ProjectMetaCell>
          ))}
        </Box>
      )}
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
