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
  Switch,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { ALL_WORK_STATES, WORK_STATE_LABEL } from "@components/support/config";
import { EMPTY_FILTERS, type CaseFilters } from "@components/support/filters";

function toggle<T>(list: T[], value: T): T[] {
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}

interface ServiceRequestsFiltersSheetProps {
  open: boolean;
  onClose: () => void;
  filters: CaseFilters;
  onApply: (filters: CaseFilters) => void;
}

// Mirrors the support FiltersSheet (src/components/support/FiltersSheet.tsx), minus the Severity
// section — severity is case-type-only, and the webapp's own CsmIssuesView explicitly hides its
// severity filter when locked to a non-"case" type (showSeverityFilter derives from the locked
// type). Work state stays, disabled until "Work in progress" is selected, same as the webapp.
export function ServiceRequestsFiltersSheet({ open, onClose, filters, onApply }: ServiceRequestsFiltersSheetProps) {
  const [draft, setDraft] = useState<CaseFilters>(filters);

  useEffect(() => {
    if (open) setDraft(filters);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-seed on open
  }, [open]);

  const workStateDisabled = !draft.states.includes("work_in_progress");

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
            <Typography variant="subtitle2" color={workStateDisabled ? "text.disabled" : "text.primary"}>
              Work state
            </Typography>
            <Stack direction="row" gap={1} flexWrap="wrap">
              {ALL_WORK_STATES.map((workState) => {
                const isSelected = draft.workStates.includes(workState);
                return (
                  <Chip
                    key={workState}
                    label={WORK_STATE_LABEL[workState]}
                    size="small"
                    disabled={workStateDisabled}
                    variant={isSelected ? "filled" : "outlined"}
                    color={isSelected ? "primary" : "default"}
                    onClick={() => setDraft({ ...draft, workStates: toggle(draft.workStates, workState) })}
                  />
                );
              })}
            </Stack>
          </Stack>

          <Divider />

          <Stack direction="row" alignItems="center" justifyContent="space-between">
            <Typography variant="body2">Assigned to me</Typography>
            <Switch
              checked={draft.assignedToMe}
              onChange={(event) => setDraft({ ...draft, assignedToMe: event.target.checked })}
            />
          </Stack>

          <Stack direction="row" alignItems="center" justifyContent="space-between">
            <Typography variant="body2">Created by me</Typography>
            <Switch
              checked={draft.createdByMe}
              onChange={(event) => setDraft({ ...draft, createdByMe: event.target.checked })}
            />
          </Stack>
        </Stack>
      </DialogContent>

      <Divider />

      <DialogActions>
        <Button
          onClick={() => {
            setDraft(EMPTY_FILTERS);
            onApply(EMPTY_FILTERS);
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
