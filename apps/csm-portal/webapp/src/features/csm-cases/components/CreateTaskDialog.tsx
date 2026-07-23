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
  Avatar,
  Box,
  Button,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  InputAdornment,
  Skeleton,
  Switch,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Search, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import {
  INTERNAL_USER_ROLES,
  type NormalizedUser,
} from "@features/csm-users/types/csmUsers";
import type { BeCreateCaseTaskPayload } from "@api/backend/types";
import {
  formatDateTimeLocal,
  parseDateTimeLocal,
  resolveDisplayTimeZone,
  zonedInputToUtcIso,
} from "@utils/dateTime";

const { DateTimePicker, LocalizationProvider } = DatePickers;

export interface CreateTaskDialogProps {
  isSaving: boolean;
  onClose: () => void;
  onSubmit: (payload: BeCreateCaseTaskPayload) => void;
}

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

/**
 * Create a task on a case (`POST /cases/{caseId}/tasks`, ServiceNow data
 * source only). Assignee search reuses {@link AssignEngineerDialog}'s
 * search-and-pick pattern (single-select, no immediate submit — the pick just
 * fills the field, the whole form submits together). ServiceNow-source only;
 * the caller surfaces a rejection on another source.
 */
export default function CreateTaskDialog({
  isSaving,
  onClose,
  onSubmit,
}: CreateTaskDialogProps): JSX.Element {
  const [subject, setSubject] = useState("");
  const [dueDate, setDueDate] = useState("");
  const [assigneeEmail, setAssigneeEmail] = useState("");
  const [assigneeName, setAssigneeName] = useState("");
  const [assigneeInput, setAssigneeInput] = useState("");
  const [visibleToCustomer, setVisibleToCustomer] = useState(false);

  const timeZone = resolveDisplayTimeZone();
  const search = useDebouncedValue(assigneeInput.trim(), 300);
  const { data, isFetching, isError } = useSearchUsers({
    filters: {
      ...(search.length > 0 && { searchQuery: search }),
      roles: INTERNAL_USER_ROLES,
      active: true,
    },
    pagination: { limit: 8, offset: 0 },
  });

  const engineers = useMemo(
    () =>
      (data?.users ?? []).filter(
        (u) =>
          !!u.email &&
          u.active !== false &&
          (u.userType ? u.userType === "internal" : true),
      ),
    [data],
  );

  const canSubmit = subject.trim().length > 0;

  const handleSubmit = (): void => {
    if (!canSubmit) return;
    const dueDateIso = dueDate ? zonedInputToUtcIso(dueDate, timeZone) : null;
    onSubmit({
      subject: subject.trim(),
      ...(dueDateIso && { dueDate: dueDateIso }),
      ...(assigneeEmail && { assignedToEmail: assigneeEmail }),
      ...(visibleToCustomer && { visibleToCustomer: true }),
    });
  };

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Create task</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.75 }}>
          <TextField
            label="Subject"
            size="small"
            fullWidth
            autoFocus
            required
            value={subject}
            onChange={(e) => setSubject(e.target.value)}
            disabled={isSaving}
          />

          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DateTimePicker
              label={`Due date (${timeZone})`}
              value={parseDateTimeLocal(dueDate)}
              onChange={(next) =>
                setDueDate(
                  next instanceof Date && !Number.isNaN(next.getTime())
                    ? formatDateTimeLocal(next)
                    : "",
                )
              }
              disabled={isSaving}
              slotProps={{
                textField: { fullWidth: true, size: "small" },
                field: { clearable: true },
              }}
            />
          </LocalizationProvider>

          <Box>
            <Typography variant="caption" color="text.secondary" sx={{ mb: 0.5, display: "block" }}>
              Assignee (optional)
            </Typography>
            {assigneeEmail ? (
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                  p: 0.75,
                  borderRadius: 1,
                  border: 1,
                  borderColor: "divider",
                }}
              >
                <Avatar sx={{ width: 24, height: 24, fontSize: "0.7rem" }}>
                  {initialsOf(assigneeName || assigneeEmail)}
                </Avatar>
                <Typography variant="body2" sx={{ flex: 1 }} noWrap>
                  {assigneeName || assigneeEmail}
                </Typography>
                <Button
                  size="small"
                  color="inherit"
                  onClick={() => {
                    setAssigneeEmail("");
                    setAssigneeName("");
                  }}
                  disabled={isSaving}
                  aria-label="Clear assignee"
                >
                  <X size={14} />
                </Button>
              </Box>
            ) : (
              <>
                <TextField
                  value={assigneeInput}
                  onChange={(e) => setAssigneeInput(e.target.value)}
                  placeholder="Search engineers by name or email…"
                  size="small"
                  fullWidth
                  disabled={isSaving}
                  slotProps={{
                    input: {
                      startAdornment: (
                        <InputAdornment position="start">
                          <Search size={16} />
                        </InputAdornment>
                      ),
                    },
                  }}
                />
                {assigneeInput.trim().length > 0 && (
                  <Box sx={{ minHeight: 44, mt: 0.5 }}>
                    {isFetching ? (
                      <Skeleton variant="rounded" height={36} />
                    ) : isError ? (
                      <Typography variant="caption" color="error">
                        Could not load engineers.
                      </Typography>
                    ) : engineers.length === 0 ? (
                      <Typography variant="caption" color="text.secondary">
                        No matching engineers.
                      </Typography>
                    ) : (
                      <Box sx={{ display: "flex", flexDirection: "column" }}>
                        {engineers.map((u) => {
                          const name = fullName(u);
                          return (
                            <Button
                              key={u.id}
                              variant="text"
                              color="inherit"
                              disabled={isSaving}
                              onClick={() => {
                                setAssigneeEmail(u.email);
                                setAssigneeName(name);
                                setAssigneeInput("");
                              }}
                              sx={{
                                justifyContent: "flex-start",
                                textTransform: "none",
                                px: 1,
                                py: 0.5,
                                gap: 1,
                              }}
                            >
                              <Avatar sx={{ width: 22, height: 22, fontSize: "0.65rem" }}>
                                {initialsOf(name)}
                              </Avatar>
                              <Box sx={{ minWidth: 0, textAlign: "left" }}>
                                <Typography variant="body2" sx={{ lineHeight: 1.2 }} noWrap>
                                  {name}
                                </Typography>
                                <Typography variant="caption" color="text.secondary" noWrap>
                                  {u.email}
                                </Typography>
                              </Box>
                            </Button>
                          );
                        })}
                      </Box>
                    )}
                  </Box>
                )}
              </>
            )}
          </Box>

          <FormControlLabel
            control={
              <Switch
                checked={visibleToCustomer}
                onChange={(e) => setVisibleToCustomer(e.target.checked)}
                disabled={isSaving}
              />
            }
            label="Visible to customer"
          />

          <Typography variant="caption" color="text.secondary">
            Tasks apply to ServiceNow-managed cases.
          </Typography>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!canSubmit || isSaving}
          loading={isSaving}
          onClick={handleSubmit}
        >
          Create task
        </Button>
      </DialogActions>
    </Dialog>
  );
}
