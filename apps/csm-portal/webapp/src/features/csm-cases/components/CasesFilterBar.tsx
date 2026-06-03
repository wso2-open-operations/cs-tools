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
  Box,
  Button,
  Checkbox,
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

export type SlaFilter = "any" | "breached" | "at_risk";
export type OwnerFilter = "anyone" | "me" | "unassigned";

/**
 * Filter state for the CSM cases list. `severities` / `states` / `customers` /
 * `projects` are multi-select arrays; `sla` / `owner` stay single-select
 * because the option sets are mutually exclusive (a case is either at-risk
 * or breached, not both).
 */
export interface CasesFilters {
  scope: DashboardScope;
  search: string;
  severities: Severity[];
  states: CaseState[];
  sla: SlaFilter;
  owner: OwnerFilter;
  customers: string[];
  projects: string[];
}

interface CasesFilterBarProps {
  filters: CasesFilters;
  onChange: (next: CasesFilters) => void;
  onReset: () => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
  /** All values seen in the current data — used to populate multi-select option lists. */
  availableCustomers: string[];
  availableProjects: string[];
}

const ALL_SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];
const PRIMARY_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "awaiting_info",
  "solution_proposed",
  "waiting_on_wso2",
  "reopen",
  "closed",
];
const SLA_OPTIONS: { value: SlaFilter; label: string }[] = [
  { value: "any", label: "Any SLA" },
  { value: "at_risk", label: "At risk" },
  { value: "breached", label: "Breached" },
];
const OWNER_OPTIONS: { value: OwnerFilter; label: string }[] = [
  { value: "anyone", label: "Anyone" },
  { value: "me", label: "Me" },
  { value: "unassigned", label: "Unassigned" },
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
  if (f.owner !== "anyone") n += 1;
  if (f.customers.length) n += 1;
  if (f.projects.length) n += 1;
  return n;
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
}

function SingleSelectField<T extends string>({
  id,
  label,
  value,
  options,
  onChange,
}: SingleSelectFieldProps<T>): JSX.Element {
  return (
    <FormControl fullWidth size="small">
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

export default function CasesFilterBar({
  filters,
  onChange,
  onReset,
  isFiltersOpen,
  onFiltersToggle,
  availableCustomers,
  availableProjects,
}: CasesFilterBarProps): JSX.Element {
  const activeCount = countActiveFilters(filters);
  const hasActive = activeCount > 0;

  const severityOptions = useMemo(
    () => ALL_SEVERITIES.map((s) => ({ value: s, label: s })),
    [],
  );
  const stateOptions = useMemo(
    () => PRIMARY_STATES.map((s) => ({ value: s, label: STATE_LABEL[s] })),
    [],
  );
  const customerOptions = useMemo(
    () => availableCustomers.map((c) => ({ value: c, label: c })),
    [availableCustomers],
  );
  const projectOptions = useMemo(
    () => availableProjects.map((p) => ({ value: p, label: p })),
    [availableProjects],
  );

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      {/* Scope + search + filters toggle. Scope buttons are CSM-specific
          (My ABT vs All customers) and live alongside search at the top. */}
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Box sx={{ display: "flex", gap: 0.75 }}>
          <Button
            size="small"
            variant={filters.scope === "my_abt" ? "contained" : "outlined"}
            color="primary"
            onClick={() => onChange({ ...filters, scope: "my_abt" })}
          >
            My ABT
          </Button>
          <Button
            size="small"
            variant={filters.scope === "all_customers" ? "contained" : "outlined"}
            color="primary"
            onClick={() => onChange({ ...filters, scope: "all_customers" })}
          >
            All customers
          </Button>
        </Box>

        <Box sx={{ position: "relative", flex: 1, minWidth: 240 }}>
          <TextField
            fullWidth
            size="small"
            placeholder="Search by case #, subject, customer, project, owner…"
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

      {/* Collapsible filter grid. Multi-select dropdowns use checkbox menu
          items so multiple values can be picked per field. */}
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
              <SingleSelectField
                id="cases-filter-sla"
                label="SLA"
                value={filters.sla}
                options={SLA_OPTIONS}
                onChange={(next) => onChange({ ...filters, sla: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              <SingleSelectField
                id="cases-filter-owner"
                label="Owner"
                value={filters.owner}
                options={OWNER_OPTIONS}
                onChange={(next) => onChange({ ...filters, owner: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              <MultiSelectField
                id="cases-filter-customer"
                label="Customer"
                values={filters.customers}
                options={customerOptions}
                onChange={(next) => onChange({ ...filters, customers: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              <MultiSelectField
                id="cases-filter-project"
                label="Project"
                values={filters.projects}
                options={projectOptions}
                onChange={(next) => onChange({ ...filters, projects: next })}
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
