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
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Skeleton,
  Typography,
  Chip,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import AsyncEntityMultiSelect from "@components/AsyncEntityMultiSelect";
import MultiSelectField from "@components/MultiSelectField";
import QueryErrorState from "@components/QueryErrorState";
import RefreshButton from "@components/RefreshButton";
import RelativeTime from "@components/RelativeTime";
import SearchableMultiSelect from "@components/SearchableMultiSelect";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSkillLevelTypes, useSkillPickerSearch } from "@features/csm-skills/api/useSkillCatalogue";
import { useSearchUserSkills } from "@features/csm-skills/api/useSkillManager";
import LevelChip from "@features/csm-skills/components/LevelChip";
import { distinctLevelValues } from "@features/csm-skills/utils/levels";
import { useScheduleCatalogue } from "@features/csm-team-schedule/api/useTeamSchedule";
import type { SearchUserSkillsRequest, Skill } from "@features/csm-skills/types";

const ROWS_PER_PAGE_OPTIONS = [25, 50, 100];

const getSkillId = (s: Skill): string => s.id;
const getSkillLabel = (s: Skill): string => s.name;

interface Filters {
  search: string;
  skillIds: string[];
  levelIds: string[];
  minLevelValue: string;
  teamKeys: string[];
}

const NO_FILTERS: Filters = { search: "", skillIds: [], levelIds: [], minLevelValue: "", teamKeys: [] };

/**
 * "Skill Manager": a person x skill table for managers ("who has X at level Y
 * in team Z"). Every filter and the paging go to the backend; nothing is
 * filtered in the browser. Filters are ANDed by the backend.
 */
export default function SkillManagerTab(): JSX.Element {
  const [filters, setFilters] = useState<Filters>(NO_FILTERS);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(ROWS_PER_PAGE_OPTIONS[0]);

  const levelTypes = useSkillLevelTypes();
  const schedule = useScheduleCatalogue();
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);

  const patch = (next: Partial<Filters>): void => {
    setFilters((f) => ({ ...f, ...next }));
    setPage(0);
  };

  const request: SearchUserSkillsRequest = {
    ...(debouncedSearch ? { searchQuery: debouncedSearch } : {}),
    ...(filters.skillIds.length ? { skillIds: filters.skillIds } : {}),
    ...(filters.levelIds.length ? { levelIds: filters.levelIds } : {}),
    ...(filters.minLevelValue !== "" ? { minLevelValue: Number(filters.minLevelValue) } : {}),
    ...(filters.teamKeys.length ? { teamKeys: filters.teamKeys } : {}),
    pagination: { limit: rowsPerPage, offset: page * rowsPerPage },
  };
  const result = useSearchUserSkills(request);
  const rows = result.data?.items ?? [];

  const teamName = useMemo(() => {
    const map = new Map<string, string>();
    for (const t of schedule.data?.teams ?? []) map.set(t.key, t.name);
    return (key: string): string => map.get(key) ?? key;
  }, [schedule.data]);
  const teamKeyOptions = useMemo(() => (schedule.data?.teams ?? []).map((t) => t.key), [schedule.data]);

  const levelOptions = useMemo(
    () =>
      (levelTypes.data ?? []).flatMap((t) =>
        t.levels.map((l) => ({ value: l.id, label: `${t.name}: ${l.name}` })),
      ),
    [levelTypes.data],
  );
  const minValues = useMemo(() => distinctLevelValues(levelTypes.data), [levelTypes.data]);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" }, alignItems: "start" }}>
        <TextField
          size="small"
          label="Search people"
          value={filters.search}
          onChange={(e) => patch({ search: e.target.value })}
          slotProps={{ htmlInput: { "aria-label": "Search people by name or email" } }}
        />
        <AsyncEntityMultiSelect<Skill>
          id="manager-skill-filter"
          label="Skills"
          placeholder="Search skills…"
          values={filters.skillIds}
          onChange={(skillIds) => patch({ skillIds })}
          useSearch={useSkillPickerSearch}
          getId={getSkillId}
          getLabel={getSkillLabel}
        />
        <SearchableMultiSelect
          id="manager-team-filter"
          label="Teams"
          values={filters.teamKeys}
          options={teamKeyOptions}
          formatOption={teamName}
          onChange={(teamKeys) => patch({ teamKeys })}
          disabled={schedule.isLoading}
        />
        <MultiSelectField
          id="manager-level-filter"
          label="Levels"
          values={filters.levelIds}
          options={levelOptions}
          onChange={(levelIds) => patch({ levelIds })}
          disabled={levelTypes.isLoading}
        />
        <TextField
          select
          size="small"
          label="Minimum level"
          value={filters.minLevelValue}
          onChange={(e) => patch({ minLevelValue: e.target.value })}
          slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
          disabled={levelTypes.isLoading}
        >
          <option value="">Any</option>
          {minValues.map((m) => (
            <option key={m.value} value={String(m.value)}>
              {m.value} ({m.names.join(", ")})
            </option>
          ))}
        </TextField>
        <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
          <RefreshButton
            onRefresh={() => void result.refetch()}
            isFetching={result.isFetching}
            updatedAt={result.dataUpdatedAt}
            label="Refresh people skills"
          />
        </Box>
      </Box>

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" aria-label="People and their skills" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Person</TableCell>
                <TableCell>Teams</TableCell>
                <TableCell>Skill</TableCell>
                <TableCell>Level</TableCell>
                <TableCell>Last updated</TableCell>
              </TableRow>
            </TableHead>
            <TableBody sx={result.isFetching && !result.isLoading ? { opacity: 0.6 } : undefined}>
              {result.isLoading ? (
                Array.from({ length: 5 }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell colSpan={5}>
                      <Skeleton variant="rounded" height={18} />
                    </TableCell>
                  </TableRow>
                ))
              ) : result.isError ? (
                <TableRow>
                  <TableCell colSpan={5} align="center">
                    <QueryErrorState message={result.error.message || "Failed to load people's skills."} error={result.error} onRetry={() => void result.refetch()} />
                  </TableCell>
                </TableRow>
              ) : rows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No one matches these filters.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                rows.map((row) => (
                  <TableRow key={`${row.userId}:${row.skillId}`} hover>
                    <TableCell>
                      <Typography variant="body2">{row.userName || row.userEmail || row.userId}</Typography>
                      {row.userName && row.userEmail && (
                        <Typography variant="caption" color="text.secondary">
                          {row.userEmail}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>{row.teamKeys.map(teamName).join(", ") || "None"}</TableCell>
                    <TableCell>{row.skillName}</TableCell>
                    <TableCell>
                      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                        <LevelChip level={row.level} />
                        {row.inherited && <Chip size="small" variant="outlined" label="Inherited" />}
                      </Box>
                    </TableCell>
                    <TableCell>
                      <RelativeTime iso={row.updatedOn} />
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={result.data?.total ?? 0}
          page={page}
          onPageChange={(_, p) => setPage(p)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(e) => {
            setRowsPerPage(parseInt(e.target.value, 10));
            setPage(0);
          }}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          showFirstButton
          showLastButton
        />
      </Box>
    </Box>
  );
}
