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
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  Skeleton,
  Typography,
} from "@wso2/oxygen-ui";
import { RefreshCw, X } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useGetTask } from "@features/csm-cases/api/useGetTask";
import { taskStateColor, taskStateLabel } from "@features/csm-cases/utils/taskState";
import { formatAbsoluteForUser } from "@utils/dateTime";

interface TaskDetailDialogProps {
  taskId: string;
  onClose: () => void;
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
 * Read-only detail view for a single task, opened from a row click on
 * {@link TasksWidget}. A lightweight dialog rather than a full separate
 * detail page/route — a task's data shape is much smaller than a call
 * request or change request, and the parent case is already known from the
 * page it's opened from.
 */
export function TaskDetailDialog({ taskId, onClose }: TaskDetailDialogProps): JSX.Element {
  const { data: task, isLoading, isError, refetch } = useGetTask(taskId);

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
              <Chip
                size="small"
                variant="outlined"
                color={taskStateColor(task.state)}
                label={taskStateLabel(task.state)}
              />
              {task.requestTypeLabel && (
                <Chip size="small" variant="outlined" label={task.requestTypeLabel} />
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
                  {task.parentCase.number ?? task.parentCase.id}
                </Typography>
              </DetailCell>
              <DetailCell label="Assigned to">
                <Typography variant="body2">{task.assignedTo?.name ?? "—"}</Typography>
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
