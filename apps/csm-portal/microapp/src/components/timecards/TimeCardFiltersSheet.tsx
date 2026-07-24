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
  AdapterDateFns,
  Autocomplete,
  Button,
  Chip,
  DatePickers,
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
import { format, parse } from "date-fns";
import { projects } from "@src/services/projects";
import type { Project, TimeCardState } from "@src/types";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import {
  EMPTY_TIMECARD_FILTERS,
  TIME_CARD_FILTER_STATES,
  TIME_CARD_STATE_META,
  type EngineerOption,
  type TimeCardFilters,
} from "@utils/timecard";

const { LocalizationProvider, DatePicker } = DatePickers;

const ISO_DATE = "yyyy-MM-dd";

// The Acrylic theme renders popup papers translucent, so a picker/dropdown that
// opens over the dialog reads as see-through. Force the opaque `background.default`
// and drop the elevation gradient so popups are solid and readable.
const OPAQUE_POPUP = { sx: { backgroundColor: "background.default", backgroundImage: "none" } };

// The filter model carries dates as YYYY-MM-DD strings (that's what the search
// endpoint wants); the picker works in Date objects — convert at the boundary.
function fromIsoDate(iso: string): Date | null {
  if (!iso) return null;
  const d = parse(iso, ISO_DATE, new Date());
  return Number.isNaN(d.getTime()) ? null : d;
}

function toIsoDate(date: Date | null): string {
  return date && !Number.isNaN(date.getTime()) ? format(date, ISO_DATE) : "";
}

function toggleState(list: TimeCardState[], value: TimeCardState): TimeCardState[] {
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}

// Async, type-to-search multi-select of projects (server-side `projectIds`
// filter). Mirrors the support ProjectSelect: debounced 300ms, top matches;
// already-picked projects are kept in the option list so their chips keep their
// labels even after a later search no longer returns them.
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

interface TimeCardFiltersSheetProps {
  open: boolean;
  onClose: () => void;
  filters: TimeCardFilters;
  onApply: (filters: TimeCardFilters) => void;
  /** Hidden on the Approvals queue, where every card is already `submitted`. */
  showStateFilter?: boolean;
  /** Hidden on the My-sheets tab (all cards are the signed-in user's own). */
  showEngineerFilter?: boolean;
  /** Case numbers to offer for the (client-side) work-item filter — derived from
   * the currently loaded cards. */
  workItemOptions: string[];
  /** Engineers to offer for the (client-side) engineer filter. */
  engineerOptions: EngineerOption[];
}

// Mobile bottom-sheet equivalent of the webapp's time-card FilterBar. Project is
// server-side; work item and engineer are client-side over the loaded cards; state
// (client-side) and a date range (server-side) round it out. Which fields show
// depends on the tab — see the webapp's per-tab filter sets.
export function TimeCardFiltersSheet({
  open,
  onClose,
  filters,
  onApply,
  showStateFilter = true,
  showEngineerFilter = false,
  workItemOptions,
  engineerOptions,
}: TimeCardFiltersSheetProps) {
  const [draft, setDraft] = useState<TimeCardFilters>(filters);

  // Re-seed the draft from the last-applied filters each time the sheet opens, so reopening it
  // doesn't show stale in-progress edits from a previous open that was dismissed without applying.
  useEffect(() => {
    if (open) setDraft(filters);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-seed on open, not on every filters identity change
  }, [open]);

  // Keep any already-selected work item in the option list so its chip renders even
  // if it's no longer among the loaded cards (avoids MUI's "value not in options" warning).
  const workItemOpts = useMemo(
    () => [...new Set([...workItemOptions, ...draft.workItems])].sort(),
    [workItemOptions, draft.workItems],
  );
  const engineerValue = engineerOptions.filter((e) => draft.engineers.includes(e.id));

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
          <ProjectMultiSelect value={draft.projects} onChange={(next) => setDraft({ ...draft, projects: next })} />

          <Autocomplete
            multiple
            size="small"
            options={workItemOpts}
            value={draft.workItems}
            onChange={(_, next) => setDraft({ ...draft, workItems: next })}
            slotProps={{ paper: OPAQUE_POPUP }}
            renderInput={(params) => <TextField {...params} label="Work item" size="small" />}
          />

          {showEngineerFilter && (
            <Autocomplete
              multiple
              size="small"
              options={engineerOptions}
              value={engineerValue}
              getOptionLabel={(o) => o.name}
              isOptionEqualToValue={(a, b) => a.id === b.id}
              onChange={(_, next) => setDraft({ ...draft, engineers: next.map((e) => e.id) })}
              slotProps={{ paper: OPAQUE_POPUP }}
              renderInput={(params) => <TextField {...params} label="Engineer" size="small" />}
            />
          )}

          {showStateFilter && (
            <Stack gap={1}>
              <Typography variant="subtitle2">State</Typography>
              <Stack direction="row" gap={1} flexWrap="wrap">
                {TIME_CARD_FILTER_STATES.map((state) => {
                  const isSelected = draft.states.includes(state);
                  return (
                    <Chip
                      key={state}
                      label={TIME_CARD_STATE_META[state].label}
                      size="small"
                      variant={isSelected ? "filled" : "outlined"}
                      color={isSelected ? "primary" : "default"}
                      onClick={() => setDraft({ ...draft, states: toggleState(draft.states, state) })}
                    />
                  );
                })}
              </Stack>
            </Stack>
          )}

          <Stack gap={1}>
            <Typography variant="subtitle2">Date range</Typography>
            <LocalizationProvider dateAdapter={AdapterDateFns}>
              <Stack direction="row" gap={1}>
                <DatePicker
                  label="From"
                  value={fromIsoDate(draft.from)}
                  onChange={(date: Date | null) => setDraft({ ...draft, from: toIsoDate(date) })}
                  maxDate={fromIsoDate(draft.to) ?? undefined}
                  slotProps={{
                    textField: { size: "small", fullWidth: true },
                    field: { clearable: true },
                    desktopPaper: OPAQUE_POPUP,
                    mobilePaper: OPAQUE_POPUP,
                  }}
                />
                <DatePicker
                  label="To"
                  value={fromIsoDate(draft.to)}
                  onChange={(date: Date | null) => setDraft({ ...draft, to: toIsoDate(date) })}
                  minDate={fromIsoDate(draft.from) ?? undefined}
                  slotProps={{
                    textField: { size: "small", fullWidth: true },
                    field: { clearable: true },
                    desktopPaper: OPAQUE_POPUP,
                    mobilePaper: OPAQUE_POPUP,
                  }}
                />
              </Stack>
            </LocalizationProvider>
          </Stack>
        </Stack>
      </DialogContent>

      <Divider />

      <DialogActions>
        <Button
          onClick={() => {
            setDraft(EMPTY_TIMECARD_FILTERS);
            onApply(EMPTY_TIMECARD_FILTERS);
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
