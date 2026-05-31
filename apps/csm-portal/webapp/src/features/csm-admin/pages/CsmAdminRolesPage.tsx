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

import { Box, Button, Chip, Skeleton, Typography, useTheme } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useNavigate } from "react-router";
import CreateRoleDialog from "@features/csm-admin/components/CreateRoleDialog";
import {
  useGetCsmRoles,
  useGetCsmUsers,
} from "@features/csm-admin/api/useCsmAdmin";

const GRID = "minmax(180px, 1.4fr) minmax(280px, 2.5fr) auto auto auto";

const HEADER_CELLS: { label: string; align?: "left" | "right" }[] = [
  { label: "Role" },
  { label: "Description" },
  { label: "Type" },
  { label: "Permissions", align: "right" },
  { label: "Assigned to", align: "right" },
];

export default function CsmAdminRolesPage(): JSX.Element {
  const navigate = useNavigate();
  const theme = useTheme();
  const { data: roles, isLoading } = useGetCsmRoles();
  const { data: users } = useGetCsmUsers();
  const [createOpen, setCreateOpen] = useState(false);

  const usageByRoleId = useMemo<Map<string, number>>(() => {
    const counts = new Map<string, number>();
    for (const u of users ?? []) {
      for (const rid of u.roleIds) {
        counts.set(rid, (counts.get(rid) ?? 0) + 1);
      }
    }
    return counts;
  }, [users]);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>

      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2 }}>
        <Typography variant="body2" color="text.secondary">
          {isLoading ? "Loading…" : `${roles?.length ?? 0} roles`}
        </Typography>
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => setCreateOpen(true)}
        >
          Create role
        </Button>
      </Box>
      <CreateRoleDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(r) => navigate(`/admin/roles/${r.id}`)}
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
          [0, 1, 2].map((i) => (
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
          roles?.map((r) => {
            const handleClick = () => navigate(`/admin/roles/${r.id}`);
            return (
              <Box
                key={r.id}
                role="button"
                tabIndex={0}
                aria-label={`${r.name} — open role`}
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
                  <strong>{r.name}</strong>
                </Typography>
                <Typography variant="body2" color="text.secondary" noWrap title={r.description}>
                  {r.description}
                </Typography>
                <Chip
                  size="small"
                  variant="outlined"
                  label={r.builtIn ? "Built-in" : "Custom"}
                  color={r.builtIn ? "default" : "primary"}
                />
                <Typography variant="body2" sx={{ textAlign: "right" }}>
                  {r.permissionIds.length}
                </Typography>
                <Typography variant="body2" sx={{ textAlign: "right" }} color="text.secondary">
                  {usageByRoleId.get(r.id) ?? 0}
                </Typography>
              </Box>
            );
          })}
      </Box>
    </Box>
  );
}
