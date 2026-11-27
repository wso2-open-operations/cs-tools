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
  MenuItem,
  Paper,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronDown, ChevronUp, ListFilter, Search, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX } from "react";
import MultiSelectField from "@components/MultiSelectField";
import type { BeSreAlertFacets, BeSreAlertIncidentFilter } from "@api/backend/types";
import {
  countActiveSreAlertFilters,
  SRE_ALERT_INCIDENT_FILTER_LABELS,
  SRE_ALERT_INCIDENT_FILTERS,
  SRE_ALERT_TIERS,
  sreAlertSeverityLabel,
  sreAlertTierLabel,
  type SreAlertFilters,
} from "@features/csm-operations/utils/sreAlerts";

const { DatePicker, LocalizationProvider } = DatePickers;

/** "YYYY-MM-DD" to a local-midnight Date, for the picker to show that day (the value means a UTC day on the wire). */
function parseDateOnly(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Local-midnight Date back to "YYYY-MM-DD". */
function formatDateOnly(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

/** Today's UTC calendar date as a local-midnight Date: no alert is received in the future. */
function todayUTCDateOnly(): Date {
  const now = new Date();
  return new Date(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
}

/**
 * The facet's values as options, plus any value already selected that the facet
 * no longer lists (a link from before it aged out of the last 30 days), so a
 * selection never disappears from its own field.
 */
function facetOptions(
  values: string[] | undefined,
  selected: string[],
  label: (v: string) => string = (v) => v,
): { value: string; label: string }[] {
  const all = [...(values ?? [])];
  for (const v of selected) if (!all.includes(v)) all.push(v);
  return all.map((v) => ({ value: v, label: label(v) }));
}

interface SreAlertsFilterBarProps {
  filters: SreAlertFilters;
  facets: BeSreAlertFacets | undefined;
  onChange: (next: SreAlertFilters) => void;
  onReset: () => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
}

/**
 * Search and filters of the Alerts tab: environment, service, severity and
 * source (choices from `GET /sre-alerts/facets`), tier, whether an incident is
 * linked, and the received date range in whole UTC days.
 */
export default function SreAlertsFilterBar({
  filters,
  facets,
  onChange,
  onReset,
  isFiltersOpen,
  onFiltersToggle,
}: SreAlertsFilterBarProps): JSX.Element {
  const activeCount = countActiveSreAlertFilters(filters);
  const hasActive = activeCount > 0;

  // Recomputed every render so a session left open across UTC midnight still caps at the real today.
  const today = todayUTCDateOnly();
  const receivedFrom = parseDateOnly(filters.receivedFrom);
  const receivedTo = parseDateOnly(filters.receivedTo);
  const fromMaxDate = receivedTo && receivedTo < today ? receivedTo : today;

  const environmentOptions = useMemo(
    () => facetOptions(facets?.environments, filters.environments),
    [facets?.environments, filters.environments],
  );
  const serviceOptions = useMemo(
    () => facetOptions(facets?.services, filters.services),
    [facets?.services, filters.services],
  );
  const severityOptions = useMemo(
    () => facetOptions(facets?.severities, filters.severities, sreAlertSeverityLabel),
    [facets?.severities, filters.severities],
  );
  const sourceOptions = useMemo(
    () => facetOptions(facets?.sources, filters.sources),
    [facets?.sources, filters.sources],
  );
  const tierOptions = useMemo(() => SRE_ALERT_TIERS.map((t) => ({ value: t, label: sreAlertTierLabel(t) })), []);

  // The picker's min/max only bound its calendar: a typed date is checked here too.
  const handleFromChange = (date: unknown): void => {
    if (!(date instanceof Date) || Number.isNaN(date.getTime())) {
      onChange({ ...filters, receivedFrom: "" });
      return;
    }
    if (date > fromMaxDate) return;
    onChange({ ...filters, receivedFrom: formatDateOnly(date) });
  };
  const handleToChange = (date: unknown): void => {
    if (!(date instanceof Date) || Number.isNaN(date.getTime())) {
      onChange({ ...filters, receivedTo: "" });
      return;
    }
    if (date > today) return;
    if (receivedFrom && date < receivedFrom) return;
    onChange({ ...filters, receivedTo: formatDateOnly(date) });
  };

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Box sx={{ position: "relative", flex: 1, minWidth: 240 }}>
          <TextField
            fullWidth
            size="small"
            placeholder="Search by number, service, metric, description or identifier…"
            value={filters.search}
            onChange={(e) => onChange({ ...filters, search: e.target.value })}
            slotProps={{
              htmlInput: { "aria-label": "Search alerts" },
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
          <Button variant="text" size="small" color="primary" onClick={onReset} startIcon={<X size={16} />}>
            Clear filters
          </Button>
        )}
      </Box>

      {isFiltersOpen && (
        <>
          <Divider />
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
              <MultiSelectField
                id="sre-alert-filter-environment"
                label="Environment"
                values={filters.environments}
                options={environmentOptions}
                onChange={(next) => onChange({ ...filters, environments: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
              <MultiSelectField
                id="sre-alert-filter-service"
                label="Service"
                values={filters.services}
                options={serviceOptions}
                onChange={(next) => onChange({ ...filters, services: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
              <MultiSelectField
                id="sre-alert-filter-severity"
                label="Severity"
                values={filters.severities}
                options={severityOptions}
                onChange={(next) => onChange({ ...filters, severities: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
              <MultiSelectField
                id="sre-alert-filter-source"
                label="Source"
                values={filters.sources}
                options={sourceOptions}
                onChange={(next) => onChange({ ...filters, sources: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
              <MultiSelectField
                id="sre-alert-filter-tier"
                label="Tier"
                values={filters.tiers}
                options={tierOptions}
                onChange={(next) => onChange({ ...filters, tiers: next })}
              />
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 4 }}>
              <TextField
                select
                fullWidth
                size="small"
                id="sre-alert-filter-incident"
                label="Incident"
                value={filters.incident}
                onChange={(e) => onChange({ ...filters, incident: e.target.value as BeSreAlertIncidentFilter })}
              >
                {SRE_ALERT_INCIDENT_FILTERS.map((v) => (
                  <MenuItem key={v} value={v}>
                    {SRE_ALERT_INCIDENT_FILTER_LABELS[v]}
                  </MenuItem>
                ))}
              </TextField>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <LocalizationProvider dateAdapter={AdapterDateFns}>
                <DatePicker
                  label="Received from (UTC)"
                  value={receivedFrom}
                  maxDate={fromMaxDate}
                  onChange={handleFromChange}
                  slotProps={{ textField: { size: "small", fullWidth: true }, field: { clearable: true } }}
                />
              </LocalizationProvider>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <LocalizationProvider dateAdapter={AdapterDateFns}>
                <DatePicker
                  label="Received to (UTC)"
                  value={receivedTo}
                  minDate={receivedFrom ?? undefined}
                  maxDate={today}
                  onChange={handleToChange}
                  slotProps={{ textField: { size: "small", fullWidth: true }, field: { clearable: true } }}
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
