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
  Alert,
  Box,
  Button,
  Chip,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { Check, Plus } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import { useSkillLevelTypes } from "@features/csm-skills/api/useSkillCatalogue";
import { useSkillDirectory } from "@features/csm-skills/api/useSkillManagement";
import LevelChip from "@features/csm-skills/components/LevelChip";
import PersonSkillsDrawer from "@features/csm-skills/components/PersonSkillsDrawer";
import { findLevelType, sortedLevels } from "@features/csm-skills/utils/levels";
import { useScheduleCatalogue } from "@features/csm-team-schedule/api/useTeamSchedule";
import type { Skill, SkillDirectoryPerson, UserSkill } from "@features/csm-skills/types";

const PAGE_SIZES = [10, 25, 50];

interface SkillDetailPanelProps {
  skill: Skill;
  /** The caller's own row for this skill, when they hold it. */
  mine: UserSkill | undefined;
  /** Holders are shown only with the directory capability; the backend checks it too. */
  canViewPeople: boolean;
  onAdd: (skill: Skill) => void;
}

/**
 * The skill selected in the All Skills tree: what it is, its levels, whether
 * it is on the caller's profile, and (with directory access) the people who
 * hold it and at what level. A person opens their whole profile.
 */
export default function SkillDetailPanel({ skill, mine, canViewPeople, onAdd }: SkillDetailPanelProps): JSX.Element {
  const levelTypes = useSkillLevelTypes();
  const scale = findLevelType(levelTypes.data, skill.levelTypeId);
  const levels = sortedLevels(scale);

  return (
    <Box component="section" aria-labelledby="skill-detail-title" sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
        <Box sx={{ minWidth: 0 }}>
          <Typography id="skill-detail-title" variant="h6" component="h2">
            {skill.name}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {skill.categories.map((c) => c.categoryPath ?? c.name).join("; ") || "Uncategorised"}
          </Typography>
        </Box>
        {mine ? (
          <Chip
            color="success"
            variant="outlined"
            icon={<Check size={14} />}
            label={`On your profile${mine.level ? `: ${mine.level.name}` : ""}`}
          />
        ) : (
          skill.levelTypeId && (
            <Button variant="outlined" size="small" startIcon={<Plus size={14} />} onClick={() => onAdd(skill)}>
              Add to my skills
            </Button>
          )
        )}
      </Box>

      {skill.description && <Typography variant="body2">{skill.description}</Typography>}
      {skill.keywords && (
        <Typography variant="caption" color="text.secondary">
          Also known as: {skill.keywords}
        </Typography>
      )}
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, flexWrap: "wrap" }}>
        <Typography variant="caption" color="text.secondary">
          {scale ? `Levels (${scale.name}):` : !skill.levelTypeId ? "No level scale" : levelTypes.isLoading ? "Loading levels…" : "Levels unavailable"}
        </Typography>
        {levels.map((l) => (
          <LevelChip key={l.id} level={l} />
        ))}
      </Box>

      {canViewPeople ? (
        <SkillHolders skill={skill} />
      ) : (
        <Alert severity="info" variant="outlined">
          Seeing who has this skill needs access to the Skill Directory.
        </Alert>
      )}
    </Box>
  );
}

/** The people who hold one skill: the directory search with that one skill as its criterion. */
function SkillHolders({ skill }: { skill: Skill }): JSX.Element {
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(PAGE_SIZES[0]);
  const [person, setPerson] = useState<SkillDirectoryPerson | null>(null);
  const [shownFor, setShownFor] = useState(skill.id);
  // A different skill starts again at the first page.
  if (shownFor !== skill.id) {
    setShownFor(skill.id);
    setPage(0);
  }
  const schedule = useScheduleCatalogue();
  const teamName = useMemo(() => {
    const map = new Map<string, string>();
    for (const t of schedule.data?.teams ?? []) map.set(t.key, t.name);
    return (key: string): string => map.get(key) ?? key;
  }, [schedule.data]);

  const result = useSkillDirectory({
    criteria: [{ skillId: skill.id }],
    match: "ALL",
    pagination: { limit: rowsPerPage, offset: page * rowsPerPage },
  });
  const people = result.data?.items ?? [];
  const total = result.data?.total ?? 0;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Typography variant="subtitle2" component="h3">
        People with this skill{result.data ? ` (${total})` : ""}
      </Typography>
      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" aria-label={`People with ${skill.name}`} sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Person</TableCell>
                <TableCell>Level</TableCell>
                <TableCell>Teams</TableCell>
              </TableRow>
            </TableHead>
            <TableBody sx={result.isFetching && !result.isLoading ? { opacity: 0.6 } : undefined}>
              {result.isLoading ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell colSpan={3}>
                      <Skeleton variant="rounded" height={18} />
                    </TableCell>
                  </TableRow>
                ))
              ) : result.isError ? (
                <TableRow>
                  <TableCell colSpan={3} align="center">
                    <QueryErrorState
                      message={result.error.message || "Failed to load the people with this skill."}
                      error={result.error}
                      onRetry={() => void result.refetch()}
                    />
                  </TableCell>
                </TableRow>
              ) : people.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={3} align="center" sx={{ py: 3 }}>
                    <Typography variant="body2" color="text.secondary">
                      No one has added this skill yet.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                people.map((p) => {
                  const held = p.skills.find((s) => s.skillId === skill.id);
                  const name = p.userName || p.userEmail || p.userId;
                  return (
                    <TableRow
                      key={p.userId}
                      hover
                      tabIndex={0}
                      sx={{ cursor: "pointer" }}
                      aria-label={`Open ${name}'s skills`}
                      onClick={() => setPerson(p)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          setPerson(p);
                        }
                      }}
                    >
                      <TableCell sx={{ maxWidth: 260 }}>
                        <Typography variant="body2" noWrap>
                          {name}
                        </Typography>
                        {p.userName && p.userEmail && (
                          <Typography variant="caption" color="text.secondary" noWrap component="div">
                            {p.userEmail}
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                          <LevelChip level={held?.level} />
                          {held?.inherited && <Chip size="small" variant="outlined" label="Inherited" />}
                        </Box>
                      </TableCell>
                      <TableCell sx={{ maxWidth: 200 }}>{p.teamKeys.map(teamName).join(", ") || "None"}</TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>
        {total > PAGE_SIZES[0] && (
          <TablePagination
            component="div"
            count={total}
            page={page}
            onPageChange={(_, p) => setPage(p)}
            rowsPerPage={rowsPerPage}
            onRowsPerPageChange={(e) => {
              setRowsPerPage(parseInt(e.target.value, 10));
              setPage(0);
            }}
            rowsPerPageOptions={PAGE_SIZES}
            showFirstButton
            showLastButton
          />
        )}
      </Box>
      {person && <PersonSkillsDrawer person={person} teamName={teamName} onClose={() => setPerson(null)} />}
    </Box>
  );
}
