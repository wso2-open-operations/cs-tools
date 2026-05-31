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

import { Box, Button, Skeleton, Typography, useTheme } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useNavigate } from "react-router";
import AdminTabs from "@features/csm-admin/components/AdminTabs";
import CreateGroupDialog from "@features/csm-admin/components/CreateGroupDialog";
import { useGetCsmGroups } from "@features/csm-admin/api/useCsmAdmin";

const GRID = "minmax(180px, 1.4fr) minmax(280px, 2.5fr) auto";

const HEADER_CELLS: { label: string; align?: "left" | "right" }[] = [
  { label: "Group" },
  { label: "Description" },
  { label: "Members", align: "right" },
];

export default function CsmAdminGroupsPage(): JSX.Element {
  const navigate = useNavigate();
  const theme = useTheme();
  const { data: groups, isLoading } = useGetCsmGroups();
  const [createOpen, setCreateOpen] = useState(false);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <AdminTabs />

      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2 }}>
        <Typography variant="body2" color="text.secondary">
          {isLoading ? "Loading…" : `${groups?.length ?? 0} groups`}
        </Typography>
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => setCreateOpen(true)}
        >
          Create group
        </Button>
      </Box>
      <CreateGroupDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(g) => navigate(`/admin/groups/${g.id}`)}
      />

      <Box
        sx={{
          border: 1,
          borderColor: "divider",
          borderRadius: 1,
          overflow: "hidden",
          display: "grid",
          gridTemplateColumns: GRID,
          columnGap: 2,
        }}
      >
        <Box
          sx={{
            gridColumn: "1 / -1",
            display: "grid",
            gridTemplateColumns: "subgrid",
            columnGap: 2,
            alignItems: "center",
            px: 2,
            py: 1.25,
            bgcolor: "action.hover",
            borderBottom: 1,
            borderColor: "divider",
          }}
        >
          {HEADER_CELLS.map((h) => (
            <Typography
              key={h.label}
              variant="caption"
              color="text.secondary"
              sx={{ textAlign: h.align ?? "left", fontWeight: 600 }}
            >
              {h.label}
            </Typography>
          ))}
        </Box>

        {isLoading &&
          [0, 1].map((i) => (
            <Box
              key={i}
              sx={{
                gridColumn: "1 / -1",
                px: 2,
                py: 1.25,
                borderBottom: 1,
                borderColor: "divider",
                "&:last-of-type": { borderBottom: 0 },
              }}
            >
              <Skeleton variant="rectangular" height={28} />
            </Box>
          ))}

        {!isLoading &&
          groups?.map((g) => {
            const handleClick = () => navigate(`/admin/groups/${g.id}`);
            return (
              <Box
                key={g.id}
                role="button"
                tabIndex={0}
                aria-label={`${g.name} — open group`}
                onClick={handleClick}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    handleClick();
                  }
                }}
                sx={{
                  gridColumn: "1 / -1",
                  display: "grid",
                  gridTemplateColumns: "subgrid",
                  columnGap: 2,
                  alignItems: "center",
                  px: 2,
                  py: 1.25,
                  borderBottom: 1,
                  borderColor: "divider",
                  cursor: "pointer",
                  "&:hover": { bgcolor: "action.hover" },
                  "&:focus-visible": {
                    outline: `2px solid ${theme.palette.primary.main}`,
                    outlineOffset: -2,
                  },
                  "&:last-of-type": { borderBottom: 0 },
                }}
              >
                <Typography variant="body2" noWrap>
                  <strong>{g.name}</strong>
                </Typography>
                <Typography variant="body2" color="text.secondary" noWrap title={g.description}>
                  {g.description}
                </Typography>
                <Typography variant="body2" sx={{ textAlign: "right" }}>
                  {g.memberIds.length}
                </Typography>
              </Box>
            );
          })}
      </Box>
    </Box>
  );
}
