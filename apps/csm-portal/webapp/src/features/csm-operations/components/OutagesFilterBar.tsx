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
  FormControlLabel,
  Grid,
  IconButton,
  InputAdornment,
  Paper,
  TextField,
} from "@wso2/oxygen-ui";
import { ChevronDown, ChevronUp, ListFilter, Search, X } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import type { BeOutageStatus, BeOutageType } from "@api/backend/types";
import { outageStatusLabel, outageTypeLabel, type OutageFilters } from "@features/csm-operations/utils/outages";
import MultiSelectField from "@components/MultiSelectField";

const OUTAGE_TYPES: BeOutageType[] = ["outage", "degradation", "planned"];
const OUTAGE_STATUSES: BeOutageStatus[] = ["in_progress", "resolved"];

interface OutagesFilterBarProps {
  filters: OutageFilters;
  onChange: (next: OutageFilters) => void;
  onReset: () => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
}

function countActive(filters: OutageFilters): number {
  return (
    (filters.types.length > 0 ? 1 : 0) +
    (filters.statuses.length > 0 ? 1 : 0) +
    (filters.publishedOnly ? 1 : 0)
  );
}

/**
 * Search + Type/Status/Published filter bar for the Outages tab, same
 * collapsible-Filters-button shape as `ProblemsFilterBar`/`IncidentsFilterBar`.
 * `publishedOnly` maps straight onto the backend's own
 * `filters.publishedOnly` search flag (`BeSearchOutagesFilters`).
 */
export default function OutagesFilterBar({
  filters,
  onChange,
  onReset,
  isFiltersOpen,
  onFiltersToggle,
}: OutagesFilterBarProps): JSX.Element {
  const activeCount = countActive(filters);
  const hasActive = activeCount > 0;

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Box sx={{ position: "relative", flex: 1, minWidth: 240 }}>
          <TextField
            fullWidth
            size="small"
            placeholder="Search by number or short description…"
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
          <Grid container spacing={2} alignItems="center">
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <MultiSelectField
                id="outage-filter-type"
                label="Type"
                values={filters.types}
                options={OUTAGE_TYPES.map((t) => ({ value: t, label: outageTypeLabel(t) }))}
                onChange={(next) => onChange({ ...filters, types: next as BeOutageType[] })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <MultiSelectField
                id="outage-filter-status"
                label="Status"
                values={filters.statuses}
                options={OUTAGE_STATUSES.map((s) => ({ value: s, label: outageStatusLabel(s) }))}
                onChange={(next) => onChange({ ...filters, statuses: next as BeOutageStatus[] })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <FormControlLabel
                control={
                  <Checkbox
                    checked={filters.publishedOnly}
                    onChange={(e) => onChange({ ...filters, publishedOnly: e.target.checked })}
                  />
                }
                label="Published to status page only"
              />
            </Grid>
          </Grid>
        </>
      )}
    </Paper>
  );
}
