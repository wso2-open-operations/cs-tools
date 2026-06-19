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
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  InputAdornment,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Search, UserCheck } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import type { User } from "@features/csm-users/types/csmUsers";

interface AssignEngineerDialogProps {
  /** Current assignee display name, or "Unassigned". */
  currentAssignee?: string;
  /** Signed-in engineer's email — enables the "Assign to me" shortcut. */
  currentUserEmail?: string;
  /** True while a PATCH is in flight; disables the actions. */
  isAssigning: boolean;
  onClose: () => void;
  /** Assign the case to this engineer's email (`PATCH { assigneeEmail }`). */
  onAssign: (email: string) => void;
}

function initialsOf(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() ?? "")
    .join("");
}

function fullName(u: User): string {
  return [u.firstName, u.lastName].filter(Boolean).join(" ").trim() || u.userName;
}

/**
 * Pick the engineer to assign a case to. Searches internal users via
 * `POST /users/search` and assigns by email through `PATCH /cases/{id}`
 * (`assigneeEmail`). Assignment is a ServiceNow-source capability; on a
 * Postgres-sourced case the backend rejects it and the caller surfaces the
 * error. Mount only while open so the user search isn't issued in the
 * background.
 */
export default function AssignEngineerDialog({
  currentAssignee,
  currentUserEmail,
  isAssigning,
  onClose,
  onAssign,
}: AssignEngineerDialogProps): JSX.Element {
  const [input, setInput] = useState("");
  const search = useDebouncedValue(input.trim(), 300);
  const { data, isFetching, isError } = useSearchUsers({
    ...(search.length > 0 && { searchQuery: search }),
    pagination: { limit: 8, offset: 0 },
  });

  // Only internal engineers are assignable, and only ones that carry an email
  // (the assign call is email-based).
  const engineers = useMemo(
    () =>
      (data?.users ?? []).filter(
        (u) => u.userType === "internal" && !!u.email,
      ),
    [data],
  );

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Assign engineer</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Typography variant="body2" color="text.secondary">
            Currently assigned to{" "}
            <Box component="span" sx={{ fontWeight: 600, color: "text.primary" }}>
              {currentAssignee || "Unassigned"}
            </Box>
            .
          </Typography>

          {currentUserEmail && (
            <Button
              variant="outlined"
              size="small"
              startIcon={<UserCheck size={16} />}
              disabled={isAssigning}
              onClick={() => onAssign(currentUserEmail)}
              sx={{ alignSelf: "flex-start" }}
            >
              Assign to me
            </Button>
          )}

          <TextField
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Search engineers by name or email…"
            size="small"
            fullWidth
            autoFocus
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <Search size={16} />
                </InputAdornment>
              ),
            }}
          />

          <Box sx={{ minHeight: 160 }}>
            {isFetching ? (
              <Box sx={{ display: "flex", justifyContent: "center", py: 3 }}>
                <CircularProgress size={22} />
              </Box>
            ) : isError ? (
              <Typography variant="body2" color="error" sx={{ py: 2 }}>
                Could not load engineers. Try again.
              </Typography>
            ) : engineers.length === 0 ? (
              <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
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
                      disabled={isAssigning}
                      onClick={() => onAssign(u.email)}
                      sx={{
                        justifyContent: "flex-start",
                        textTransform: "none",
                        px: 1,
                        py: 0.75,
                        gap: 1.25,
                      }}
                    >
                      <Avatar sx={{ width: 28, height: 28, fontSize: "0.75rem" }}>
                        {initialsOf(name)}
                      </Avatar>
                      <Box sx={{ minWidth: 0, textAlign: "left" }}>
                        <Typography variant="body2" sx={{ lineHeight: 1.2 }} noWrap>
                          {name}
                        </Typography>
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          noWrap
                          sx={{ display: "block" }}
                        >
                          {u.email}
                        </Typography>
                      </Box>
                    </Button>
                  );
                })}
              </Box>
            )}
          </Box>

          <Typography variant="caption" color="text.secondary">
            Assignment applies to ServiceNow-managed cases.
          </Typography>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isAssigning}>
          Cancel
        </Button>
      </DialogActions>
    </Dialog>
  );
}
