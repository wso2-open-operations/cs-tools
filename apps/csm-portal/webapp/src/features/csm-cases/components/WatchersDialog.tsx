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
  InputAdornment,
  Skeleton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Search, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import type { NormalizedUser } from "@features/csm-users/types/csmUsers";
import type { CaseWatcher } from "@features/csm-cases/types/csmCases";

interface WatchersDialogProps {
  /** Current watch list; the dialog edits a local copy and submits a full replacement. */
  currentWatchers: CaseWatcher[];
  /** True while the PATCH is in flight; disables the actions. */
  isSaving: boolean;
  onClose: () => void;
  /** Full-list-replace submit (`PATCH { watchList: string[] }`) — every email currently selected. */
  onSave: (emails: string[]) => void;
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
 * Edit a case's watch list. Unlike {@link AssignEngineerDialog} (single-select,
 * immediate PATCH per pick), this collects a set of emails locally and submits
 * one full-list-replace `PATCH /cases/{id}` (`watchList`) on Save — the backend
 * has no add/remove-one endpoint, only a full replace. Search reuses
 * `POST /users/search`; watchers already on the case are shown as removable
 * chips regardless of whether they still match the current search text.
 * ServiceNow-source only; the caller surfaces a rejection on another source.
 */
export default function WatchersDialog({
  currentWatchers,
  isSaving,
  onClose,
  onSave,
}: WatchersDialogProps): JSX.Element {
  // Selected watchers, keyed by lowercased email (the identity the backend
  // PATCH accepts). Seeded from the case's current watch list.
  const [selected, setSelected] = useState<Map<string, { name: string; email: string }>>(
    () =>
      new Map(
        currentWatchers
          .filter((w): w is CaseWatcher & { email: string } => !!w.email)
          .map((w) => [w.email.toLowerCase(), { name: w.name, email: w.email }]),
      ),
  );

  const [input, setInput] = useState("");
  const search = useDebouncedValue(input.trim(), 300);
  const { data, isFetching, isError } = useSearchUsers({
    filters: {
      ...(search.length > 0 && { searchQuery: search }),
      active: true,
    },
    pagination: { limit: 8, offset: 0 },
  });

  const candidates = useMemo(
    () => (data?.users ?? []).filter((u) => !!u.email && u.active !== false),
    [data],
  );

  const toggle = (name: string, email: string): void => {
    setSelected((prev) => {
      const next = new Map(prev);
      const key = email.toLowerCase();
      if (next.has(key)) next.delete(key);
      else next.set(key, { name, email });
      return next;
    });
  };

  const remove = (key: string): void => {
    setSelected((prev) => {
      const next = new Map(prev);
      next.delete(key);
      return next;
    });
  };

  const originalKeys = useMemo(
    () =>
      new Set(
        currentWatchers
          .filter((w): w is CaseWatcher & { email: string } => !!w.email)
          .map((w) => w.email.toLowerCase()),
      ),
    [currentWatchers],
  );
  const changed =
    selected.size !== originalKeys.size ||
    [...selected.keys()].some((k) => !originalKeys.has(k));

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Manage watchers</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Typography variant="body2" color="text.secondary">
            Watchers are notified of updates to this case.
          </Typography>

          {selected.size > 0 && (
            <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5 }}>
              {[...selected.entries()].map(([key, w]) => (
                <Chip
                  key={key}
                  size="small"
                  variant="outlined"
                  label={w.name || w.email}
                  disabled={isSaving}
                  onDelete={() => remove(key)}
                  deleteIcon={<X size={14} />}
                />
              ))}
            </Box>
          )}

          <TextField
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Search people to add…"
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
              <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5, pt: 0.75 }}>
                {[0, 1, 2, 3].map((i) => (
                  <Box key={i} sx={{ display: "flex", alignItems: "center", gap: 1.25, px: 1, py: 0.75 }}>
                    <Skeleton variant="circular" width={28} height={28} />
                    <Box sx={{ flex: 1, minWidth: 0 }}>
                      <Skeleton variant="text" sx={{ fontSize: "0.875rem", width: "55%" }} />
                      <Skeleton variant="text" sx={{ fontSize: "0.75rem", width: "75%" }} />
                    </Box>
                  </Box>
                ))}
              </Box>
            ) : isError ? (
              <Typography variant="body2" color="error" sx={{ py: 2 }}>
                Could not load people. Try again.
              </Typography>
            ) : candidates.length === 0 ? (
              <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
                No matches.
              </Typography>
            ) : (
              <Box sx={{ display: "flex", flexDirection: "column" }}>
                {candidates.map((u) => {
                  const name = fullName(u);
                  const email = u.email;
                  const isSelected = selected.has(email.toLowerCase());
                  return (
                    <Button
                      key={u.id}
                      variant="text"
                      color="inherit"
                      disabled={isSaving}
                      onClick={() => toggle(name, email)}
                      sx={{
                        justifyContent: "flex-start",
                        textTransform: "none",
                        px: 1,
                        py: 0.75,
                        gap: 1.25,
                        bgcolor: isSelected ? "action.selected" : undefined,
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
                          {email}
                        </Typography>
                      </Box>
                    </Button>
                  );
                })}
              </Box>
            )}
          </Box>

          <Typography variant="caption" color="text.secondary">
            Watchers apply to ServiceNow-managed cases.
          </Typography>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!changed || isSaving}
          loading={isSaving}
          onClick={() => onSave([...selected.values()].map((w) => w.email))}
        >
          Save watchers
        </Button>
      </DialogActions>
    </Dialog>
  );
}
