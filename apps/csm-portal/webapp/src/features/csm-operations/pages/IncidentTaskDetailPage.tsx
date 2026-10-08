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

import { Box, Button, Card, Chip, Link, MenuItem, Skeleton, TextField, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type ReactNode } from "react";
import { Link as RouterLink, useLocation } from "react-router";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useGetIncidentTask } from "@features/csm-operations/api/useGetIncidentTask";
import { usePatchIncidentTask } from "@features/csm-operations/api/usePatchIncidentTask";
import CloseIncidentTaskDialog, {
  type ClosedTaskState,
} from "@features/csm-operations/components/CloseIncidentTaskDialog";
import {
  CLOSED_INCIDENT_TASK_STATES,
  incidentRelatedTabPath,
} from "@features/csm-operations/utils/incidents";
import type { BeEntityRef, BeIncidentTaskState } from "@api/backend/types";
import { useNavTransition } from "@hooks/useNavTransition";
import { useNormalizedIdParam } from "@hooks/useNormalizedIdParam";

const OPERATIONS_PATH = "/operations";

/**
 * The State menu. Any state can be picked from any other, as in ServiceNow --
 * incident_task has no state model, buttons or rules of its own (discovery
 * script 70) -- but the three closed states sit behind one "Close" entry,
 * whose dialog asks which outcome and for optional close notes.
 */
const OPEN_TASK_STATES: { value: BeIncidentTaskState; label: string }[] = [
  { value: "PENDING", label: "Pending" },
  { value: "OPEN", label: "Open" },
  { value: "WORK_IN_PROGRESS", label: "Work in Progress" },
];
const CLOSE_OPTION = "CLOSE";
const CLOSED_TASK_LABELS: Record<ClosedTaskState, string> = {
  CLOSED_COMPLETE: "Closed Complete",
  CLOSED_INCOMPLETE: "Closed Incomplete",
  CLOSED_SKIPPED: "Closed Skipped",
};

function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      dateStyle: "medium",
      timeStyle: "short",
    }) ?? "—"
  );
}

/** "MODERATE" -> "Moderate". */
function humanize(value?: string | null): string {
  if (!value) return "—";
  return value.charAt(0) + value.slice(1).toLowerCase().replace(/_/g, " ");
}

function MetaCell({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
      >
        {label}
      </Typography>
      <Box sx={{ minWidth: 0 }}>{children}</Box>
    </Box>
  );
}

function RefText({ value }: { value?: BeEntityRef | null }): JSX.Element {
  return <Typography variant="body2">{value?.name || "—"}</Typography>;
}

/**
 * Detail page for one incident task, opened from the incident's Related tab.
 * An open task can be closed (which is what lets its incident be closed);
 * a closed one can be reopened.
 */
export default function IncidentTaskDetailPage(): JSX.Element {
  const id = useNormalizedIdParam("id");
  const navigate = useNavTransition();
  const { data, isLoading, isError } = useGetIncidentTask(id);
  const patchTask = usePatchIncidentTask();
  // Set while the close dialog is open: the closed state picked from the
  // State dropdown, pre-selected in the dialog so close notes can go with it.
  const [closeAs, setCloseAs] = useState<ClosedTaskState | null>(null);
  const closeOpen = closeAs !== null;
  // Prefer the page the row link captured; else the parent incident's
  // Related tab once the task is loaded; else Operations.
  const backState = useLocation().state as { from?: string } | undefined;
  const parentIncidentId = data?.incident?.id;
  const backTarget =
    backState?.from ??
    (parentIncidentId ? incidentRelatedTabPath(parentIncidentId) : OPERATIONS_PATH);

  const BackButton = (
    <Button
      variant="text"
      size="small"
      className="csm-print-hide"
      startIcon={<ArrowLeft size={16} />}
      onClick={() => navigate(backTarget)}
      sx={{ alignSelf: "flex-start" }}
    >
      Back
    </Button>
  );

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rounded" height={32} width={240} />
        <Skeleton variant="rounded" height={260} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="body1" color="error">
          Could not load incident task {id}.
        </Typography>
      </Box>
    );
  }

  if (!data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="h5">Incident task not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No incident task with id <code>{id}</code>.
        </Typography>
      </Box>
    );
  }

  const task = data;

  const closedState =
    task.state && CLOSED_INCIDENT_TASK_STATES.includes(task.state) ? (task.state as ClosedTaskState) : null;

  // An open state saves at once; "Close" opens the close dialog, starting
  // from the task's own outcome when it is already closed.
  const onStateChange = (next: string): void => {
    if (next === task.state) return;
    patchTask.reset();
    if (next === CLOSE_OPTION) {
      setCloseAs(closedState ?? "CLOSED_COMPLETE");
      return;
    }
    patchTask.mutate({ id: task.id as string, patch: { state: next as BeIncidentTaskState } });
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      {BackButton}

      <Box
        sx={{
          display: "flex",
          gap: 2,
          alignItems: "flex-start",
          flexWrap: { xs: "wrap", md: "nowrap" },
          justifyContent: "space-between",
        }}
      >
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1, minWidth: 0 }}>
          <Typography
            variant="h6"
            sx={{ fontFamily: "monospace", fontWeight: 700, letterSpacing: 0.2, lineHeight: 1.2 }}
          >
            {task.number || task.id}
          </Typography>
          {task.stateLabel && (
            <Box>
              <Chip size="small" label={task.stateLabel} />
            </Box>
          )}
          <Typography variant="h5">{task.subject || "Incident task"}</Typography>
        </Box>
        <Box className="csm-print-hide" sx={{ flexShrink: 0 }}>
          <TextField
            select
            size="small"
            label="State"
            value={task.state ?? ""}
            onChange={(e) => onStateChange(e.target.value)}
            disabled={patchTask.isPending}
            sx={{ minWidth: 200 }}
          >
            {OPEN_TASK_STATES.map((s) => (
              <MenuItem key={s.value} value={s.value}>
                {s.label}
              </MenuItem>
            ))}
            {/* A closed task shows its outcome; the menu itself offers only "Close". */}
            {closedState && (
              <MenuItem value={closedState} sx={{ display: "none" }}>
                {CLOSED_TASK_LABELS[closedState]}
              </MenuItem>
            )}
            <MenuItem value={CLOSE_OPTION}>Close</MenuItem>
          </TextField>
        </Box>
      </Box>
      {!closeOpen && patchTask.isError && (
        <Typography variant="body2" color="error">
          {patchTask.error.message}
        </Typography>
      )}

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Overview</Typography>
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", md: "repeat(3, 1fr)" },
          }}
        >
          <MetaCell label="Incident">
            {task.incident?.id ? (
              <Link
                component={RouterLink}
                to={incidentRelatedTabPath(task.incident.id)}
                variant="body2"
              >
                {task.incident.number || task.incident.id}
              </Link>
            ) : (
              <Typography variant="body2">—</Typography>
            )}
          </MetaCell>
          <MetaCell label="Priority">
            <Typography variant="body2">{humanize(task.priority)}</Typography>
          </MetaCell>
          <MetaCell label="Assignment group">
            <RefText value={task.assignmentGroup} />
          </MetaCell>
          <MetaCell label="Assigned to">
            <RefText value={task.assignedTo} />
          </MetaCell>
          <MetaCell label="Opened">
            <Typography variant="body2">{formatDateTime(task.openedOn)}</Typography>
          </MetaCell>
          <MetaCell label="Closed">
            <Typography variant="body2">{formatDateTime(task.closedOn)}</Typography>
          </MetaCell>
        </Box>
        {task.closeNotes && (
          <MetaCell label="Close notes">
            <Typography variant="body2" sx={{ whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}>
              {task.closeNotes}
            </Typography>
          </MetaCell>
        )}
      </Card>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
        <Typography variant="subtitle2">Description</Typography>
        {task.description ? (
          <Typography variant="body2" sx={{ whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}>
            {task.description}
          </Typography>
        ) : (
          <Typography variant="body2" color="text.secondary">
            No description.
          </Typography>
        )}
      </Card>

      {closeOpen && (
        <CloseIncidentTaskDialog
          taskNumber={task.number || "task"}
          isSubmitting={patchTask.isPending}
          error={patchTask.isError ? patchTask.error.message : null}
          initialState={closeAs ?? undefined}
          onClose={() => setCloseAs(null)}
          onConfirm={({ state, closeNotes }) =>
            patchTask.mutate(
              { id: task.id as string, patch: closeNotes ? { state, closeNotes } : { state } },
              { onSuccess: () => setCloseAs(null) },
            )
          }
        />
      )}
    </Box>
  );
}
