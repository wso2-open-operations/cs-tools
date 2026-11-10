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

import {
  Alert,
  Box,
  Card,
  Chip,
  CircularProgress,
  FormControlLabel,
  FormHelperText,
  Switch,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX, type ReactNode } from "react";
import { BackendApiError } from "@api/backend/client";
import { usePatchProjectSuspensionOverride } from "@features/csm-projects/api/usePatchProjectSuspensionOverride";
import ClosureStateChip from "@features/csm-projects/components/ClosureStateChip";
import ProjectMetaCell from "@features/csm-projects/components/ProjectMetaCell";
import type { ProjectDetails } from "@features/csm-projects/types/csmProjects";
import {
  parseClosureProcessState,
  type ActionTone,
} from "@features/csm-projects/utils/closureProcessState";
import { formatUtcDateForDisplay } from "@utils/dateTime";

interface ProjectClosureTabProps {
  /** The project the page already loaded; this tab fetches nothing itself. */
  project: ProjectDetails;
  /** Make the automatic-closure switch editable (`canUpdateProjectSuspensionOverride`).
   *  Only decides what is shown; the backend's 403 is the real gate. */
  canEdit: boolean;
}

const GRID_COLUMNS = {
  xs: "1fr",
  sm: "repeat(2, minmax(0, 1fr))",
  md: "repeat(3, minmax(0, 1fr))",
};

function dateCell(value?: string | null): string {
  return formatUtcDateForDisplay(value) ?? "—";
}

/** Same 5xx-safe rule as the page's `closureStateSaveErrorMessage`: a raw
 *  upstream message is only shown for a client error (<500). */
function overrideSaveErrorMessage(err: unknown): string {
  return err instanceof BackendApiError && err.status < 500 && err.message
    ? err.message
    : "Could not update automatic closure for this project. Please try again.";
}

function toneColor(tone: ActionTone): "success" | "warning" | "error" | undefined {
  return tone === "default" ? undefined : tone;
}

function SectionTitle({ id, children }: { id?: string; children: ReactNode }): JSX.Element {
  return (
    <Typography id={id} variant="subtitle2" component="h3">
      {children}
    </Typography>
  );
}

function TrackCard({
  title,
  detail,
  state,
}: {
  title: string;
  detail: string;
  state: string | null | undefined;
}): JSX.Element {
  return (
    <Box
      component="li"
      sx={{
        listStyle: "none",
        border: 1,
        borderColor: "divider",
        borderRadius: 1,
        p: 1.5,
        display: "flex",
        flexDirection: "column",
        gap: 0.75,
        minWidth: 0,
      }}
    >
      <Typography variant="body2" sx={{ fontWeight: 600 }}>
        {title}
      </Typography>
      <Typography variant="caption" color="text.secondary">
        {detail}
      </Typography>
      <Box>
        <ClosureStateChip closureState={state} />
      </Box>
    </Box>
  );
}

/**
 * Project detail "Closure" tab: the overall closure state, the three tracks
 * it is derived from, an admin switch that excludes the project from the
 * automatic closure process, and the process's own record of what it has
 * done. Read-only except for that switch, which saves immediately (no confirm
 * step) and reverts with an error message if the backend refuses.
 */
export default function ProjectClosureTab({
  project,
  canEdit,
}: ProjectClosureTabProps): JSX.Element {
  const patch = usePatchProjectSuspensionOverride(project.id);
  // The value being saved while the request is in flight (null when idle).
  const [pendingValue, setPendingValue] = useState<boolean | null>(null);
  // The backend's answer to the last successful save. Tied to the project
  // value it replaced, so once the refetched project differs it is ignored and
  // the page's own data is authoritative again.
  const [confirmed, setConfirmed] = useState<{ replaced: boolean | undefined; value: boolean } | null>(
    null,
  );
  const [notice, setNotice] = useState<string | null>(null);

  const fromProject = project.suspensionProcessOverridden;
  const saved =
    confirmed && confirmed.replaced === fromProject ? confirmed.value : (fromProject ?? false);
  const checked = pendingValue ?? saved;
  const isSaving = patch.isPending;

  const handleToggle = (next: boolean): void => {
    setNotice(null);
    patch.reset();
    setPendingValue(next);
    patch.mutate(next, {
      onSuccess: (res) => {
        setConfirmed({
          replaced: fromProject,
          value: res?.project?.suspensionProcessOverridden ?? next,
        });
        setNotice(
          next
            ? "This project is now excluded from automatic closure."
            : "This project is included in automatic closure again.",
        );
      },
      onSettled: () => setPendingValue(null),
    });
  };

  const record = parseClosureProcessState(project.suspensionProcessState);
  const complianceDetail = project.complianceViolationDate
    ? `Violation date ${dateCell(project.complianceViolationDate)}`
    : "No violation date";

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2.5 }}>
      <Typography variant="subtitle2" component="h2">
        Closure
      </Typography>

      <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: GRID_COLUMNS }}>
        <ProjectMetaCell label="Overall closure state">
          <ClosureStateChip closureState={project.closureState} />
        </ProjectMetaCell>
        <ProjectMetaCell label="Start date">
          <Typography variant="body2">{dateCell(project.startDate)}</Typography>
        </ProjectMetaCell>
        <ProjectMetaCell label="End date">
          <Typography variant="body2">{dateCell(project.endDate)}</Typography>
        </ProjectMetaCell>
      </Box>

      <Box component="section" aria-labelledby="closure-tracks-title" sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
        <SectionTitle id="closure-tracks-title">Closure tracks</SectionTitle>
        <Box
          component="ul"
          sx={{ display: "grid", gap: 2, gridTemplateColumns: GRID_COLUMNS, m: 0, p: 0 }}
        >
          <TrackCard
            title="Subscription end date"
            detail={`End date ${dateCell(project.endDate)}`}
            state={project.endDateClosureState}
          />
          <TrackCard
            title="Invoice due date"
            detail="Overdue invoices"
            state={project.invoiceDueDateClosureState}
          />
          <TrackCard
            title="Compliance violation"
            detail={complianceDetail}
            state={project.complianceViolationClosureState}
          />
        </Box>
      </Box>

      <Box component="section" aria-labelledby="closure-auto-title" sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        <SectionTitle id="closure-auto-title">Automatic closure</SectionTitle>
        {canEdit ? (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <FormControlLabel
                control={
                  <Switch
                    checked={checked}
                    disabled={isSaving}
                    onChange={(e) => handleToggle(e.target.checked)}
                    slotProps={{
                      input: {
                        "aria-describedby": "closure-auto-help",
                        "aria-busy": isSaving,
                      },
                    }}
                  />
                }
                label="Exclude this project from automatic closure"
              />
              {isSaving && <CircularProgress size={16} aria-label="Saving" />}
            </Box>
            <FormHelperText id="closure-auto-help" sx={{ mx: 0 }}>
              When on, the automatic closure process never sends notices for this project or
              suspends it.
            </FormHelperText>
          </Box>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
            <Typography variant="body2">
              {fromProject === undefined
                ? "—"
                : checked
                  ? "Excluded from automatic closure"
                  : "Included in automatic closure"}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              When excluded, the automatic closure process never sends notices for this project
              or suspends it. Only an administrator can change this.
            </Typography>
          </Box>
        )}
        {notice && !patch.isError && (
          <Alert severity="success" role="status" onClose={() => setNotice(null)}>
            {notice}
          </Alert>
        )}
        {patch.isError && <Alert severity="error">{overrideSaveErrorMessage(patch.error)}</Alert>}
      </Box>

      <Box component="section" aria-labelledby="closure-record-title" sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
        <SectionTitle id="closure-record-title">Closure process record</SectionTitle>
        {record.kind === "empty" && (
          <Typography variant="body2" color="text.secondary">
            No closure process record yet.
          </Typography>
        )}
        {record.kind === "tracks" && (
          <Box
            component="ul"
            sx={{ display: "grid", gap: 2, gridTemplateColumns: GRID_COLUMNS, m: 0, p: 0 }}
          >
            {record.tracks.map((track) => (
              <Box
                key={track.key}
                component="li"
                sx={{
                  listStyle: "none",
                  border: 1,
                  borderColor: "divider",
                  borderRadius: 1,
                  p: 1.5,
                  display: "flex",
                  flexDirection: "column",
                  gap: 0.75,
                  minWidth: 0,
                }}
              >
                <Typography variant="body2" sx={{ fontWeight: 600 }}>
                  {track.title}
                </Typography>
                <Typography variant="body2">{track.label}</Typography>
                {track.actions.length > 0 && (
                  <Box
                    component="ul"
                    sx={{ m: 0, p: 0, display: "flex", flexDirection: "column", gap: 0.5 }}
                  >
                    {track.actions.map((a) => (
                      <Box
                        component="li"
                        key={a.key}
                        sx={{
                          listStyle: "none",
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "space-between",
                          gap: 1,
                        }}
                      >
                        <Typography variant="caption" color="text.secondary">
                          {a.title}
                        </Typography>
                        <Chip
                          size="small"
                          variant="outlined"
                          label={a.result}
                          color={toneColor(a.tone)}
                        />
                      </Box>
                    ))}
                  </Box>
                )}
              </Box>
            ))}
          </Box>
        )}
        {(record.kind === "unrecognised" ||
          (record.kind === "tracks" && record.leftover !== null)) && (
          <Box component="details" sx={{ minWidth: 0 }}>
            <Typography
              component="summary"
              variant="body2"
              sx={{ cursor: "pointer", "&:focus-visible": { outline: "2px solid", outlineColor: "primary.main", outlineOffset: 2 } }}
            >
              {record.kind === "unrecognised"
                ? "Closure process record (unrecognised format)"
                : "Other fields in the closure process record"}
            </Typography>
            <Box
              component="pre"
              tabIndex={0}
              aria-label="Raw closure process record"
              sx={{
                mt: 1,
                mb: 0,
                p: 1.5,
                maxHeight: 240,
                overflow: "auto",
                bgcolor: "action.hover",
                borderRadius: 1,
                fontFamily: "monospace",
                fontSize: "0.75rem",
                whiteSpace: "pre-wrap",
                wordBreak: "break-word",
              }}
            >
              {record.kind === "unrecognised" ? record.raw : record.leftover}
            </Box>
          </Box>
        )}
      </Box>
    </Card>
  );
}
