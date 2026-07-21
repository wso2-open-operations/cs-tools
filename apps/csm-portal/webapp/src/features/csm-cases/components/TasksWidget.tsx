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
  Card,
  Chip,
  IconButton,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { ListChecks, RefreshCw } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useSearchCaseTasks } from "@features/csm-cases/api/useSearchCaseTasks";
import { taskStateColor, taskStateLabel } from "@features/csm-cases/utils/taskState";
import { formatAbsoluteForUser } from "@utils/dateTime";
import RelativeTime from "@components/RelativeTime";
import { TaskDetailDialog } from "./TaskDetailDialog";

const TASKS_TABLE_COLUMNS = ["Subject", "State", "Due date", "Assignee", "Updated"];

interface TasksWidgetProps {
  caseId: string;
}

/**
 * Tasks tab content on the case detail page. Tasks are a lightweight,
 * case-scoped to-do item (no multi-stage lifecycle, unlike call requests /
 * change requests), so the list is a plain table with a row click opening a
 * small read-only detail dialog rather than a full separate detail page.
 */
export function TasksWidget({ caseId }: TasksWidgetProps): JSX.Element {
  const { data, isLoading, isError, isFetching, refetch } = useSearchCaseTasks(caseId);
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);

  const tasks = data?.tasks ?? [];
  const total = data?.total ?? tasks.length;
  const isTruncated = !isLoading && !isError && tasks.length < total;

  return (
    <>
      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        {/* Header */}
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 1,
            flexWrap: "wrap",
          }}
        >
          <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
            <ListChecks size={16} />
            <Typography variant="subtitle2">Tasks</Typography>
            {!isLoading && !isError && (
              <Chip size="small" variant="outlined" label={`${total} total`} />
            )}
          </Box>
          <IconButton
            size="small"
            onClick={() => void refetch()}
            disabled={isFetching}
            aria-label="Refresh tasks"
          >
            <RefreshCw size={14} />
          </IconButton>
        </Box>

        {isTruncated ? (
          <Typography variant="caption" color="text.secondary">
            {`Showing first ${tasks.length} of ${total}`}
          </Typography>
        ) : null}

        {/* Content */}
        {isError ? (
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
              Could not load tasks for this case.
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
        ) : (
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  {TASKS_TABLE_COLUMNS.map((col) => (
                    <TableCell key={col}>{col}</TableCell>
                  ))}
                </TableRow>
              </TableHead>
              <TableBody>
                {isLoading ? (
                  [0, 1, 2].map((i) => (
                    <TableRow key={i}>
                      {TASKS_TABLE_COLUMNS.map((col) => (
                        <TableCell key={col}>
                          <Skeleton variant="text" />
                        </TableCell>
                      ))}
                    </TableRow>
                  ))
                ) : tasks.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={TASKS_TABLE_COLUMNS.length}
                      align="center"
                      sx={{ py: 4 }}
                    >
                      <Typography variant="body2" color="text.secondary">
                        No tasks on this case.
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : (
                  tasks.map((task) => (
                    <TableRow
                      key={task.id}
                      hover
                      onClick={() => setSelectedTaskId(task.id)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          setSelectedTaskId(task.id);
                        }
                      }}
                      tabIndex={0}
                      role="button"
                      aria-label={`View task ${task.subject}`}
                      sx={{ cursor: "pointer" }}
                    >
                      <TableCell>{task.subject}</TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          variant="outlined"
                          color={taskStateColor(task.state)}
                          label={taskStateLabel(task.state)}
                        />
                      </TableCell>
                      {/* dueDate is sparse (~13% populated) — a plain dash for
                          the rest is the expected common case, not a broken
                          layout. */}
                      <TableCell>
                        {task.dueDate ? (formatAbsoluteForUser(task.dueDate) ?? "—") : "—"}
                      </TableCell>
                      <TableCell>{task.assignedTo?.name ?? "—"}</TableCell>
                      <TableCell>
                        <RelativeTime iso={task.updatedOn} />
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Card>

      {selectedTaskId && (
        <TaskDetailDialog
          taskId={selectedTaskId}
          onClose={() => setSelectedTaskId(null)}
        />
      )}
    </>
  );
}

export default TasksWidget;
