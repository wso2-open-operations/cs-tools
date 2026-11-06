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

import { Alert, Box, Drawer, IconButton, List, ListItem, ListItemText, Skeleton, TablePagination, Typography } from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import RelativeTime from "@components/RelativeTime";
import { useMySkillHistory } from "@features/csm-skills/api/useMySkills";

const PAGE_SIZE = 20;

interface SkillHistoryDrawerProps {
  skillId: string;
  skillName: string;
  onClose: () => void;
}

/** The caller's level changes for one skill, newest first, paged by the backend. */
export default function SkillHistoryDrawer({ skillId, skillName, onClose }: SkillHistoryDrawerProps): JSX.Element {
  const [page, setPage] = useState(0);
  const history = useMySkillHistory(skillId, PAGE_SIZE, page * PAGE_SIZE);
  const entries = history.data?.history ?? [];

  return (
    <Drawer
      anchor="right"
      open
      onClose={onClose}
      aria-labelledby="skill-history-title"
      slotProps={{ paper: { sx: { width: { xs: "100%", sm: 420 } } } }}
    >
      <Box sx={{ p: 2, display: "flex", flexDirection: "column", gap: 1, height: "100%" }}>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
          <Typography id="skill-history-title" variant="h6" component="h2">
            History: {skillName}
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
            No changes recorded for this skill.
          </Typography>
        ) : (
          <List aria-label={`${skillName} history`} disablePadding sx={{ overflowY: "auto", flex: 1 }}>
            {entries.map((e) => (
              <ListItem key={e.id} divider disableGutters>
                <ListItemText
                  primary={`${e.levelFrom?.name ?? "Not set"} to ${e.levelTo?.name ?? "Removed"}`}
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
