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
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControlLabel,
  IconButton,
  Skeleton,
  Switch,
  TablePagination,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Pencil, Plus } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import RefreshButton from "@components/RefreshButton";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import {
  useSearchSkills,
  useSkillCategories,
  useSkillLevelTypes,
  useUpdateSkill,
} from "@features/csm-skills/api/useSkillCatalogue";
import { CategorySelect } from "@features/csm-skills/components/CategorySelect";
import LevelChip from "@features/csm-skills/components/LevelChip";
import SkillFormDialog from "@features/csm-skills/components/SkillFormDialog";
import { findLevelType, sortedLevels } from "@features/csm-skills/utils/levels";
import type { SearchSkillsRequest, Skill } from "@features/csm-skills/types";

const ROWS_PER_PAGE_OPTIONS = [10, 25, 50];
const UNCATEGORISED = "Uncategorised";

interface AllSkillsTabProps {
  /** Shows create, edit and (de)activate. Hidden for everyone else; the backend still enforces it. */
  isAdmin: boolean;
}

/** Groups one page of skills under each category path they are filed in. Display only; filtering is the backend's. */
function groupByCategory(skills: readonly Skill[]): { path: string; skills: Skill[] }[] {
  const groups = new Map<string, Skill[]>();
  for (const skill of skills) {
    const paths = skill.categories.length
      ? skill.categories.map((c) => c.categoryPath ?? c.name)
      : [UNCATEGORISED];
    for (const path of new Set(paths)) groups.set(path, [...(groups.get(path) ?? []), skill]);
  }
  return [...groups.entries()]
    .sort(([a], [b]) => (a === UNCATEGORISED ? 1 : b === UNCATEGORISED ? -1 : a.localeCompare(b)))
    .map(([path, list]) => ({ path, skills: list }));
}

/**
 * "All Skills": the read-only catalogue, searched, filtered by category and
 * paged by the backend, grouped by category on screen, with each skill's
 * description and level scale. Admins also get create, edit and (de)activate.
 */
export default function AllSkillsTab({ isAdmin }: AllSkillsTabProps): JSX.Element {
  const [search, setSearch] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const [showInactive, setShowInactive] = useState(false);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(ROWS_PER_PAGE_OPTIONS[0]);
  const [editing, setEditing] = useState<Skill | "new" | null>(null);
  const [toggling, setToggling] = useState<Skill | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const debounced = useDebouncedValue(search.trim(), 300);
  const categories = useSkillCategories();
  const levelTypes = useSkillLevelTypes();
  const update = useUpdateSkill();

  const request: SearchSkillsRequest = {
    ...(debounced ? { searchQuery: debounced } : {}),
    ...(categoryId ? { categoryIds: [categoryId] } : {}),
    ...(isAdmin && showInactive ? { includeInactive: true } : {}),
    pagination: { limit: rowsPerPage, offset: page * rowsPerPage },
  };
  const result = useSearchSkills(request);
  const skills = result.data?.skills;
  const groups = useMemo(() => groupByCategory(skills ?? []), [skills]);

  const confirmToggle = (skill: Skill): void => {
    setError(null);
    setNotice(null);
    update.mutate(
      { id: skill.id, body: { active: !skill.active } },
      {
        onSuccess: () => {
          setToggling(null);
          setNotice(`${skill.name} was ${skill.active ? "deactivated" : "activated"}.`);
        },
        onError: (e) => {
          setToggling(null);
          setError(e.message || "The skill could not be changed.");
        },
      },
    );
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "flex-start", flexWrap: "wrap", gap: 2 }}>
        <TextField
          size="small"
          label="Search skills"
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(0);
          }}
          sx={{ flex: "1 1 240px", maxWidth: 360 }}
          slotProps={{ htmlInput: { "aria-label": "Search skills by name or keyword" } }}
        />
        <Box sx={{ flex: "1 1 240px", maxWidth: 360 }}>
          <CategorySelect
            id="all-skills-category"
            label="Category"
            categories={categories.data}
            loading={categories.isLoading}
            value={categoryId}
            onChange={(id) => {
              setCategoryId(id);
              setPage(0);
            }}
          />
        </Box>
        {isAdmin && (
          <FormControlLabel
            control={
              <Switch
                checked={showInactive}
                onChange={(e) => {
                  setShowInactive(e.target.checked);
                  setPage(0);
                }}
              />
            }
            label="Show inactive"
          />
        )}
        <Box sx={{ flexGrow: 1 }} />
        <RefreshButton
          onRefresh={() => void result.refetch()}
          isFetching={result.isFetching}
          updatedAt={result.dataUpdatedAt}
          label="Refresh skills"
        />
        {isAdmin && (
          <Button variant="contained" startIcon={<Plus size={16} />} onClick={() => setEditing("new")}>
            New skill
          </Button>
        )}
      </Box>

      {error && (
        <Alert severity="error" onClose={() => setError(null)}>
          {error}
        </Alert>
      )}
      {notice && (
        <Alert severity="success" role="status" onClose={() => setNotice(null)}>
          {notice}
        </Alert>
      )}

      {result.isLoading ? (
        <Skeleton variant="rounded" height={200} aria-label="Loading skills" />
      ) : result.isError ? (
        <QueryErrorState message={result.error.message || "Failed to load skills."} error={result.error} onRetry={() => void result.refetch()} />
      ) : groups.length === 0 ? (
        <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, p: 4, textAlign: "center" }}>
          <Typography variant="body1">No skills found.</Typography>
          <Typography variant="body2" color="text.secondary">
            Try a different search or category.
          </Typography>
        </Box>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 3, opacity: result.isFetching ? 0.6 : 1 }}>
          {groups.map((group, gi) => (
            <Box key={group.path} component="section" aria-labelledby={`skill-group-${gi}`}>
              <Typography id={`skill-group-${gi}`} variant="subtitle2" component="h2" sx={{ mb: 1 }}>
                {group.path}
              </Typography>
              <Box component="ul" sx={{ listStyle: "none", m: 0, p: 0, display: "flex", flexDirection: "column", gap: 1 }}>
                {group.skills.map((skill) => {
                  const scale = findLevelType(levelTypes.data, skill.levelTypeId);
                  return (
                    <Box
                      key={skill.id}
                      component="li"
                      sx={{ border: 1, borderColor: "divider", borderRadius: 1, p: 1.5, display: "flex", gap: 2, alignItems: "flex-start" }}
                    >
                      <Box sx={{ flex: 1, minWidth: 0 }}>
                        <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
                          <Typography variant="body1">{skill.name}</Typography>
                          {!skill.active && <Chip size="small" variant="outlined" label="Inactive" />}
                        </Box>
                        {skill.description && (
                          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
                            {skill.description}
                          </Typography>
                        )}
                        <Box sx={{ mt: 1, display: "flex", alignItems: "center", gap: 0.75, flexWrap: "wrap" }}>
                          <Typography variant="caption" color="text.secondary">
                            {scale
                              ? `Levels (${scale.name}):`
                              : !skill.levelTypeId
                                ? "No level scale"
                                : levelTypes.isLoading
                                  ? "Loading levels…"
                                  : "Levels unavailable"}
                          </Typography>
                          {sortedLevels(scale).map((l) => (
                            <LevelChip key={l.id} level={l} />
                          ))}
                        </Box>
                      </Box>
                      {isAdmin && (
                        <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, flexShrink: 0 }}>
                          <Tooltip title="Edit">
                            <IconButton size="small" aria-label={`Edit ${skill.name}`} onClick={() => setEditing(skill)}>
                              <Pencil size={16} />
                            </IconButton>
                          </Tooltip>
                          <Button size="small" onClick={() => setToggling(skill)} aria-label={`${skill.active ? "Deactivate" : "Activate"} ${skill.name}`}>
                            {skill.active ? "Deactivate" : "Activate"}
                          </Button>
                        </Box>
                      )}
                    </Box>
                  );
                })}
              </Box>
            </Box>
          ))}
        </Box>
      )}

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

      {isAdmin && editing && (
        <SkillFormDialog
          skill={editing === "new" ? undefined : editing}
          onClose={(saved) => {
            setEditing(null);
            if (saved) {
              setError(null);
              setNotice(saved);
            }
          }}
        />
      )}

      {isAdmin && toggling && (
        <Dialog open onClose={update.isPending ? undefined : () => setToggling(null)} maxWidth="xs" fullWidth aria-labelledby="toggle-skill-title">
          <DialogTitle id="toggle-skill-title">
            {toggling.active ? "Deactivate" : "Activate"} {toggling.name}?
          </DialogTitle>
          <DialogContent>
            <DialogContentText>
              {toggling.active
                ? "It is marked inactive. You can activate it again later."
                : "It is marked active again."}
            </DialogContentText>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setToggling(null)} disabled={update.isPending}>
              Cancel
            </Button>
            <Button variant="contained" color={toggling.active ? "error" : "primary"} onClick={() => confirmToggle(toggling)} disabled={update.isPending}>
              {toggling.active ? "Deactivate" : "Activate"}
            </Button>
          </DialogActions>
        </Dialog>
      )}
    </Box>
  );
}
