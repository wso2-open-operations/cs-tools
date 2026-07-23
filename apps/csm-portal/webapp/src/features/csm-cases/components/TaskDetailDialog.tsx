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
  Avatar,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  InputAdornment,
  MenuItem,
  Select,
  Skeleton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Pencil, RefreshCw, Search, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useGetTask } from "@features/csm-cases/api/useGetTask";
import { useUpdateTask } from "@features/csm-cases/api/useUpdateTask";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import {
  INTERNAL_USER_ROLES,
  type NormalizedUser,
} from "@features/csm-users/types/csmUsers";
import { taskStateColor, taskStateLabel } from "@features/csm-cases/utils/taskState";
import { formatAbsoluteForUser } from "@utils/dateTime";

interface TaskDetailDialogProps {
  taskId: string;
  /** Owning case id, so a state/assignee edit also invalidates the case's
   * tasks list (see {@link useUpdateTask}). Optional: omit when the caller
   * only has the task id (the task's own detail query still refreshes). */
  caseId?: string;
  onClose: () => void;
}

/** Task states an engineer can set via the inline edit (excludes `OTHER`,
 * which is a display-only fallback for undocumented raw values, not a
 * settable target). */
const EDITABLE_TASK_STATES: Array<"OPEN" | "CLOSED"> = ["OPEN", "CLOSED"];

function initialsOf(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() ?? "")
    .join("");
}

function fullName(u: NormalizedUser): string {
  return u.name.trim() || u.userName;
}

function DetailCell({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Box>{children}</Box>
    </Box>
  );
}

/**
 * Detail view for a single task, opened from a row click on
 * {@link TasksWidget}. A lightweight dialog rather than a full separate
 * detail page/route — a task's data shape is much smaller than a call
 * request or change request, and the parent case is already known from the
 * page it's opened from. State and assignee are editable inline via
 * `PATCH /tasks/{id}` ({@link useUpdateTask}); everything else stays
 * read-only (the backend has no write path for the rest of the task shape).
 */
export function TaskDetailDialog({
  taskId,
  caseId,
  onClose,
}: TaskDetailDialogProps): JSX.Element {
  const { data: task, isLoading, isError, refetch } = useGetTask(taskId);
  const updateTask = useUpdateTask(taskId, caseId);
  const [editingAssignee, setEditingAssignee] = useState(false);
  const [assigneeInput, setAssigneeInput] = useState("");
  const assigneeSearch = useDebouncedValue(assigneeInput.trim(), 300);
  const { data: userResults, isFetching: isFetchingUsers } = useSearchUsers({
    filters: {
      ...(assigneeSearch.length > 0 && { searchQuery: assigneeSearch }),
      roles: INTERNAL_USER_ROLES,
      active: true,
    },
    pagination: { limit: 6, offset: 0 },
  });
  const assigneeCandidates = useMemo(
    () => (userResults?.users ?? []).filter((u) => !!u.email && u.active !== false),
    [userResults],
  );

  const onChangeState = (nextState: "OPEN" | "CLOSED"): void => {
    if (!task || nextState === task.state) return;
    updateTask.mutate({ state: nextState });
  };

  const onPickAssignee = (email: string): void => {
    setEditingAssignee(false);
    setAssigneeInput("");
    updateTask.mutate({ assignedToEmail: email });
  };

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle
        sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}
      >
        <Typography variant="subtitle1" component="span">
          Task details
        </Typography>
        <IconButton size="small" onClick={onClose} aria-label="Close">
          <X size={16} />
        </IconButton>
      </DialogTitle>
      <DialogContent dividers sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {isLoading && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Skeleton variant="text" width="60%" />
            <Skeleton variant="rounded" height={80} />
          </Box>
        )}

        {isError && (
          <Box
            sx={{
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              gap: 1,
              py: 3,
            }}
          >
            <Typography variant="body2" color="error">
              Could not load this task.
            </Typography>
            <Button
              size="small"
              variant="outlined"
              startIcon={<RefreshCw size={14} />}
              onClick={() => void refetch()}
              sx={{ textTransform: "none" }}
            >
              Retry
            </Button>
          </Box>
        )}

        {!isLoading && !isError && task && (
          <>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
              <Select
                size="small"
                value={EDITABLE_TASK_STATES.includes(task.state as "OPEN" | "CLOSED") ? task.state : ""}
                onChange={(e) => onChangeState(e.target.value as "OPEN" | "CLOSED")}
                disabled={updateTask.isPending}
                displayEmpty
                renderValue={() => (
                  <Chip
                    size="small"
                    variant="outlined"
                    color={taskStateColor(task.state)}
                    label={taskStateLabel(task.state)}
                  />
                )}
                sx={{ "& .MuiSelect-select": { py: 0.25, display: "flex" } }}
              >
                {EDITABLE_TASK_STATES.map((s) => (
                  <MenuItem key={s} value={s}>
                    {taskStateLabel(s)}
                  </MenuItem>
                ))}
              </Select>
              {task.requestTypeLabel && (
                <Chip size="small" variant="outlined" label={task.requestTypeLabel} />
              )}
              {updateTask.isError && (
                <Typography variant="caption" color="error">
                  Could not update this task.
                </Typography>
              )}
            </Box>

            <Typography variant="body1" sx={{ fontWeight: 600 }}>
              {task.subject}
            </Typography>

            <Divider />

            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fill, minmax(160px, 1fr))",
                gap: 2,
              }}
            >
              <DetailCell label="Parent case">
                <Typography variant="body2">
                  {task.parentCase?.number ?? task.parentCase?.id ?? "—"}
                </Typography>
              </DetailCell>
              <DetailCell label="Assigned to">
                {editingAssignee ? (
                  <Box>
                    <TextField
                      value={assigneeInput}
                      onChange={(e) => setAssigneeInput(e.target.value)}
                      placeholder="Search engineers…"
                      size="small"
                      fullWidth
                      autoFocus
                      disabled={updateTask.isPending}
                      slotProps={{
                        input: {
                          startAdornment: (
                            <InputAdornment position="start">
                              <Search size={14} />
                            </InputAdornment>
                          ),
                        },
                      }}
                    />
                    {assigneeInput.trim().length > 0 && (
                      <Box sx={{ mt: 0.5 }}>
                        {isFetchingUsers ? (
                          <Typography variant="caption" color="text.secondary">
                            Searching…
                          </Typography>
                        ) : assigneeCandidates.length === 0 ? (
                          <Typography variant="caption" color="text.secondary">
                            No matches.
                          </Typography>
                        ) : (
                          assigneeCandidates.map((u) => (
                            <Button
                              key={u.id}
                              size="small"
                              variant="text"
                              color="inherit"
                              startIcon={
                                <Avatar sx={{ width: 18, height: 18, fontSize: "0.6rem" }}>
                                  {initialsOf(fullName(u))}
                                </Avatar>
                              }
                              onClick={() => onPickAssignee(u.email)}
                              sx={{
                                display: "flex",
                                justifyContent: "flex-start",
                                textTransform: "none",
                                width: "100%",
                              }}
                            >
                              {fullName(u)}
                            </Button>
                          ))
                        )}
                      </Box>
                    )}
                  </Box>
                ) : (
                  <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                    <Typography variant="body2">{task.assignedTo?.name ?? "—"}</Typography>
                    <IconButton
                      size="small"
                      onClick={() => setEditingAssignee(true)}
                      aria-label="Change assignee"
                      disabled={updateTask.isPending}
                    >
                      <Pencil size={12} />
                    </IconButton>
                  </Box>
                )}
              </DetailCell>
              <DetailCell label="Product">
                <Typography variant="body2">{task.product?.name ?? "—"}</Typography>
              </DetailCell>
              {/* Environment is only shown when populated — most tasks won't
                  carry one, and this isn't the primary field on this record. */}
              {task.environmentLabel && (
                <DetailCell label="Environment">
                  <Typography variant="body2">{task.environmentLabel}</Typography>
                </DetailCell>
              )}
              {/* dueDate is genuinely sparse (~13% populated) — omitted from
                  the grid entirely rather than shown as a bare dash when
                  absent, so most tasks don't render an obviously-empty cell. */}
              {task.dueDate && (
                <DetailCell label="Due date">
                  <Typography variant="body2">
                    {formatAbsoluteForUser(task.dueDate) ?? "—"}
                  </Typography>
                </DetailCell>
              )}
              <DetailCell label="Created">
                <Typography variant="body2">
                  {formatAbsoluteForUser(task.createdOn) ?? "—"}
                </Typography>
              </DetailCell>
              <DetailCell label="Last updated">
                <Typography variant="body2">
                  {formatAbsoluteForUser(task.updatedOn) ?? "—"}
                </Typography>
              </DetailCell>
              <DetailCell label="Visible to customer">
                <Typography variant="body2">{task.visibleToCustomer ? "Yes" : "No"}</Typography>
              </DetailCell>
            </Box>
          </>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}

export default TaskDetailDialog;
