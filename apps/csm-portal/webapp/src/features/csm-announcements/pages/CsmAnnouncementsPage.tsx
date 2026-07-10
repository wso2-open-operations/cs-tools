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
  FormControl,
  IconButton,
  InputAdornment,
  InputLabel,
  LinearProgress,
  ListItemText,
  MenuItem,
  Select,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import { Search, X } from "@wso2/oxygen-ui-icons-react";
import { useState, type ChangeEvent, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import SemanticChip, { type SemanticRole } from "@components/SemanticChip";
import AsyncProjectMultiSelect from "@features/csm-cases/components/AsyncProjectMultiSelect";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useSearchAnnouncements } from "@features/csm-announcements/api/useSearchAnnouncements";
import {
  DEFAULT_ANNOUNCEMENT_FILTERS,
  type AnnouncementFilters,
} from "@features/csm-announcements/types/csmAnnouncements";
import { STATE_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import type { CaseState } from "@features/csm-dashboard/types/abtDashboard";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];

// `reopened` is intentionally excluded — it only appears as a `nextStates`
// signal, never as a case's own state (see CaseState's doc).
const STATE_OPTIONS: { value: CaseState; label: string }[] = (
  [
    "open",
    "work_in_progress",
    "solution_proposed",
    "awaiting_info",
    "waiting_on_wso2",
    "closed",
  ] as CaseState[]
).map((s) => ({ value: s, label: STATE_LABEL[s] }));

/**
 * Chip colour for an announcement's lifecycle state — announcement-specific,
 * not the case-state palette (which paints a closed case green). Here an Open
 * announcement is live/published → green, a Closed one is inactive → grey.
 */
function announcementStateRole(state?: CaseState): SemanticRole {
  switch (state) {
    case "open":
      return "success";
    case "closed":
      return "default";
    default:
      return "info";
  }
}

function formatDate(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      year: "numeric",
      month: "short",
      day: "numeric",
    }) ?? "—"
  );
}

interface MultiSelectProps<T extends string> {
  id: string;
  label: string;
  values: T[];
  options: { value: T; label: string }[];
  onChange: (next: T[]) => void;
}

/** Checkbox multi-select over a fixed enum (State). */
function MultiSelect<T extends string>({
  id,
  label,
  values,
  options,
  onChange,
}: MultiSelectProps<T>): JSX.Element {
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
        renderValue={(selected) =>
          (Array.isArray(selected) ? selected : [])
            .map((v) => options.find((o) => o.value === v)?.label ?? v)
            .join(", ")
        }
      >
        {options.map((opt) => (
          <MenuItem key={opt.value} value={opt.value} sx={{ py: 0.5 }}>
            <Checkbox size="small" checked={values.includes(opt.value)} sx={{ mr: 1, p: 0.25 }} />
            <ListItemText primary={opt.label} />
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}

/**
 * Read-only announcements list. Announcements are cases of
 * `type: "announcement"` surfaced via `POST /cases/search`. Filterable by
 * state and project (all default to "show all"). Creating /
 * targeting / unpublishing needs the dedicated announcement backend
 * (digiops-cs#2053), which isn't built yet, so this page is view-only for now.
 */
export default function CsmAnnouncementsPage(): JSX.Element {
  const [filters, setFilters] = useState<AnnouncementFilters>(DEFAULT_ANNOUNCEMENT_FILTERS);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);

  const { data, isLoading, isFetching, isError, error } = useSearchAnnouncements(
    { ...filters, search: debouncedSearch },
    page,
    rowsPerPage,
  );

  const announcements = data?.announcements ?? [];
  const total = data?.total ?? 0;

  // Any filter change resets to the first page.
  const patchFilters = (patch: Partial<AnnouncementFilters>): void => {
    setFilters((prev) => ({ ...prev, ...patch }));
    setPage(0);
  };

  const activeFilterCount = filters.states.length + filters.projectIds.length;

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box>
        <Typography variant="h5">Announcements</Typography>
        <Typography variant="body2" color="text.secondary">
          Customer-facing announcements published across projects and tiers.
        </Typography>
      </Box>

      {/* Filters — search + state + project, all "show all" by default */}
      <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap", alignItems: "center" }}>
        <Box sx={{ flex: "1 1 260px", minWidth: 220 }}>
          <TextField
            fullWidth
            size="small"
            placeholder="Search by subject or number…"
            value={filters.search}
            onChange={(e) => patchFilters({ search: e.target.value })}
            slotProps={{
              htmlInput: { "aria-label": "Search announcements" },
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
                      onClick={() => patchFilters({ search: "" })}
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
        <Box sx={{ flex: "1 1 160px", minWidth: 150 }}>
          <MultiSelect
            id="announcements-filter-state"
            label="State"
            values={filters.states}
            options={STATE_OPTIONS}
            onChange={(next) => patchFilters({ states: next })}
          />
        </Box>
        <Box sx={{ flex: "1 1 220px", minWidth: 200 }}>
          <AsyncProjectMultiSelect
            id="announcements-filter-project"
            label="Project"
            values={filters.projectIds}
            onChange={(next) => patchFilters({ projectIds: next })}
          />
        </Box>
        {activeFilterCount > 0 && (
          <Button
            variant="text"
            size="small"
            color="primary"
            startIcon={<X size={16} />}
            onClick={() => patchFilters({ states: [], projectIds: [] })}
          >
            Clear filters
          </Button>
        )}
      </Box>

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        {/* Thin bar during a background refetch (page / filter change) so the
            table isn't blanked to skeletons — cached rows stay visible. */}
        <Box sx={{ height: 2 }}>
          {isFetching && !isLoading && <LinearProgress sx={{ height: 2 }} />}
        </Box>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Number</TableCell>
                <TableCell>Subject</TableCell>
                <TableCell>Project</TableCell>
                <TableCell>State</TableCell>
                <TableCell>Created by</TableCell>
                <TableCell>Updated</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell><Skeleton variant="rounded" width="80%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="90%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="85%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={72} height={22} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="70%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={80} height={18} /></TableCell>
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={6} align="center">
                    <QueryErrorState
                      message={`Failed to load announcements: ${error instanceof Error ? error.message : "unknown error"}`}
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : announcements.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No announcements found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                announcements.map((a) => (
                  <TableRow key={a.id} hover>
                    <TableCell>{a.number || "—"}</TableCell>
                    <TableCell>{a.subject}</TableCell>
                    <TableCell>{a.projectName}</TableCell>
                    <TableCell>
                      {a.state ? (
                        <SemanticChip
                          role={announcementStateRole(a.state)}
                          label={STATE_LABEL[a.state] ?? a.state}
                        />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell>{a.createdBy || "—"}</TableCell>
                    <TableCell>{formatDate(a.updatedAt)}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={total}
          page={page}
          onPageChange={(_, newPage) => setPage(newPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
        />
      </Box>
    </Box>
  );
}
