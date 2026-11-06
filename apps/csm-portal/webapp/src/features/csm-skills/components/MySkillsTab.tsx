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
  IconButton,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { History, Lock, Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import RefreshButton from "@components/RefreshButton";
import RelativeTime from "@components/RelativeTime";
import {
  useChangeMySkillLevel,
  useMySkills,
  useRemoveMySkill,
} from "@features/csm-skills/api/useMySkills";
import { useSkillLevelTypes } from "@features/csm-skills/api/useSkillCatalogue";
import AddSkillDialog from "@features/csm-skills/components/AddSkillDialog";
import LevelChip from "@features/csm-skills/components/LevelChip";
import SkillHistoryDrawer from "@features/csm-skills/components/SkillHistoryDrawer";
import { findLevelType, sortedLevels } from "@features/csm-skills/utils/levels";
import type { UserSkill } from "@features/csm-skills/types";

/**
 * "My Skills": the caller's own profile. Levels can be changed inline and
 * skills removed after a confirmation. A row inherited from a group is
 * read-only: it carries a lock and offers neither edit nor remove.
 */
export default function MySkillsTab(): JSX.Element {
  const mine = useMySkills();
  const levelTypes = useSkillLevelTypes();
  const change = useChangeMySkillLevel();
  const remove = useRemoveMySkill();

  const [adding, setAdding] = useState(false);
  const [removing, setRemoving] = useState<UserSkill | null>(null);
  const [history, setHistory] = useState<UserSkill | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const rows = [...(mine.data?.skills ?? [])].sort((a, b) => a.skillName.localeCompare(b.skillName));

  const setLevel = (row: UserSkill, levelId: string): void => {
    if (!levelId || levelId === row.level?.id) return;
    setError(null);
    setNotice(null);
    change.mutate(
      { skillId: row.skillId, levelId },
      {
        onSuccess: () => setNotice(`${row.skillName} level updated.`),
        onError: (e) => setError(e.message || "The level could not be changed."),
      },
    );
  };

  const confirmRemove = (row: UserSkill): void => {
    setError(null);
    setNotice(null);
    remove.mutate(row.skillId, {
      onSuccess: () => {
        setRemoving(null);
        setNotice(`${row.skillName} was removed from your skills.`);
      },
      onError: (e) => {
        setRemoving(null);
        setError(e.message || "The skill could not be removed.");
      },
    });
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="body2" color="text.secondary">
          Your skills and how strong you are in each. Others use this to find the right people.
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <RefreshButton
            onRefresh={() => void mine.refetch()}
            isFetching={mine.isFetching}
            updatedAt={mine.dataUpdatedAt}
            label="Refresh my skills"
          />
          <Button variant="contained" startIcon={<Plus size={16} />} onClick={() => setAdding(true)}>
            Add skill
          </Button>
        </Box>
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

      {mine.isLoading ? (
        <Skeleton variant="rounded" height={160} aria-label="Loading my skills" />
      ) : mine.isError ? (
        <QueryErrorState message={mine.error.message || "Failed to load your skills."} error={mine.error} onRetry={() => void mine.refetch()} />
      ) : rows.length === 0 ? (
        <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, p: 4, textAlign: "center" }}>
          <Typography variant="body1">You haven't added any skills yet.</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Add the skills you have and set your level in each.
          </Typography>
          <Button variant="outlined" startIcon={<Plus size={16} />} onClick={() => setAdding(true)}>
            Add your first skill
          </Button>
        </Box>
      ) : (
        <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
          <TableContainer>
            <Table size="small" aria-label="My skills" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
              <TableHead>
                <TableRow sx={{ bgcolor: "action.hover" }}>
                  <TableCell>Skill</TableCell>
                  <TableCell>Categories</TableCell>
                  <TableCell>Level</TableCell>
                  <TableCell>Last updated</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {rows.map((row) => {
                  const levels = sortedLevels(findLevelType(levelTypes.data, row.levelTypeId));
                  const pending = change.isPending && change.variables?.skillId === row.skillId;
                  return (
                    <TableRow key={row.skillId} hover>
                      <TableCell>
                        <Typography variant="body2">{row.skillName}</Typography>
                        {!row.skillActive && (
                          <Chip size="small" variant="outlined" label="Inactive" sx={{ mt: 0.5 }} />
                        )}
                      </TableCell>
                      <TableCell sx={{ maxWidth: 280 }}>
                        <Typography variant="caption" color="text.secondary">
                          {row.categories.map((c) => c.categoryPath ?? c.name).join("; ") || "No category"}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        {row.inherited ? (
                          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                            <LevelChip level={row.level} />
                            <Tooltip title="Inherited from a group. You can't change or remove it here.">
                              <Chip
                                size="small"
                                variant="outlined"
                                icon={<Lock size={12} />}
                                label="Inherited"
                                aria-label={`${row.skillName} is inherited and read-only`}
                              />
                            </Tooltip>
                          </Box>
                        ) : levels.length === 0 ? (
                          <LevelChip level={row.level} />
                        ) : (
                          <TextField
                            select
                            size="small"
                            value={row.level?.id ?? ""}
                            onChange={(e) => setLevel(row, e.target.value)}
                            disabled={pending}
                            slotProps={{ select: { native: true } }}
                            sx={{ minWidth: 150 }}
                            inputProps={{ "aria-label": `Level for ${row.skillName}` }}
                          >
                            {!row.level && (
                              <option value="" disabled>
                                Not set
                              </option>
                            )}
                            {levels.map((l) => (
                              <option key={l.id} value={l.id}>
                                {l.name}
                              </option>
                            ))}
                          </TextField>
                        )}
                      </TableCell>
                      <TableCell>
                        <RelativeTime iso={row.updatedOn} />
                      </TableCell>
                      <TableCell align="right" sx={{ whiteSpace: "nowrap" }}>
                        <Tooltip title="History">
                          <IconButton size="small" aria-label={`History of ${row.skillName}`} onClick={() => setHistory(row)}>
                            <History size={16} />
                          </IconButton>
                        </Tooltip>
                        {!row.inherited && (
                          <Tooltip title="Remove">
                            <IconButton size="small" aria-label={`Remove ${row.skillName}`} onClick={() => setRemoving(row)}>
                              <Trash2 size={16} />
                            </IconButton>
                          </Tooltip>
                        )}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </TableContainer>
        </Box>
      )}

      {adding && <AddSkillDialog onClose={() => setAdding(false)} heldSkillIds={rows.map((r) => r.skillId)} />}

      {history && (
        <SkillHistoryDrawer skillId={history.skillId} skillName={history.skillName} onClose={() => setHistory(null)} />
      )}

      {removing && (
        <Dialog
          open
          onClose={remove.isPending ? undefined : () => setRemoving(null)}
          maxWidth="xs"
          fullWidth
          aria-labelledby="remove-skill-title"
        >
          <DialogTitle id="remove-skill-title">Remove {removing.skillName}?</DialogTitle>
          <DialogContent>
            <DialogContentText>It will be taken off your skills. You can add it again later.</DialogContentText>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setRemoving(null)} disabled={remove.isPending}>
              Cancel
            </Button>
            <Button color="error" variant="contained" onClick={() => confirmRemove(removing)} disabled={remove.isPending}>
              Remove
            </Button>
          </DialogActions>
        </Dialog>
      )}
    </Box>
  );
}
