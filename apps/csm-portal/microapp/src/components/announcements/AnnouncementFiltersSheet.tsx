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

import { useEffect, useMemo, useState } from "react";
import {
  Autocomplete,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useQuery } from "@tanstack/react-query";
import { projects } from "@src/services/projects";
import type { CaseState, Project } from "@src/types";
import { STATE_LABELS } from "@components/support/config";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import { ANNOUNCEMENT_FILTER_STATES, EMPTY_ANNOUNCEMENT_FILTERS, type AnnouncementFilters } from "@utils/announcements";

// The Acrylic theme renders popup papers translucent, so a dropdown that opens
// over the dialog reads as see-through — force the opaque `background.default`.
const OPAQUE_POPUP = { sx: { backgroundColor: "background.default", backgroundImage: "none" } };

function toggleState(list: CaseState[], value: CaseState): CaseState[] {
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}

// Async, type-to-search multi-select of projects (server-side `projectIds`).
// Mirrors the support ProjectSelect (debounced 300ms); already-picked projects
// stay in the option list so their chips keep their labels.
function ProjectMultiSelect({ value, onChange }: { value: Project[]; onChange: (projects: Project[]) => void }) {
  const [input, setInput] = useState("");
  const debounced = useDebouncedValue(input, 300);
  const { data, isFetching } = useQuery(projects.search(debounced.trim()));

  const options = useMemo(() => {
    const results = data ?? [];
    return [...value, ...results.filter((r) => !value.some((v) => v.id === r.id))];
  }, [data, value]);

  return (
    <Autocomplete
      multiple
      size="small"
      options={options}
      value={value}
      loading={isFetching}
      getOptionLabel={(o) => o.name}
      isOptionEqualToValue={(a, b) => a.id === b.id}
      onChange={(_, next) => onChange(next)}
      onInputChange={(_, next) => setInput(next)}
      slotProps={{ paper: OPAQUE_POPUP }}
      renderInput={(params) => <TextField {...params} label="Project" size="small" />}
    />
  );
}

interface AnnouncementFiltersSheetProps {
  open: boolean;
  onClose: () => void;
  filters: AnnouncementFilters;
  onApply: (filters: AnnouncementFilters) => void;
}

// Mobile bottom-sheet filters for the announcements list: State (multi) + Project
// (async multi). Search lives in the always-visible bar on the page, not here.
export function AnnouncementFiltersSheet({ open, onClose, filters, onApply }: AnnouncementFiltersSheetProps) {
  const [draft, setDraft] = useState<AnnouncementFilters>(filters);

  // Re-seed the draft from the last-applied filters each time the sheet opens, so reopening it
  // doesn't show stale in-progress edits from a previous open that was dismissed without applying.
  useEffect(() => {
    if (open) setDraft(filters);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-seed on open, not on every filters identity change
  }, [open]);

  return (
    <Dialog
      open={open}
      onClose={onClose}
      fullWidth
      maxWidth="xs"
      slotProps={{ paper: { sx: { backgroundImage: "none", backgroundColor: "background.default" } } }}
    >
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
            <Typography variant="subtitle2">State</Typography>
            <Stack direction="row" gap={1} flexWrap="wrap">
              {ANNOUNCEMENT_FILTER_STATES.map((state) => {
                const isSelected = draft.states.includes(state);
                return (
                  <Chip
                    key={state}
                    label={STATE_LABELS[state] ?? state}
                    size="small"
                    aria-pressed={isSelected}
                    variant={isSelected ? "filled" : "outlined"}
                    color={isSelected ? "primary" : "default"}
                    onClick={() => setDraft({ ...draft, states: toggleState(draft.states, state) })}
                  />
                );
              })}
            </Stack>
          </Stack>

          <ProjectMultiSelect value={draft.projects} onChange={(next) => setDraft({ ...draft, projects: next })} />
        </Stack>
      </DialogContent>

      <Divider />

      <DialogActions>
        <Button
          onClick={() => {
            // Keep the current search text; only the sheet's own filters reset.
            const cleared = { ...EMPTY_ANNOUNCEMENT_FILTERS, search: draft.search };
            setDraft(cleared);
            onApply(cleared);
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
