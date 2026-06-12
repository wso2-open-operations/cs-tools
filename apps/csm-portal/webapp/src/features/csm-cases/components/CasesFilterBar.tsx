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
  Autocomplete,
  Box,
  Button,
  Checkbox,
  Chip,
  Divider,
  FormControl,
  Grid,
  IconButton,
  InputAdornment,
  InputLabel,
  ListItemText,
  MenuItem,
  Paper,
  Select,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import {
  ChevronDown,
  ChevronUp,
  ListFilter,
  Search,
  X,
} from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX } from "react";
import type {
  CaseState,
  DashboardScope,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";
import { STATE_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import { isMockMode } from "@api/backend/client";

export type SlaFilter = "any" | "breached" | "at_risk";

/** Sentinel used inside `assignees` to mean "the current user". */
export const ASSIGNEE_ME_TOKEN = "@me";
/** Literal assignee name used in mock data for unassigned cases. */
export const ASSIGNEE_UNASSIGNED = "Unassigned";

/**
 * Filter state for the CSM cases list. `severities` / `states` are multi-select
 * arrays driven by fixed enums. `assignees` / `projects` / `products` are
 * free-form multi-selects driven by an Autocomplete with type-to-filter:
 * options are derived from the data currently in scope. The `assignees` list
 * accepts the sentinel `@me` (resolves against `assigneeIsMe`) and the literal
 * "Unassigned"; any other entries are matched against `case.assignee` by name.
 */
export interface CasesFilters {
  scope: DashboardScope;
  search: string;
  severities: Severity[];
  states: CaseState[];
  sla: SlaFilter;
  assignees: string[];
  projects: string[];
  products: string[];
}

/**
 * Lightweight user directory entry surfaced in the assignee picker. The
 * filter still stores assignees as bare name strings (plus the `@me` /
 * "Unassigned" sentinels) — `email` is only used for richer rendering.
 */
export interface AssigneeUser {
  name: string;
  email: string;
}

interface CasesFilterBarProps {
  filters: CasesFilters;
  onChange: (next: CasesFilters) => void;
  onReset: () => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
  /** Full user directory shown in the assignee picker. */
  availableAssigneeUsers: AssigneeUser[];
  /** Projects for the (id-based) project filter — value is the id, label the name. */
  availableProjects: { id: string; name: string }[];
  /** Product names seen in the current data. */
  availableProducts: string[];
}

const ALL_SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];
const PRIMARY_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "awaiting_info",
  "solution_proposed",
  "waiting_on_wso2",
  "reopened",
  "closed",
];
const SLA_OPTIONS: { value: SlaFilter; label: string }[] = [
  { value: "any", label: "Any SLA" },
  { value: "at_risk", label: "At risk" },
  { value: "breached", label: "Breached" },
];

/**
 * Count the filters that have non-default values. Search is included
 * separately so the badge correctly reflects "search + N filters".
 */
function countActiveFilters(f: CasesFilters): number {
  let n = 0;
  if (f.search.trim()) n += 1;
  if (f.severities.length) n += 1;
  if (f.states.length) n += 1;
  if (f.sla !== "any") n += 1;
  if (f.assignees.length) n += 1;
  if (f.projects.length) n += 1;
  if (f.products.length) n += 1;
  return n;
}

/** Pretty-print an assignee value (handle the @me sentinel). */
function assigneeLabel(value: string): string {
  return value === ASSIGNEE_ME_TOKEN ? "Me" : value;
}

interface MultiSelectFieldProps<T extends string> {
  id: string;
  label: string;
  values: T[];
  options: { value: T; label: string }[];
  onChange: (next: T[]) => void;
}

function MultiSelectField<T extends string>({
  id,
  label,
  values,
  options,
  onChange,
}: MultiSelectFieldProps<T>): JSX.Element {
  const handleChange = (event: SelectChangeEvent<string[]>): void => {
    const val = event.target.value;
    onChange((Array.isArray(val) ? val : [val]) as T[]);
  };
  return (
    <FormControl fullWidth size="small">
      <InputLabel id={`${id}-label`}>{label}</InputLabel>
      <Select
        multiple
        labelId={`${id}-label`}
        id={id}
        value={values as unknown as string[]}
        label={label}
        onChange={handleChange}
        renderValue={(selected) => {
          if (!Array.isArray(selected) || selected.length === 0) return "";
          const labels = selected.map(
            (v) => options.find((o) => o.value === v)?.label ?? v,
          );
          const text = labels.join(", ");
          if (labels.length === 1) return text;
          return (
            <Tooltip title={text} placement="top">
              <Box
                component="span"
                sx={{
                  display: "block",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {text}
              </Box>
            </Tooltip>
          );
        }}
      >
        {options.length === 0 ? (
          <MenuItem disabled value="">
            <em>No options</em>
          </MenuItem>
        ) : (
          options.map((opt) => (
            <MenuItem key={opt.value} value={opt.value} sx={{ py: 0.5 }}>
              <Checkbox
                size="small"
                checked={values.includes(opt.value)}
                sx={{ mr: 1, p: 0.25 }}
              />
              <ListItemText primary={opt.label} />
            </MenuItem>
          ))
        )}
      </Select>
    </FormControl>
  );
}

interface SingleSelectFieldProps<T extends string> {
  id: string;
  label: string;
  value: T;
  options: { value: T; label: string }[];
  onChange: (next: T) => void;
  disabled?: boolean;
}

function SingleSelectField<T extends string>({
  id,
  label,
  value,
  options,
  onChange,
  disabled,
}: SingleSelectFieldProps<T>): JSX.Element {
  return (
    <FormControl fullWidth size="small" disabled={disabled}>
      <InputLabel id={`${id}-label`}>{label}</InputLabel>
      <Select
        labelId={`${id}-label`}
        id={id}
        value={value as unknown as string}
        label={label}
        onChange={(e) => onChange(e.target.value as T)}
      >
        {options.map((opt) => (
          <MenuItem key={opt.value} value={opt.value}>
            {opt.label}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}

interface SearchableMultiSelectProps {
  id: string;
  label: string;
  placeholder?: string;
  values: string[];
  options: string[];
  /** Optional renderer for option labels (e.g. the @me sentinel → "Me"). */
  formatOption?: (value: string) => string;
  /** Optional secondary line shown beneath each option (e.g. email). */
  getOptionSecondary?: (value: string) => string | undefined;
  /** Optional filter against synthetic text (e.g. include email in the query). */
  getOptionSearchText?: (value: string) => string;
  onChange: (next: string[]) => void;
  disabled?: boolean;
}

/**
 * Multi-select autocomplete. Users type to filter the option list and
 * select multiple. Picked values render as removable chips inside the field.
 * Internally a thin wrapper over MUI Autocomplete so we keep oxygen-ui
 * theming via the re-exported components.
 */
function SearchableMultiSelect({
  id,
  label,
  placeholder,
  values,
  options,
  formatOption,
  getOptionSecondary,
  getOptionSearchText,
  onChange,
  disabled,
}: SearchableMultiSelectProps): JSX.Element {
  const format = formatOption ?? ((v: string) => v);
  const searchText = getOptionSearchText ?? format;
  return (
    <Autocomplete
      multiple
      size="small"
      disabled={disabled}
      id={id}
      options={options}
      value={values}
      onChange={(_event, next) => onChange(next as string[])}
      disableCloseOnSelect
      getOptionLabel={(opt) => format(opt as string)}
      isOptionEqualToValue={(opt, val) => opt === val}
      filterOptions={(opts, state) => {
        const q = state.inputValue.trim().toLowerCase();
        if (!q) return opts;
        return opts.filter((o) => searchText(o as string).toLowerCase().includes(q));
      }}
      renderTags={(value, getTagProps) =>
        value.map((option, index) => {
          const { key, ...tagProps } = getTagProps({ index });
          return (
            <Chip
              key={key}
              size="small"
              label={format(option as string)}
              {...tagProps}
            />
          );
        })
      }
      renderOption={(props, option, { selected }) => {
        const { key, ...liProps } = props as React.HTMLAttributes<HTMLLIElement> & {
          key: string;
        };
        const primary = format(option as string);
        const secondary = getOptionSecondary?.(option as string);
        return (
          <li key={key} {...liProps} style={{ paddingTop: 2, paddingBottom: 2 }}>
            <Checkbox
              size="small"
              checked={selected}
              sx={{ mr: 1, p: 0.25 }}
            />
            <ListItemText
              primary={primary}
              secondary={secondary}
              slotProps={{
                primary: { style: { fontSize: 13 } },
                secondary: { style: { fontSize: 11 } },
              }}
            />
          </li>
        );
      }}
      renderInput={(params) => (
        <TextField
          {...params}
          label={label}
          placeholder={values.length ? undefined : placeholder ?? "Type to search…"}
        />
      )}
    />
  );
}

export default function CasesFilterBar({
  filters,
  onChange,
  onReset,
  isFiltersOpen,
  onFiltersToggle,
  availableAssigneeUsers,
  availableProjects,
  availableProducts,
}: CasesFilterBarProps): JSX.Element {
  const activeCount = countActiveFilters(filters);
  const hasActive = activeCount > 0;

  // Scope (ABT), assignee, and SLA have no backend support yet (no assignee or
  // SLA fields), so disable them against the live backend — they only do
  // anything against seeded mock data.
  const beUnsupported = !isMockMode();
  const beUnsupportedReason =
    "Not available against the live backend yet (no assignee / SLA data).";

  const severityOptions = useMemo(
    () => ALL_SEVERITIES.map((s) => ({ value: s, label: s })),
    [],
  );
  const stateOptions = useMemo(
    () => PRIMARY_STATES.map((s) => ({ value: s, label: STATE_LABEL[s] })),
    [],
  );

  // Project filter is id-based: options are project ids, displayed by name.
  const projectIdOptions = useMemo(
    () => availableProjects.map((p) => p.id),
    [availableProjects],
  );
  const projectNameById = useMemo(
    () => new Map(availableProjects.map((p) => [p.id, p.name])),
    [availableProjects],
  );

  // Pin "@me" and "Unassigned" to the top of the assignee option list, then
  // every user from the directory (sorted by name, deduped). Pulling from
  // the user directory rather than only the owners present in loaded cases
  // means typing a name finds anyone, not just people who happen to own one
  // of the currently-listed cases.
  const assigneeOptions = useMemo(() => {
    const names = Array.from(
      new Set(availableAssigneeUsers.map((u) => u.name).filter(Boolean)),
    ).sort();
    return [ASSIGNEE_ME_TOKEN, ASSIGNEE_UNASSIGNED, ...names];
  }, [availableAssigneeUsers]);

  const emailByName = useMemo(() => {
    const m = new Map<string, string>();
    availableAssigneeUsers.forEach((u) => {
      if (u.name) m.set(u.name, u.email);
    });
    return m;
  }, [availableAssigneeUsers]);

  const assigneeSecondary = (value: string): string | undefined => {
    if (value === ASSIGNEE_ME_TOKEN || value === ASSIGNEE_UNASSIGNED) return undefined;
    return emailByName.get(value);
  };

  const assigneeSearchText = (value: string): string => {
    const label = assigneeLabel(value);
    const email = emailByName.get(value);
    return email ? `${label} ${email}` : label;
  };

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      {/* Scope + search + filters toggle. Scope buttons are CSM-specific
          (My ABT vs All customers) and live alongside search at the top. */}
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Tooltip title={beUnsupported ? beUnsupportedReason : ""}>
          <Box sx={{ display: "flex", gap: 0.75 }}>
            <Button
              size="small"
              variant={filters.scope === "my_abt" ? "contained" : "outlined"}
              color="primary"
              disabled={beUnsupported}
              onClick={() => onChange({ ...filters, scope: "my_abt" })}
            >
              My ABT
            </Button>
            <Button
              size="small"
              variant={
                filters.scope === "all_customers" ? "contained" : "outlined"
              }
              color="primary"
              disabled={beUnsupported}
              onClick={() => onChange({ ...filters, scope: "all_customers" })}
            >
              All customers
            </Button>
          </Box>
        </Tooltip>

        <Box sx={{ position: "relative", flex: 1, minWidth: 240 }}>
          <TextField
            fullWidth
            size="small"
            placeholder="Search by case #, subject, customer, project, assignee…"
            value={filters.search}
            onChange={(e) => onChange({ ...filters, search: e.target.value })}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
                endAdornment: filters.search ? (
                  <InputAdornment position="end">
                    <IconButton
                      size="small"
                      edge="end"
                      onClick={() => onChange({ ...filters, search: "" })}
                      aria-label="Clear search"
                    >
                      <X size={16} />
                    </IconButton>
                  </InputAdornment>
                ) : undefined,
              },
            }}
          />
        </Box>

        <Button
          variant="outlined"
          size="small"
          onClick={hasActive ? onReset : onFiltersToggle}
          startIcon={hasActive ? <X size={16} /> : <ListFilter size={16} />}
          endIcon={
            !hasActive &&
            (isFiltersOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />)
          }
        >
          {hasActive ? `Clear filters (${activeCount})` : "Filters"}
        </Button>
      </Box>

      {/* Collapsible filter grid. Severity / state stay as fixed multi-selects;
          assignee / project / product are type-to-search Autocompletes. */}
      {isFiltersOpen && (
        <>
          <Divider />
          <Grid container spacing={2} sx={{ mt: 0 }}>
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              <MultiSelectField
                id="cases-filter-severity"
                label="Severity"
                values={filters.severities}
                options={severityOptions}
                onChange={(next) => onChange({ ...filters, severities: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              <MultiSelectField
                id="cases-filter-state"
                label="State"
                values={filters.states}
                options={stateOptions}
                onChange={(next) => onChange({ ...filters, states: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              <Tooltip title={beUnsupported ? beUnsupportedReason : ""}>
                <Box>
                  <SingleSelectField
                    id="cases-filter-sla"
                    label="SLA"
                    value={filters.sla}
                    options={SLA_OPTIONS}
                    onChange={(next) => onChange({ ...filters, sla: next })}
                    disabled={beUnsupported}
                  />
                </Box>
              </Tooltip>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              <Tooltip title={beUnsupported ? beUnsupportedReason : ""}>
                <Box>
                  <SearchableMultiSelect
                    id="cases-filter-assignee"
                    label="Assignee"
                    placeholder="Search users…"
                    values={filters.assignees}
                    options={assigneeOptions}
                    formatOption={assigneeLabel}
                    getOptionSecondary={assigneeSecondary}
                    getOptionSearchText={assigneeSearchText}
                    onChange={(next) =>
                      onChange({ ...filters, assignees: next })
                    }
                    disabled={beUnsupported}
                  />
                </Box>
              </Tooltip>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              <SearchableMultiSelect
                id="cases-filter-project"
                label="Project"
                placeholder="Type a project…"
                values={filters.projects}
                options={projectIdOptions}
                formatOption={(id) => projectNameById.get(id) ?? id}
                getOptionSearchText={(id) => projectNameById.get(id) ?? id}
                onChange={(next) => onChange({ ...filters, projects: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              <SearchableMultiSelect
                id="cases-filter-product"
                label="Product"
                placeholder="Type a product…"
                values={filters.products}
                options={availableProducts}
                onChange={(next) => onChange({ ...filters, products: next })}
              />
            </Grid>
          </Grid>
          {activeCount > 0 && (
            <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
              <Typography variant="caption" color="text.secondary">
                {activeCount} {activeCount === 1 ? "filter" : "filters"} active
              </Typography>
            </Box>
          )}
        </>
      )}
    </Paper>
  );
}
