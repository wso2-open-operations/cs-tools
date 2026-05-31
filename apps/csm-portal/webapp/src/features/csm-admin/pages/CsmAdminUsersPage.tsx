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
import CreateUserDialog from "@features/csm-admin/components/CreateUserDialog";
import {
  useGetCsmGroups,
  useGetCsmRoles,
  useGetCsmUsers,
} from "@features/csm-admin/api/useCsmAdmin";
import { formatRelativeTime } from "@features/csm-dashboard/utils/abtDashboard";
import type {
  CsmGroup,
  CsmRole,
  CsmUserStatus,
} from "@features/csm-admin/types/csmAdmin";

const GRID = "minmax(180px, 1.5fr) minmax(220px, 1.6fr) auto auto auto auto";

const HEADER_CELLS: { label: string; align?: "left" | "right" }[] = [
  { label: "Name" },
  { label: "Email" },
  { label: "Status" },
  { label: "Roles", align: "right" },
  { label: "Groups", align: "right" },
  { label: "Last active", align: "right" },
];

function StatusChip({ status }: { status: CsmUserStatus }): JSX.Element {
  const color: "success" | "warning" | "default" =
    status === "Active"
      ? "success"
      : status === "Invited"
        ? "warning"
        : "default";
  return <Chip size="small" variant="outlined" label={status} color={color} />;
}

export default function CsmAdminUsersPage(): JSX.Element {
  const navigate = useNavigate();
  const theme = useTheme();
  const { data: users, isLoading } = useGetCsmUsers();
  const { data: roles } = useGetCsmRoles();
  const { data: groups } = useGetCsmGroups();
  const [createOpen, setCreateOpen] = useState(false);

  const roleById = useMemo<Map<string, CsmRole>>(
    () => new Map((roles ?? []).map((r) => [r.id, r])),
    [roles],
  );
  const groupById = useMemo<Map<string, CsmGroup>>(
    () => new Map((groups ?? []).map((g) => [g.id, g])),
    [groups],
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>

      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2 }}>
        <Typography variant="body2" color="text.secondary">
          {isLoading ? "Loading…" : `${users?.length ?? 0} users`}
        </Typography>
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => setCreateOpen(true)}
        >
          Invite user
        </Button>
      </Box>
      <CreateUserDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(u) => navigate(`/admin/users/${u.id}`)}
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
          [0, 1, 2, 3].map((i) => (
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
          users?.map((u) => {
            const handleClick = () => navigate(`/admin/users/${u.id}`);
            const roleLabels = u.roleIds
              .map((id) => roleById.get(id)?.name)
              .filter((v): v is string => !!v);
            const groupLabels = u.groupIds
              .map((id) => groupById.get(id)?.name)
              .filter((v): v is string => !!v);
            return (
              <Box
                key={u.id}
                role="button"
                tabIndex={0}
                aria-label={`${u.name} — open user`}
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
                <Box sx={{ minWidth: 0 }}>
                  <Typography variant="body2" noWrap>
                    <strong>{u.name}</strong>
                  </Typography>
                  <Typography variant="caption" color="text.secondary" noWrap title={roleLabels.join(", ")}>
                    {roleLabels.join(", ") || "No roles"}
                  </Typography>
                </Box>
                <Typography variant="body2" noWrap>
                  {u.email}
                </Typography>
                <StatusChip status={u.status} />
                <Typography variant="body2" sx={{ textAlign: "right" }} title={roleLabels.join(", ")}>
                  {u.roleIds.length}
                </Typography>
                <Typography variant="body2" sx={{ textAlign: "right" }} title={groupLabels.join(", ")}>
                  {u.groupIds.length}
                </Typography>
                <Typography
                  variant="caption"
                  color="text.secondary"
                  sx={{ textAlign: "right" }}
                  noWrap
                >
                  {formatRelativeTime(u.lastActiveAt)}
                </Typography>
              </Box>
            );
          })}
      </Box>
    </Box>
  );
}
