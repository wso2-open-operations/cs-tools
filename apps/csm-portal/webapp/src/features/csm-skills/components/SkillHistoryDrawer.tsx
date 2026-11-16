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

import { Alert, Box, Chip, Drawer, IconButton, List, ListItem, ListItemText, Skeleton, TablePagination, Typography } from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import RelativeTime from "@components/RelativeTime";
import { useMySkillHistory } from "@features/csm-skills/api/useMySkills";
import { useUserSkillHistory } from "@features/csm-skills/api/useSkillManagement";
import { describeHistoryEntry } from "@features/csm-skills/utils/skillRequests";

const PAGE_SIZE = 20;

interface SkillHistoryDrawerProps {
  /** One skill's history; without it (and with userId) the person's whole history. */
  skillId?: string;
  /** Heading: the skill's name, or the person's. */
  title: string;
  /** Someone else's history (the directory). Absent: the caller's own. */
  userId?: string;
  onClose: () => void;
}

/**
 * Skill history, newest first, paged by the backend: the caller's own for one
 * skill, or (from the directory) a person's for one skill or all of them.
 * Each row says what happened (added, level changed, removed, added again).
 */
export default function SkillHistoryDrawer({ skillId, title, userId, onClose }: SkillHistoryDrawerProps): JSX.Element {
  const [page, setPage] = useState(0);
  const mine = useMySkillHistory(userId ? undefined : skillId, PAGE_SIZE, page * PAGE_SIZE);
  const theirs = useUserSkillHistory(userId, skillId ?? "", PAGE_SIZE, page * PAGE_SIZE);
  const history = userId ? theirs : mine;
  const entries = history.data?.history ?? [];
  const allSkills = !skillId;

  return (
    <Drawer
      anchor="right"
      open
      onClose={onClose}
      aria-labelledby="skill-history-title"
      slotProps={{ paper: { sx: { width: { xs: "100%", sm: 440 } } } }}
    >
      <Box sx={{ p: 2, display: "flex", flexDirection: "column", gap: 1, height: "100%" }}>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
          <Typography id="skill-history-title" variant="h6" component="h2">
            History: {title}
          </Typography>
          <IconButton aria-label="Close history" onClick={onClose} size="small">
            <X size={18} />
          </IconButton>
        </Box>
        {history.isLoading ? (
          <Skeleton variant="rounded" height={120} aria-label="Loading history" />
        ) : history.isError ? (
          <Alert severity="error">{history.error.message || "The history could not be loaded."}</Alert>
        ) : entries.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            No changes recorded.
          </Typography>
        ) : (
          <List aria-label={`${title} history`} disablePadding sx={{ overflowY: "auto", flex: 1 }}>
            {entries.map((e) => (
              <ListItem key={e.id} divider disableGutters>
                <ListItemText
                  primary={
                    <Box component="span" sx={{ display: "inline-flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
                      {allSkills && <strong>{e.skillName}:</strong>}
                      {describeHistoryEntry(e)}
                      {e.skillRequestId && <Chip size="small" variant="outlined" label="Via request" />}
                    </Box>
                  }
                  secondary={
                    <>
                      {e.changedBy ? `${e.changedBy}, ` : ""}
                      <RelativeTime iso={e.changedOn} />
                    </>
                  }
                />
              </ListItem>
            ))}
          </List>
        )}
        {(history.data?.total ?? 0) > PAGE_SIZE && (
          <TablePagination
            component="div"
            count={history.data?.total ?? 0}
            page={page}
            onPageChange={(_, p) => setPage(p)}
            rowsPerPage={PAGE_SIZE}
            rowsPerPageOptions={[]}
          />
        )}
      </Box>
    </Drawer>
  );
}
