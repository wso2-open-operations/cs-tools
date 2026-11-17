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
  Chip,
  IconButton,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import QueryErrorState from "@components/QueryErrorState";
import RefreshButton from "@components/RefreshButton";
import SearchableMultiSelect from "@components/SearchableMultiSelect";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSkillCategories, useSkillLevelTypes, useSkillPickerSearch } from "@features/csm-skills/api/useSkillCatalogue";
import { useSkillDirectory } from "@features/csm-skills/api/useSkillManagement";
import { CategoryMultiSelect } from "@features/csm-skills/components/CategorySelect";
import LevelChip from "@features/csm-skills/components/LevelChip";
import PersonSkillsDrawer from "@features/csm-skills/components/PersonSkillsDrawer";
import { findLevelType, sortedLevels } from "@features/csm-skills/utils/levels";
import { useScheduleCatalogue } from "@features/csm-team-schedule/api/useTeamSchedule";
import type { SearchSkillDirectoryRequest, Skill, SkillDirectoryPerson } from "@features/csm-skills/types";

const ROWS_PER_PAGE_OPTIONS = [25, 50, 100];
const OTHER_SKILLS_SHOWN = 3;

const getSkillId = (s: Skill): string => s.id;
const getSkillLabel = (s: Skill): string => s.name;

/** One "has this skill at this level or above" row of the criteria builder. */
interface CriterionRow {
  key: number;
  skillId: string;
  skillName: string;
  levelTypeId?: string | null;
  /** 0 = any level. */
  minLevelValue: number;
}

let nextKey = 1;
const emptyRow = (): CriterionRow => ({ key: nextKey++, skillId: "", skillName: "", minLevelValue: 0 });

/**
 * "Skill Directory": one row per person who matches. The criteria are pairs of
 * skill and minimum level ("APIM at Intermediate or above") combined with ALL
 * (every one) or ANY (at least one); teams, categories and a name search narrow
 * it further. Every filter and the paging go to the backend. Clicking a person
 * opens their whole profile and history.
 */
export default function SkillDirectoryTab(): JSX.Element {
  const [search, setSearch] = useState("");
  const [teamKeys, setTeamKeys] = useState<string[]>([]);
  const [categoryIds, setCategoryIds] = useState<string[]>([]);
  const [rows, setRows] = useState<CriterionRow[]>(() => [emptyRow()]);
  const [match, setMatch] = useState<"ALL" | "ANY">("ALL");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(ROWS_PER_PAGE_OPTIONS[0]);
  const [person, setPerson] = useState<SkillDirectoryPerson | null>(null);

  const levelTypes = useSkillLevelTypes();
  const categories = useSkillCategories();
  const schedule = useScheduleCatalogue();
  const debouncedSearch = useDebouncedValue(search.trim(), 300);

  const criteria = rows.filter((r) => r.skillId);
  const request: SearchSkillDirectoryRequest = {
    ...(debouncedSearch ? { searchQuery: debouncedSearch } : {}),
    ...(teamKeys.length ? { teamKeys } : {}),
    ...(categoryIds.length ? { categoryIds } : {}),
    ...(criteria.length
      ? { criteria: criteria.map((c) => ({ skillId: c.skillId, minLevelValue: c.minLevelValue })), match }
      : {}),
    pagination: { limit: rowsPerPage, offset: page * rowsPerPage },
  };
  const result = useSkillDirectory(request);
  const people = result.data?.items ?? [];

  const teamName = useMemo(() => {
    const map = new Map<string, string>();
    for (const t of schedule.data?.teams ?? []) map.set(t.key, t.name);
    return (key: string): string => map.get(key) ?? key;
  }, [schedule.data]);
  const teamKeyOptions = useMemo(() => (schedule.data?.teams ?? []).map((t) => t.key), [schedule.data]);

  const patchRow = (key: number, next: Partial<CriterionRow>): void => {
    setRows((list) => list.map((r) => (r.key === key ? { ...r, ...next } : r)));
    setPage(0);
  };
  const removeRow = (key: number): void => {
    setRows((list) => (list.length === 1 ? [emptyRow()] : list.filter((r) => r.key !== key)));
    setPage(0);
  };
  const clearAll = (): void => {
    setSearch("");
    setTeamKeys([]);
    setCategoryIds([]);
    setRows([emptyRow()]);
    setMatch("ALL");
    setPage(0);
  };

  const criteriaIds = new Set(criteria.map((c) => c.skillId));
  const showMatches = match === "ANY" && criteria.length > 1;
  const colCount = 3 + criteria.length + (showMatches ? 1 : 0);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="body2" color="text.secondary">
        Find people by skill. Add skills with a minimum level, then choose whether a person needs all of them or any.
      </Typography>

      <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", md: "repeat(3, minmax(0, 1fr))" }, alignItems: "start" }}>
        <TextField
          size="small"
          label="Search people"
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(0);
          }}
          slotProps={{ htmlInput: { "aria-label": "Search people by name or email" } }}
        />
        <SearchableMultiSelect
          id="directory-team-filter"
          label="Teams"
          values={teamKeys}
          options={teamKeyOptions}
          formatOption={teamName}
          onChange={(keys) => {
            setTeamKeys(keys);
            setPage(0);
          }}
          disabled={schedule.isLoading}
        />
        <CategoryMultiSelect
          id="directory-category-filter"
          label="Categories"
          categories={categories.data}
          loading={categories.isLoading}
          values={categoryIds}
          onChange={(ids) => {
            setCategoryIds(ids);
            setPage(0);
          }}
          helperText="People with any skill in these categories."
        />
      </Box>

      <Box
        component="fieldset"
        sx={{ border: 1, borderColor: "divider", borderRadius: 1, p: 2, m: 0, display: "flex", flexDirection: "column", gap: 1.5 }}
      >
        <Box component="legend" sx={{ px: 0.5 }}>
          <Typography variant="subtitle2" component="span">
            Skills
          </Typography>
        </Box>
        {rows.map((row, i) => {
          const levels = sortedLevels(findLevelType(levelTypes.data, row.levelTypeId));
          return (
            <Box key={row.key} sx={{ display: "grid", gap: 1.5, gridTemplateColumns: { xs: "1fr", md: "minmax(0, 2fr) minmax(0, 1fr) auto" }, alignItems: "start" }}>
              <AsyncEntitySelect<Skill>
                id={`directory-skill-${row.key}`}
                label={`Skill ${i + 1}`}
                placeholder="Search skills…"
                value={row.skillId}
                knownLabel={row.skillName || undefined}
                onChange={(id, item) =>
                  patchRow(row.key, {
                    skillId: id,
                    skillName: item?.name ?? "",
                    levelTypeId: item?.levelTypeId ?? null,
                    minLevelValue: 0,
                  })
                }
                useSearch={useSkillPickerSearch}
                getId={getSkillId}
                getLabel={getSkillLabel}
                excludeIds={[...criteriaIds].filter((id) => id !== row.skillId)}
              />
              <TextField
                select
                size="small"
                label="Minimum level"
                value={String(row.minLevelValue)}
                onChange={(e) => patchRow(row.key, { minLevelValue: Number(e.target.value) })}
                disabled={!row.skillId || levels.length === 0}
                slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
              >
                <option value="0">Any level</option>
                {levels
                  .filter((l) => l.value != null)
                  .map((l) => (
                    <option key={l.id} value={String(l.value)}>
                      {l.name} or above
                    </option>
                  ))}
              </TextField>
              <Tooltip title="Remove this skill">
                <span>
                  <IconButton
                    aria-label={`Remove skill ${i + 1}`}
                    onClick={() => removeRow(row.key)}
                    disabled={rows.length === 1 && !row.skillId}
                  >
                    <X size={16} />
                  </IconButton>
                </span>
              </Tooltip>
            </Box>
          );
        })}
        <Box sx={{ display: "flex", alignItems: "center", gap: 2, flexWrap: "wrap" }}>
          <Button
            size="small"
            startIcon={<Plus size={14} />}
            onClick={() => setRows((list) => [...list, emptyRow()])}
            disabled={rows.some((r) => !r.skillId) || rows.length >= 20}
          >
            Add skill
          </Button>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Typography variant="body2" id="directory-match-label">
              People need
            </Typography>
            <ToggleButtonGroup
              size="small"
              exclusive
              value={match}
              aria-labelledby="directory-match-label"
              onChange={(_, v: "ALL" | "ANY" | null) => {
                if (v) {
                  setMatch(v);
                  setPage(0);
                }
              }}
            >
              <ToggleButton value="ALL">All of these</ToggleButton>
              <ToggleButton value="ANY">Any of these</ToggleButton>
            </ToggleButtonGroup>
          </Box>
          <Box sx={{ flexGrow: 1 }} />
          <Button size="small" onClick={clearAll}>
            Clear filters
          </Button>
          <RefreshButton
            onRefresh={() => void result.refetch()}
            isFetching={result.isFetching}
            updatedAt={result.dataUpdatedAt}
            label="Refresh directory"
          />
        </Box>
      </Box>

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" aria-label="People by skill" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Person</TableCell>
                <TableCell>Teams</TableCell>
                {criteria.map((c) => (
                  <TableCell key={c.skillId}>{c.skillName || "Skill"}</TableCell>
                ))}
                <TableCell>{criteria.length ? "Other skills" : "Skills"}</TableCell>
                {showMatches && <TableCell>Matches</TableCell>}
              </TableRow>
            </TableHead>
            <TableBody sx={result.isFetching && !result.isLoading ? { opacity: 0.6 } : undefined}>
              {result.isLoading ? (
                Array.from({ length: rowsPerPage > 10 ? 10 : rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell colSpan={colCount}>
                      <Skeleton variant="rounded" height={18} />
                    </TableCell>
                  </TableRow>
                ))
              ) : result.isError ? (
                <TableRow>
                  <TableCell colSpan={colCount} align="center">
                    <QueryErrorState
                      message={result.error.message || "Failed to search the skill directory."}
                      error={result.error}
                      onRetry={() => void result.refetch()}
                    />
                  </TableCell>
                </TableRow>
              ) : people.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={colCount} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No one matches these filters.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                people.map((p) => {
                  const bySkill = new Map(p.skills.map((s) => [s.skillId, s]));
                  const others = p.skills.filter((s) => !criteriaIds.has(s.skillId));
                  return (
                    <TableRow
                      key={p.userId}
                      hover
                      tabIndex={0}
                      sx={{ cursor: "pointer" }}
                      onClick={() => setPerson(p)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          setPerson(p);
                        }
                      }}
                      aria-label={`Open ${p.userName || p.userEmail || "person"}'s skills`}
                    >
                      <TableCell sx={{ maxWidth: 280 }}>
                        <Typography variant="body2" noWrap>
                          {p.userName || p.userEmail || p.userId}
                        </Typography>
                        {p.userName && p.userEmail && (
                          <Typography variant="caption" color="text.secondary" noWrap component="div">
                            {p.userEmail}
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell sx={{ maxWidth: 200 }}>{p.teamKeys.map(teamName).join(", ") || "None"}</TableCell>
                      {criteria.map((c) => {
                        const held = bySkill.get(c.skillId);
                        return (
                          <TableCell key={c.skillId}>
                            {held ? (
                              <LevelChip level={held.level} />
                            ) : (
                              <Typography variant="body2" color="text.secondary">
                                —
                              </Typography>
                            )}
                          </TableCell>
                        );
                      })}
                      <TableCell sx={{ maxWidth: 360 }}>
                        <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5 }}>
                          {others.slice(0, OTHER_SKILLS_SHOWN).map((s) => (
                            <Chip key={s.skillId} size="small" variant="outlined" label={`${s.skillName}${s.level ? ` · ${s.level.name}` : ""}`} />
                          ))}
                          {others.length > OTHER_SKILLS_SHOWN && (
                            <Chip size="small" label={`+${others.length - OTHER_SKILLS_SHOWN} more`} />
                          )}
                        </Box>
                      </TableCell>
                      {showMatches && (
                        <TableCell>
                          {p.matchedCount} of {criteria.length}
                        </TableCell>
                      )}
                    </TableRow>
                  );
                })
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

      {person && <PersonSkillsDrawer person={person} teamName={teamName} onClose={() => setPerson(null)} />}
    </Box>
  );
}
