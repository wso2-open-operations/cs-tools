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
  Drawer,
  IconButton,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { History, X } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import RelativeTime from "@components/RelativeTime";
import { useUserSkillProfile } from "@features/csm-skills/api/useSkillManagement";
import LevelChip from "@features/csm-skills/components/LevelChip";
import SkillHistoryDrawer from "@features/csm-skills/components/SkillHistoryDrawer";
import type { SkillDirectoryPerson } from "@features/csm-skills/types";

interface PersonSkillsDrawerProps {
  person: Pick<SkillDirectoryPerson, "userId" | "userName" | "userEmail" | "teamKeys">;
  teamName: (key: string) => string;
  onClose: () => void;
}

/**
 * One person's whole skills profile, from the directory, with their history
 * (all skills, or one). Read-only: a person changes only their own skills.
 */
export default function PersonSkillsDrawer({ person, teamName, onClose }: PersonSkillsDrawerProps): JSX.Element {
  const profile = useUserSkillProfile(person.userId);
  const [history, setHistory] = useState<{ skillId?: string; title: string } | null>(null);
  const name = person.userName || person.userEmail || person.userId;
  const skills = [...(profile.data?.skills ?? [])].sort((a, b) => a.skillName.localeCompare(b.skillName));

  return (
    <>
      <Drawer
        anchor="right"
        open
        onClose={onClose}
        aria-labelledby="person-skills-title"
        slotProps={{ paper: { sx: { width: { xs: "100%", sm: 560 } } } }}
      >
        <Box sx={{ p: 2, display: "flex", flexDirection: "column", gap: 2 }}>
          <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 1 }}>
            <Box>
              <Typography id="person-skills-title" variant="h6" component="h2">
                {name}
              </Typography>
              {person.userName && person.userEmail && (
                <Typography variant="body2" color="text.secondary">
                  {person.userEmail}
                </Typography>
              )}
              <Box sx={{ display: "flex", gap: 0.5, flexWrap: "wrap", mt: 0.5 }}>
                {person.teamKeys.map((k) => (
                  <Chip key={k} size="small" variant="outlined" label={teamName(k)} />
                ))}
              </Box>
            </Box>
            <IconButton aria-label="Close" onClick={onClose} size="small">
              <X size={18} />
            </IconButton>
          </Box>
          <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <Typography variant="subtitle2">Skills ({skills.length})</Typography>
            <Button size="small" startIcon={<History size={14} />} onClick={() => setHistory({ title: name })}>
              Full history
            </Button>
          </Box>
          {profile.isLoading ? (
            <Skeleton variant="rounded" height={160} aria-label="Loading skills" />
          ) : profile.isError ? (
            <Alert severity="error">{profile.error.message || "Their skills could not be loaded."}</Alert>
          ) : skills.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No skills on this profile.
            </Typography>
          ) : (
            <Table size="small" aria-label={`${name}'s skills`} sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
              <TableHead>
                <TableRow sx={{ bgcolor: "action.hover" }}>
                  <TableCell>Skill</TableCell>
                  <TableCell>Level</TableCell>
                  <TableCell>Updated</TableCell>
                  <TableCell align="right" />
                </TableRow>
              </TableHead>
              <TableBody>
                {skills.map((s) => (
                  <TableRow key={s.skillId}>
                    <TableCell sx={{ maxWidth: 220 }}>
                      <Typography variant="body2">{s.skillName}</Typography>
                      <Typography variant="caption" color="text.secondary">
                        {s.categories.map((c) => c.categoryPath ?? c.name).join("; ")}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Box sx={{ display: "flex", gap: 0.5, alignItems: "center" }}>
                        <LevelChip level={s.level} />
                        {s.inherited && <Chip size="small" variant="outlined" label="Inherited" />}
                      </Box>
                    </TableCell>
                    <TableCell>
                      <RelativeTime iso={s.updatedOn} />
                    </TableCell>
                    <TableCell align="right">
                      <IconButton
                        size="small"
                        aria-label={`History of ${s.skillName}`}
                        onClick={() => setHistory({ skillId: s.skillId, title: `${s.skillName} (${name})` })}
                      >
                        <History size={16} />
                      </IconButton>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </Box>
      </Drawer>
      {history && (
        <SkillHistoryDrawer userId={person.userId} skillId={history.skillId} title={history.title} onClose={() => setHistory(null)} />
      )}
    </>
  );
}
