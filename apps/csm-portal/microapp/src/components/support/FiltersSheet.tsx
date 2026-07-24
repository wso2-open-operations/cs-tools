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
import { ALL_SEVERITIES, ALL_WORK_STATES, SEVERITY_LABELS, WORK_STATE_LABEL } from "./config";
import { EMPTY_FILTERS, type CaseFilters } from "./filters";

function toggle<T>(list: T[], value: T): T[] {
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}

interface ToggleChipGroupProps<T extends string> {
  options: T[];
  selected: T[];
  labels: Record<T, string>;
  onToggle: (value: T) => void;
  disabled?: boolean;
}

function ToggleChipGroup<T extends string>({ options, selected, labels, onToggle, disabled }: ToggleChipGroupProps<T>) {
  return (
    <Stack direction="row" gap={1} flexWrap="wrap">
      {options.map((option) => {
        const isSelected = selected.includes(option);
        return (
          <Chip
            key={option}
            label={labels[option]}
            size="small"
            disabled={disabled}
            variant={isSelected ? "filled" : "outlined"}
            color={isSelected ? "primary" : "default"}
            onClick={() => onToggle(option)}
          />
        );
      })}
    </Stack>
  );
}

interface FiltersSheetProps {
  open: boolean;
  onClose: () => void;
  filters: CaseFilters;
  onApply: (filters: CaseFilters) => void;
}

// Mobile bottom-sheet-style equivalent of the webapp's CasesFilterBar collapsible panel: severity
// is picked from the same fixed enum the webapp filters on
// (apps/csm-portal/webapp/src/features/csm-cases/components/CasesFilterBar.tsx — ALL_SEVERITIES).
// State lives in its own tab row above the list (not here) — work state is only enabled once
// "Work in progress" is the selected state (same invariant the webapp enforces). Assignee/project/
// product filters are deliberately scoped out here in favor of simple "Assigned to me" / "Created
// by me" toggles — the webapp's full async assignee/project search pickers aren't a good fit for a
// mobile filter sheet.
export function FiltersSheet({ open, onClose, filters, onApply }: FiltersSheetProps) {
  const [draft, setDraft] = useState<CaseFilters>(filters);

  // Re-seed the draft from the last-applied filters each time the sheet opens, so reopening it
  // doesn't show stale in-progress edits from a previous open that was dismissed without applying.
  useEffect(() => {
    if (open) setDraft(filters);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-seed on open, not on every filters identity change
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
            <Typography variant="subtitle2">Severity</Typography>
            <ToggleChipGroup
              options={ALL_SEVERITIES}
              selected={draft.severities}
              labels={SEVERITY_LABELS}
              onToggle={(severity) => setDraft({ ...draft, severities: toggle(draft.severities, severity) })}
            />
          </Stack>

          <Stack gap={1}>
            <Typography variant="subtitle2" color={workStateDisabled ? "text.disabled" : "text.primary"}>
              Work state
            </Typography>
            <ToggleChipGroup
              options={ALL_WORK_STATES}
              selected={draft.workStates}
              labels={WORK_STATE_LABEL}
              disabled={workStateDisabled}
              onToggle={(workState) => setDraft({ ...draft, workStates: toggle(draft.workStates, workState) })}
            />
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
