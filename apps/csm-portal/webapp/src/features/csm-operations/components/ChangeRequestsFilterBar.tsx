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
  AdapterDateFns,
  Box,
  Button,
  DatePickers,
  Divider,
  Grid,
  IconButton,
  InputAdornment,
  Paper,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import {
  ChevronDown,
  ChevronUp,
  ListFilter,
  Search,
  X,
} from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX } from "react";
import { useTeams } from "@features/csm-dashboard/api/useTeams";
import {
  CHANGE_REQUEST_IMPACTS,
  CHANGE_REQUEST_STATES,
  changeRequestImpactLabel,
  changeRequestStateLabel,
  countActiveCRFilters,
  type ChangeRequestFilters,
} from "@features/csm-operations/utils/changeRequests";
import MultiSelectField from "@components/MultiSelectField";

const { DatePicker, LocalizationProvider } = DatePickers;

/** "YYYY-MM-DD" to a local-midnight Date (avoids the UTC-parse day-shift
 * `new Date(dateString)` can cause depending on the viewer's timezone). */
function parseDateOnly(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Local-midnight Date back to "YYYY-MM-DD", matching ChangeRequestFilters'
 * closedStartDate/closedEndDate wire format. */
function formatDateOnly(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

interface ChangeRequestsFilterBarProps {
  filters: ChangeRequestFilters;
  onChange: (next: ChangeRequestFilters) => void;
  onReset: () => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
}


export default function ChangeRequestsFilterBar({
  filters,
  onChange,
  onReset,
  isFiltersOpen,
  onFiltersToggle,
}: ChangeRequestsFilterBarProps): JSX.Element {
  const activeCount = countActiveCRFilters(filters);
  const hasActive = activeCount > 0;

  const stateOptions = useMemo(
    () =>
      CHANGE_REQUEST_STATES.map((s) => ({
        value: s,
        label: changeRequestStateLabel(s),
      })),
    [],
  );
  const impactOptions = useMemo(
    () =>
      CHANGE_REQUEST_IMPACTS.map((i) => ({
        value: i,
        label: changeRequestImpactLabel(i),
      })),
    [],
  );

  // Change requests are SRE-owned, so this control is scoped to the
  // `sre-abt` team family — see `IncidentsFilterBar`'s equivalent note on
  // why this isn't `cre-abt` (the cases list's own "SRE Team" control's
  // family scoping).
  const { data: teams } = useTeams(true, "sre-abt");
  const sreTeamOptions = useMemo(
    () =>
      (teams ?? [])
        .filter(
          (t): t is typeof t & { sreGroupId: string } =>
            Boolean(t.sreGroupId) && t.family === "sre-abt",
        )
        .map((t) => ({ value: t.sreGroupId, label: t.name })),
    [teams],
  );

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      {/* Search bar + filters toggle */}
      <Box
        sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}
      >
        <Box sx={{ position: "relative", flex: 1, minWidth: 240 }}>
          <TextField
            fullWidth
            size="small"
            placeholder="Search by number or subject…"
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
          color="primary"
          onClick={onFiltersToggle}
          startIcon={<ListFilter size={16} />}
          endIcon={isFiltersOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
        >
          {hasActive ? `Filters (${activeCount})` : "Filters"}
        </Button>
        {hasActive && (
          <Button
            variant="text"
            size="small"
            color="primary"
            onClick={onReset}
            startIcon={<X size={16} />}
          >
            Clear filters
          </Button>
        )}
      </Box>

      {/* Collapsible filter grid */}
      {isFiltersOpen && (
        <>
          <Divider />
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <MultiSelectField
                id="cr-filter-state"
                label="State"
                values={filters.states}
                options={stateOptions}
                onChange={(next) => onChange({ ...filters, states: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <MultiSelectField
                id="cr-filter-impact"
                label="Impact"
                values={filters.impacts}
                options={impactOptions}
                onChange={(next) => onChange({ ...filters, impacts: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <MultiSelectField
                id="cr-filter-sre-team"
                label="SRE Team"
                values={filters.sreTeamIds}
                options={sreTeamOptions}
                onChange={(next) => onChange({ ...filters, sreTeamIds: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <LocalizationProvider dateAdapter={AdapterDateFns}>
                <DatePicker
                  label="Closed from"
                  value={parseDateOnly(filters.closedStartDate)}
                  maxDate={parseDateOnly(filters.closedEndDate) ?? undefined}
                  onChange={(date) =>
                    onChange({
                      ...filters,
                      closedStartDate:
                        date instanceof Date && !Number.isNaN(date.getTime())
                          ? formatDateOnly(date)
                          : "",
                    })
                  }
                  slotProps={{
                    textField: { size: "small", fullWidth: true },
                    field: { clearable: true },
                  }}
                />
              </LocalizationProvider>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <LocalizationProvider dateAdapter={AdapterDateFns}>
                <DatePicker
                  label="Closed to"
                  value={parseDateOnly(filters.closedEndDate)}
                  minDate={parseDateOnly(filters.closedStartDate) ?? undefined}
                  onChange={(date) =>
                    onChange({
                      ...filters,
                      closedEndDate:
                        date instanceof Date && !Number.isNaN(date.getTime())
                          ? formatDateOnly(date)
                          : "",
                    })
                  }
                  slotProps={{
                    textField: { size: "small", fullWidth: true },
                    field: { clearable: true },
                  }}
                />
              </LocalizationProvider>
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
