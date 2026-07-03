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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  Grid,
  IconButton,
  InputAdornment,
  InputLabel,
  ListItemIcon,
  ListItemText,
  ListSubheader,
  Menu,
  MenuItem,
  Paper,
  Select,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import {
  Bookmark,
  BookmarkPlus,
  Check,
  ChevronDown,
  ChevronUp,
  ListFilter,
  Search,
  Trash2,
  X,
} from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import type {
  CaseState,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";
import { STATE_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import {
  countActiveFilters,
  readCasesFiltersFromUrl,
  writeCasesFiltersToUrl,
} from "@features/csm-cases/utils/casesFiltersUrl";
import {
  deleteFilterView,
  saveFilterView,
  SUGGESTED_FILTER_VIEWS,
  useSavedFilterViews,
} from "@features/csm-cases/utils/savedFilterViews";
import type {
  BeCaseType,
  BeCaseWorkState,
  BeEngagementType,
} from "@api/backend/types";
import {
  ALL_CASE_TYPES,
  CASE_TYPE_LABEL,
} from "@features/csm-cases/utils/caseType";
import AsyncProjectMultiSelect from "@features/csm-cases/components/AsyncProjectMultiSelect";
import AsyncAssigneeMultiSelect from "@features/csm-cases/components/AsyncAssigneeMultiSelect";


/**
 * Filter state for the CSM cases list. `severities` / `states` / `caseTypes`
 * are multi-select arrays driven by fixed enums; `projects` is an id-based
 * type-to-search multi-select. `assignees` holds engineer **emails** plus the
 * sentinel `@me`; `useGetCsmCases` resolves these to the engineer UUIDs that
 * `/cases/search` filters on. All are pushed into the `/cases/search` payload
 * server-side.
 */
export interface CasesFilters {
  search: string;
  severities: Severity[];
  states: CaseState[];
  /** Case-type filter (BE `typeKeys`). Empty = all types. */
  caseTypes: BeCaseType[];
  /** Engineer emails (+ the `@me` sentinel) to filter by assigned engineer. */
  assignees: string[];
  /** Work sub-state filter; only meaningful when `states` includes `work_in_progress`. */
  workStates: BeCaseWorkState[];
  projects: string[];
  /** Engagement sub-type filter; only meaningful when `caseTypes` is locked to `engagement`. */
  engagementTypes: BeEngagementType[];
}

/**
 * Lightweight user-directory entry surfaced in the assignee picker. The filter
 * stores the `email` as the value; the `name` is shown as the option label.
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
  /** Hide the case-type control when the surrounding view locks the type. */
  hideTypeFilter?: boolean;
  /** Hide the project control when the surrounding view is project-scoped. */
  hideProjectFilter?: boolean;
  /** Show the engagement-type multi-select (only relevant when type is locked to engagement). */
  showEngagementTypeFilter?: boolean;
}

const ALL_ENGAGEMENT_TYPES: BeEngagementType[] = [
  "migration",
  "consultancy",
  "new_feature_improvement",
  "follow_up",
  "onboarding",
];

const ENGAGEMENT_TYPE_LABEL: Record<BeEngagementType, string> = {
  migration: "Migration",
  consultancy: "Consultancy",
  new_feature_improvement: "New feature / improvement",
  follow_up: "Follow-up",
  onboarding: "Onboarding",
};

const ALL_WORK_STATES: BeCaseWorkState[] = ["ongoing", "paused"];
const WORK_STATE_LABEL: Record<BeCaseWorkState, string> = {
  ongoing: "Ongoing",
  paused: "Paused",
};

const ALL_SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];
const PRIMARY_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "awaiting_info",
  "solution_proposed",
  "waiting_on_wso2",
  "closed",
];

interface MultiSelectFieldProps<T extends string> {
  id: string;
  label: string;
  values: T[];
  options: { value: T; label: string }[];
  onChange: (next: T[]) => void;
  disabled?: boolean;
}

function MultiSelectField<T extends string>({
  id,
  label,
  values,
  options,
  onChange,
  disabled,
}: MultiSelectFieldProps<T>): JSX.Element {
  const handleChange = (event: SelectChangeEvent<string[]>): void => {
    const val = event.target.value;
    onChange((Array.isArray(val) ? val : [val]) as T[]);
  };
  return (
    <FormControl fullWidth size="small" disabled={disabled}>
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

export default function CasesFilterBar({
  filters,
  onChange,
  onReset,
  isFiltersOpen,
  onFiltersToggle,
  availableAssigneeUsers,
  availableProjects,
  hideTypeFilter = false,
  hideProjectFilter = false,
  showEngagementTypeFilter = false,
}: CasesFilterBarProps): JSX.Element {
  const activeCount = countActiveFilters(filters);
  const hasActive = activeCount > 0;

  // ── Saved views ──────────────────────────────────────────────────────────
  // A saved view is just a name pointing at a serialized filter query string;
  // applying one feeds the parsed filters back through onChange (which the page
  // writes to the URL), so the URL stays the source of truth.
  const savedViews = useSavedFilterViews();
  const currentQs = writeCasesFiltersToUrl(filters).toString();
  // Canonicalize a query string (normalize comma encoding, param order, and
  // drop unknown params) so the "active view" check matches regardless of how a
  // view's qs was authored — suggested presets use literal commas, while
  // writeCasesFiltersToUrl emits %2C.
  const canonicalQs = (qs: string): string =>
    writeCasesFiltersToUrl(
      readCasesFiltersFromUrl(new URLSearchParams(qs)),
    ).toString();
  const currentCanonical = canonicalQs(currentQs);
  const isActiveView = (qs: string): boolean => canonicalQs(qs) === currentCanonical;
  const [savedAnchor, setSavedAnchor] = useState<HTMLElement | null>(null);
  const [saveDialogOpen, setSaveDialogOpen] = useState(false);
  const [newViewName, setNewViewName] = useState("");

  const applyView = (qs: string): void => {
    setSavedAnchor(null);
    onChange(readCasesFiltersFromUrl(new URLSearchParams(qs)));
  };

  const handleSaveView = (): void => {
    if (!newViewName.trim()) return;
    saveFilterView(newViewName, currentQs);
    setNewViewName("");
    setSaveDialogOpen(false);
    setSavedAnchor(null);
  };

  const severityOptions = useMemo(
    () => ALL_SEVERITIES.map((s) => ({ value: s, label: s })),
    [],
  );
  const stateOptions = useMemo(
    () => PRIMARY_STATES.map((s) => ({ value: s, label: STATE_LABEL[s] })),
    [],
  );
  const workStateOptions = useMemo(
    () => ALL_WORK_STATES.map((s) => ({ value: s, label: WORK_STATE_LABEL[s] })),
    [],
  );
  const caseTypeOptions = useMemo(
    () => ALL_CASE_TYPES.map((t) => ({ value: t, label: CASE_TYPE_LABEL[t] })),
    [],
  );
  const engagementTypeOptions = useMemo(
    () => ALL_ENGAGEMENT_TYPES.map((t) => ({ value: t, label: ENGAGEMENT_TYPE_LABEL[t] })),
    [],
  );

  // Project filter loads the first page of projects on open and pages through
  // the rest on scroll (and narrows as you type) rather than loading the whole
  // catalogue at once. `availableProjects` (projects on the loaded cases) only
  // seeds chip labels for already-selected ids before any page loads.
  const projectNameSeed = useMemo(
    () => new Map(availableProjects.map((p) => [p.id, p.name])),
    [availableProjects],
  );

  // The assignee filter searches the user directory from the backend as you
  // type (see AsyncAssigneeMultiSelect), so anyone is findable — not just the
  // first page of users. `availableAssigneeUsers` (the directory prefetch /
  // owners on loaded cases) only seeds chip labels for already-selected emails
  // before any search has run.
  const assigneeNameSeed = useMemo(() => {
    const m = new Map<string, string>();
    availableAssigneeUsers.forEach((u) => {
      if (u.email) m.set(u.email, u.name);
    });
    return m;
  }, [availableAssigneeUsers]);

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      {/* Search + saved views + filters toggle. */}
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
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
          color="inherit"
          onClick={(e) => setSavedAnchor(e.currentTarget)}
          startIcon={<Bookmark size={16} />}
          endIcon={<ChevronDown size={16} />}
          aria-haspopup="true"
          aria-expanded={Boolean(savedAnchor)}
        >
          Saved views
        </Button>
        <Menu
          anchorEl={savedAnchor}
          open={Boolean(savedAnchor)}
          onClose={() => setSavedAnchor(null)}
          anchorOrigin={{ vertical: "bottom", horizontal: "left" }}
        >
          <MenuItem
            onClick={() => {
              setSavedAnchor(null);
              setSaveDialogOpen(true);
            }}
          >
            <ListItemIcon>
              <BookmarkPlus size={16} />
            </ListItemIcon>
            <ListItemText primary="Save current view…" />
          </MenuItem>
          <Divider />
          <ListSubheader sx={{ lineHeight: "32px" }}>Suggested</ListSubheader>
          {SUGGESTED_FILTER_VIEWS.map((v) => (
            <MenuItem
              key={`suggested-${v.name}`}
              selected={isActiveView(v.qs)}
              onClick={() => applyView(v.qs)}
            >
              <ListItemIcon>
                {isActiveView(v.qs) ? <Check size={16} /> : null}
              </ListItemIcon>
              <ListItemText primary={v.name} />
            </MenuItem>
          ))}
          <Divider />
          <ListSubheader sx={{ lineHeight: "32px" }}>Saved</ListSubheader>
          {savedViews.length === 0 ? (
            <MenuItem disabled>
              <ListItemText
                primary="No saved views yet"
                slotProps={{ primary: { variant: "body2" } }}
              />
            </MenuItem>
          ) : (
            savedViews.map((v) => (
              <MenuItem
                key={`saved-${v.name}`}
                selected={isActiveView(v.qs)}
                onClick={() => applyView(v.qs)}
              >
                <ListItemIcon>
                  {isActiveView(v.qs) ? <Check size={16} /> : null}
                </ListItemIcon>
                <ListItemText primary={v.name} />
                <IconButton
                  size="small"
                  edge="end"
                  aria-label={`Delete saved view ${v.name}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    deleteFilterView(v.name);
                  }}
                  sx={{ ml: 1 }}
                >
                  <Trash2 size={15} />
                </IconButton>
              </MenuItem>
            ))
          )}
        </Menu>

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

      <Dialog
        open={saveDialogOpen}
        onClose={() => setSaveDialogOpen(false)}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>Save current view</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            size="small"
            margin="dense"
            label="View name"
            placeholder="e.g. My open S1/S2"
            value={newViewName}
            onChange={(e) => setNewViewName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                handleSaveView();
              }
            }}
            helperText={
              activeCount === 0
                ? "Tip: no filters are active — this view will show all cases."
                : `Captures the ${activeCount} active filter${activeCount === 1 ? "" : "s"}.`
            }
          />
        </DialogContent>
        <DialogActions>
          <Button color="inherit" onClick={() => setSaveDialogOpen(false)}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleSaveView}
            disabled={!newViewName.trim()}
          >
            Save
          </Button>
        </DialogActions>
      </Dialog>

      {/* Collapsible filter grid. Severity / state / case type are fixed
          multi-selects; assignee / project are type-to-search Autocompletes. */}
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
                // Work sub-state only applies to `work_in_progress` cases, so
                // drop any selected work states when that state leaves the
                // filter — keeps shared URLs / saved views from carrying an
                // inert work-state selection.
                onChange={(next) =>
                  onChange({
                    ...filters,
                    states: next,
                    workStates: next.includes("work_in_progress")
                      ? filters.workStates
                      : [],
                  })
                }
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              {/* Only meaningful for `work_in_progress` cases (ongoing/paused
                  are sub-states of it); disabled until that state is filtered
                  in, so the control can't add an inert filter. */}
              <MultiSelectField
                id="cases-filter-work-state"
                label="Work state"
                values={filters.workStates}
                options={workStateOptions}
                onChange={(next) => onChange({ ...filters, workStates: next })}
                disabled={!filters.states.includes("work_in_progress")}
              />
            </Grid>
            {showEngagementTypeFilter && (
              <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
                <MultiSelectField
                  id="cases-filter-engagement-type"
                  label="Engagement type"
                  values={filters.engagementTypes}
                  options={engagementTypeOptions}
                  onChange={(next) => onChange({ ...filters, engagementTypes: next })}
                />
              </Grid>
            )}
            {!hideTypeFilter && (
              <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
                <MultiSelectField
                  id="cases-filter-type"
                  label="Case type"
                  values={filters.caseTypes}
                  options={caseTypeOptions}
                  onChange={(next) => onChange({ ...filters, caseTypes: next })}
                />
              </Grid>
            )}
            <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
              {/* Email/`@me`-based picker; `useGetCsmCases` resolves the
                  selection to the UUIDs `/cases/search` expects (`@me` via the
                  app-wide current-user context, named engineers via
                  `/users/search`). Searches the directory as you type. */}
              <AsyncAssigneeMultiSelect
                values={filters.assignees}
                onChange={(next) => onChange({ ...filters, assignees: next })}
                nameSeed={assigneeNameSeed}
              />
            </Grid>
            {!hideProjectFilter && (
              <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
                <AsyncProjectMultiSelect
                  values={filters.projects}
                  onChange={(next) => onChange({ ...filters, projects: next })}
                  nameSeed={projectNameSeed}
                />
              </Grid>
            )}
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
