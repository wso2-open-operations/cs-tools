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
} from "@wso2/oxygen-ui";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import { ChevronDown, ChevronUp, ListFilter, Search, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX } from "react";
import type { BeIncidentPriority } from "@api/backend/types";
import {
  countActiveIncidentFilters,
  incidentPriorityLabel,
  INCIDENT_PRIORITIES,
  type IncidentFilters,
} from "@features/csm-operations/utils/incidents";

interface IncidentsFilterBarProps {
  filters: IncidentFilters;
  onChange: (next: IncidentFilters) => void;
  onReset: () => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
}

/**
 * Search + Priority filter bar for the Incidents tab. Priority is the only
 * filterable field beyond free text — `IncidentSearchPayload.filters` has no
 * state/category filter to build a control for (see openapi.yaml).
 */
export default function IncidentsFilterBar({
  filters,
  onChange,
  onReset,
  isFiltersOpen,
  onFiltersToggle,
}: IncidentsFilterBarProps): JSX.Element {
  const activeCount = countActiveIncidentFilters(filters);
  const hasActive = activeCount > 0;

  const priorityOptions = useMemo(
    () =>
      INCIDENT_PRIORITIES.map((p) => ({
        value: p,
        label: incidentPriorityLabel(p),
      })),
    [],
  );

  const handlePriorityChange = (event: SelectChangeEvent<string[]>): void => {
    const val = event.target.value;
    onChange({
      ...filters,
      priorities: (Array.isArray(val) ? val : [val]) as BeIncidentPriority[],
    });
  };

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
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

      {isFiltersOpen && (
        <>
          <Divider />
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <FormControl fullWidth size="small">
                <InputLabel id="incident-filter-priority-label">Priority</InputLabel>
                <Select
                  multiple
                  labelId="incident-filter-priority-label"
                  id="incident-filter-priority"
                  value={filters.priorities}
                  label="Priority"
                  onChange={handlePriorityChange}
                  renderValue={(selected) =>
                    (selected as string[])
                      .map((v) => priorityOptions.find((o) => o.value === v)?.label ?? v)
                      .join(", ")
                  }
                >
                  {priorityOptions.map((opt) => (
                    <MenuItem key={opt.value} value={opt.value} sx={{ py: 0.5 }}>
                      <Checkbox
                        size="small"
                        checked={filters.priorities.includes(opt.value)}
                        sx={{ mr: 1, p: 0.25 }}
                      />
                      <ListItemText primary={opt.label} />
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
          </Grid>
        </>
      )}
    </Paper>
  );
}
