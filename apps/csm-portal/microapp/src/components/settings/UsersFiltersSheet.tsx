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

import { useEffect, useState } from "react";
import {
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import type { AdminUserRole } from "@src/types";
import { ALL_USER_ROLES, USER_ROLE_LABELS } from "./config";
import { EMPTY_USERS_FILTERS, type UsersFilters } from "./filters";

const STATUS_OPTIONS: { value: UsersFilters["active"]; label: string }[] = [
  { value: "all", label: "All" },
  { value: "active", label: "Active" },
  { value: "inactive", label: "Inactive" },
];

function toggleRole(roles: AdminUserRole[], role: AdminUserRole): AdminUserRole[] {
  return roles.includes(role) ? roles.filter((r) => r !== role) : [...roles, role];
}

interface UsersFiltersSheetProps {
  open: boolean;
  onClose: () => void;
  filters: UsersFilters;
  onApply: (filters: UsersFilters) => void;
}

// Mobile bottom-sheet equivalent of the webapp's CsmUsersPage filter row (role multi-select +
// status select), mirroring the microapp's own support/FiltersSheet.tsx bottom-sheet pattern.
export function UsersFiltersSheet({ open, onClose, filters, onApply }: UsersFiltersSheetProps) {
  const [draft, setDraft] = useState<UsersFilters>(filters);

  // Re-seed the draft from the last-applied filters each time the sheet opens, so reopening it
  // doesn't show stale in-progress edits from a previous open that was dismissed without applying.
  useEffect(() => {
    if (open) setDraft(filters);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-seed on open, not on every filters identity change
  }, [open]);

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xs">
      <DialogTitle sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        Filters
        <IconButton size="small" aria-label="Close filters" onClick={onClose}>
          <X size={18} />
        </IconButton>
      </DialogTitle>

      <Divider />

      <DialogContent>
        <Stack gap={2.5}>
          <Stack gap={1}>
            <Typography variant="subtitle2">Roles</Typography>
            <Stack direction="row" gap={1} flexWrap="wrap">
              {ALL_USER_ROLES.map((role) => {
                const isSelected = draft.roles.includes(role);
                return (
                  <Chip
                    key={role}
                    label={USER_ROLE_LABELS[role]}
                    size="small"
                    variant={isSelected ? "filled" : "outlined"}
                    color={isSelected ? "primary" : "default"}
                    aria-pressed={isSelected}
                    onClick={() => setDraft({ ...draft, roles: toggleRole(draft.roles, role) })}
                  />
                );
              })}
            </Stack>
          </Stack>

          <Stack gap={1}>
            <Typography variant="subtitle2">Status</Typography>
            <Stack direction="row" gap={1} flexWrap="wrap" role="radiogroup" aria-label="Status">
              {STATUS_OPTIONS.map((option) => {
                const isSelected = draft.active === option.value;
                return (
                  <Chip
                    key={option.value}
                    label={option.label}
                    size="small"
                    variant={isSelected ? "filled" : "outlined"}
                    color={isSelected ? "primary" : "default"}
                    role="radio"
                    aria-checked={isSelected}
                    onClick={() => setDraft({ ...draft, active: option.value })}
                  />
                );
              })}
            </Stack>
          </Stack>
        </Stack>
      </DialogContent>

      <Divider />

      <DialogActions>
        <Button
          onClick={() => {
            setDraft(EMPTY_USERS_FILTERS);
            onApply(EMPTY_USERS_FILTERS);
            onClose();
          }}
        >
          Clear all
        </Button>
        <Button
          variant="contained"
          onClick={() => {
            onApply(draft);
            onClose();
          }}
        >
          Apply
        </Button>
      </DialogActions>
    </Dialog>
  );
}
