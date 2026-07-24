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
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useQuery } from "@tanstack/react-query";
import { projects } from "@src/services/projects";
import { adminUsers } from "@src/services/adminUsers";
import { products } from "@src/services/products";
import type { Project } from "@src/types";
import { ALL_WORK_STATES, FILTERABLE_STATES, STATE_LABELS, WORK_STATE_LABEL } from "@components/support/config";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import {
  ALL_ENGAGEMENT_TYPES,
  EMPTY_ENGAGEMENT_FILTERS,
  ENGAGEMENT_TYPE_LABEL,
  type EngagementAssignee,
  type EngagementFilters,
} from "@utils/engagements";

// The Acrylic theme renders popup papers translucent, so a dropdown that opens
// over the dialog reads as see-through — force the opaque `background.default`.
const OPAQUE_POPUP = { sx: { backgroundColor: "background.default", backgroundImage: "none" } };

function toggle<T>(list: T[], value: T): T[] {
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}

// Async, type-to-search multi-select of projects (server-side `projectIds`).
// Mirrors the support ProjectSelect / announcements' own ProjectMultiSelect
// (debounced 300ms); already-picked projects stay in the option list so their
// chips keep their labels.
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

// Async, type-to-search multi-select of engineers (server-side `assignedUserIds`).
// Reuses adminUsers.search's internal-roles scope (LogTimeCardDialog's approver
// picker) — "engineer" and "eligible approver" are the same directory slice.
// Requires at least one typed character, same as that picker.
function AssigneeMultiSelect({
  value,
  onChange,
}: {
  value: EngagementAssignee[];
  onChange: (assignees: EngagementAssignee[]) => void;
}) {
  const [input, setInput] = useState("");
  const debounced = useDebouncedValue(input, 300);
  const { data, isFetching } = useQuery(adminUsers.search(debounced.trim()));

  const options = useMemo(() => {
    const results = data?.users ?? [];
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
      onChange={(_, next) => onChange(next.map((o) => ({ id: o.id, name: o.name })))}
      onInputChange={(_, next) => setInput(next)}
      slotProps={{ paper: OPAQUE_POPUP }}
      renderInput={(params) => <TextField {...params} label="Assignee" size="small" placeholder="Search engineers…" />}
    />
  );
}

// Products are a bounded catalogue, so the distinct family names are fetched
// once (products.names()) and filtered locally as the user types — mirrors the
// webapp's ProductNameMultiSelect/useProductNameOptions.
function ProductMultiSelect({ value, onChange }: { value: string[]; onChange: (names: string[]) => void }) {
  const { data, isFetching } = useQuery(products.names());

  const options = useMemo(() => {
    const merged = new Set<string>(data ?? []);
    value.forEach((v) => merged.add(v));
    return [...merged].sort((a, b) => a.localeCompare(b));
  }, [data, value]);

  return (
    <Autocomplete
      multiple
      size="small"
      options={options}
      value={value}
      loading={isFetching}
      onChange={(_, next) => onChange(next)}
      slotProps={{ paper: OPAQUE_POPUP }}
      renderInput={(params) => <TextField {...params} label="Product" size="small" />}
    />
  );
}

interface EngagementFiltersSheetProps {
  open: boolean;
  onClose: () => void;
  filters: EngagementFilters;
  onApply: (filters: EngagementFilters) => void;
}

// Mobile bottom-sheet filters for the engagements list: State, Work state,
// Engagement type (all chip multi-selects) + Assignee, Project, Product
// (async multi-selects). Search lives in the always-visible bar on the page,
// not here. Mirrors the webapp's CasesFilterBar for a view locked to
// `caseTypes: ["engagement"]` with `showEngagementTypeFilter` — severity and
// case type are left out entirely, same as there (severity's a "case"-only
// concept, and case type is fixed to "engagement" here).
export function EngagementFiltersSheet({ open, onClose, filters, onApply }: EngagementFiltersSheetProps) {
  const [draft, setDraft] = useState<EngagementFilters>(filters);

  // Re-seed the draft from the last-applied filters each time the sheet opens, so reopening it
  // doesn't show stale in-progress edits from a previous open that was dismissed without applying.
  useEffect(() => {
    if (open) setDraft(filters);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-seed on open, not on every filters identity change
  }, [open]);

  const workInProgressSelected = draft.states.includes("work_in_progress");

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
              {FILTERABLE_STATES.map((state) => {
                const isSelected = draft.states.includes(state);
                return (
                  <Chip
                    key={state}
                    label={STATE_LABELS[state] ?? state}
                    size="small"
                    aria-pressed={isSelected}
                    variant={isSelected ? "filled" : "outlined"}
                    color={isSelected ? "primary" : "default"}
                    onClick={() => {
                      const states = toggle(draft.states, state);
                      // Work sub-state only applies to work_in_progress cases, so drop any
                      // selected work states when that state leaves the filter — keeps a
                      // stale, inert work-state selection from lingering.
                      setDraft({
                        ...draft,
                        states,
                        workStates: states.includes("work_in_progress") ? draft.workStates : [],
                      });
                    }}
                  />
                );
              })}
            </Stack>
          </Stack>

          <Stack gap={1}>
            <Typography variant="subtitle2">Work state</Typography>
            <Stack direction="row" gap={1} flexWrap="wrap">
              {ALL_WORK_STATES.map((workState) => {
                const isSelected = draft.workStates.includes(workState);
                const chip = (
                  <Chip
                    label={WORK_STATE_LABEL[workState]}
                    size="small"
                    aria-pressed={isSelected}
                    variant={isSelected ? "filled" : "outlined"}
                    color={isSelected ? "primary" : "default"}
                    disabled={!workInProgressSelected}
                    onClick={() => setDraft({ ...draft, workStates: toggle(draft.workStates, workState) })}
                  />
                );
                // Disabled MUI chips don't fire pointer events, so the tooltip needs a
                // span wrapper to still show on hover/focus.
                return workInProgressSelected ? (
                  <span key={workState}>{chip}</span>
                ) : (
                  <Tooltip key={workState} title={`Select "${STATE_LABELS.work_in_progress}" to filter by work state`}>
                    <span>{chip}</span>
                  </Tooltip>
                );
              })}
            </Stack>
          </Stack>

          <Stack gap={1}>
            <Typography variant="subtitle2">Engagement type</Typography>
            <Stack direction="row" gap={1} flexWrap="wrap">
              {ALL_ENGAGEMENT_TYPES.map((type) => {
                const isSelected = draft.engagementTypes.includes(type);
                return (
                  <Chip
                    key={type}
                    label={ENGAGEMENT_TYPE_LABEL[type]}
                    size="small"
                    aria-pressed={isSelected}
                    variant={isSelected ? "filled" : "outlined"}
                    color={isSelected ? "primary" : "default"}
                    onClick={() => setDraft({ ...draft, engagementTypes: toggle(draft.engagementTypes, type) })}
                  />
                );
              })}
            </Stack>
          </Stack>

          <AssigneeMultiSelect value={draft.assignees} onChange={(next) => setDraft({ ...draft, assignees: next })} />

          <ProjectMultiSelect value={draft.projects} onChange={(next) => setDraft({ ...draft, projects: next })} />

          <ProductMultiSelect
            value={draft.productNames}
            onChange={(next) => setDraft({ ...draft, productNames: next })}
          />
        </Stack>
      </DialogContent>

      <Divider />

      <DialogActions>
        <Button
          onClick={() => {
            // Keep the current search text; only the sheet's own filters reset.
            const cleared = { ...EMPTY_ENGAGEMENT_FILTERS, search: draft.search };
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
