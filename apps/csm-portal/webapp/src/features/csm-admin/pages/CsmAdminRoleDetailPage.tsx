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
  Card,
  Chip,
  Skeleton,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, Trash } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useState, type JSX } from "react";
import { useNavigate, useParams } from "react-router";
import AdminTabs from "@features/csm-admin/components/AdminTabs";
import AssignmentEditor, {
  type AssignmentOption,
} from "@features/csm-admin/components/AssignmentEditor";
import ConfirmDialog from "@features/csm-admin/components/ConfirmDialog";
import {
  useDeleteCsmRole,
  useGetCsmGroups,
  useGetCsmPermissions,
  useGetCsmRoleDetail,
  useGetCsmUsers,
  useUpdateRoleAssignments,
} from "@features/csm-admin/api/useCsmAdmin";
import type {
  CsmPermission,
  CsmPermissionCategory,
} from "@features/csm-admin/types/csmAdmin";

const CATEGORY_LABEL: Record<CsmPermissionCategory, string> = {
  cases: "Cases",
  projects: "Projects",
  accounts: "Accounts",
  engagements: "Engagements",
  updates: "Updates",
  security: "Security",
  time_cards: "Time cards",
  admin: "Administration",
};

const CATEGORY_ORDER: CsmPermissionCategory[] = [
  "cases",
  "projects",
  "accounts",
  "engagements",
  "updates",
  "security",
  "time_cards",
  "admin",
];

function arraysEqualAsSets(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  const set = new Set(a);
  return b.every((v) => set.has(v));
}

export default function CsmAdminRoleDetailPage(): JSX.Element {
  const { roleId } = useParams<{ roleId: string }>();
  const navigate = useNavigate();
  const { data: role, isLoading, isError } = useGetCsmRoleDetail(roleId);
  const { data: permissions } = useGetCsmPermissions();
  const { data: users } = useGetCsmUsers();
  const { data: groups } = useGetCsmGroups();
  const update = useUpdateRoleAssignments();
  const remove = useDeleteCsmRole();
  const [confirmDelete, setConfirmDelete] = useState(false);

  const usersWithRole = useMemo(
    () =>
      (users ?? []).filter((u) => roleId && u.roleIds.includes(roleId)).map((u) => u.id),
    [users, roleId],
  );
  const groupsWithRole = useMemo(
    () =>
      (groups ?? []).filter((g) => roleId && g.roleIds.includes(roleId)).map((g) => g.id),
    [groups, roleId],
  );

  const [draftPermissionIds, setDraftPermissionIds] = useState<string[]>([]);
  const [draftUserIds, setDraftUserIds] = useState<string[]>([]);
  const [draftGroupIds, setDraftGroupIds] = useState<string[]>([]);

  useEffect(() => {
    if (role) setDraftPermissionIds(role.permissionIds);
  }, [role]);
  // Reset the cross-side drafts whenever the source data refreshes (e.g. after
  // a save). Comparing against the derived lists computed above.
  useEffect(() => {
    setDraftUserIds(usersWithRole);
  }, [usersWithRole]);
  useEffect(() => {
    setDraftGroupIds(groupsWithRole);
  }, [groupsWithRole]);

  // For the breakdown panel, group selected permissions by category.
  const permissionsById = useMemo(() => {
    const map = new Map<string, CsmPermission>();
    for (const p of permissions ?? []) map.set(p.id, p);
    return map;
  }, [permissions]);

  const grantedByCategory = useMemo<
    Record<CsmPermissionCategory, string[]>
  >(() => {
    const out: Record<CsmPermissionCategory, string[]> = {
      cases: [],
      projects: [],
      accounts: [],
      engagements: [],
      updates: [],
      security: [],
      time_cards: [],
      admin: [],
    };
    for (const id of draftPermissionIds) {
      const p = permissionsById.get(id);
      if (p) out[p.category].push(p.name);
    }
    return out;
  }, [draftPermissionIds, permissionsById]);

  const options: AssignmentOption[] = useMemo(
    () =>
      (permissions ?? []).map((p) => ({
        id: p.id,
        label: `${CATEGORY_LABEL[p.category]} · ${p.name}`,
        description: p.description,
      })),
    [permissions],
  );

  const userOptions: AssignmentOption[] = useMemo(
    () =>
      (users ?? []).map((u) => ({
        id: u.id,
        label: u.name,
        description: u.email,
      })),
    [users],
  );
  const groupOptions: AssignmentOption[] = useMemo(
    () =>
      (groups ?? []).map((g) => ({
        id: g.id,
        label: g.name,
        description: g.description,
      })),
    [groups],
  );

  const dirty =
    !!role &&
    (!arraysEqualAsSets(draftPermissionIds, role.permissionIds) ||
      !arraysEqualAsSets(draftUserIds, usersWithRole) ||
      !arraysEqualAsSets(draftGroupIds, groupsWithRole));

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <AdminTabs />
        <Skeleton variant="rectangular" height={140} />
      </Box>
    );
  }
  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <AdminTabs />
        <Typography variant="body1" color="error">
          Could not load role.
        </Typography>
      </Box>
    );
  }
  if (!role) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <AdminTabs />
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate("/admin/roles")}
          sx={{ alignSelf: "flex-start" }}
        >
          Back to roles
        </Button>
        <Typography variant="h5">Role not found</Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <AdminTabs />
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate("/admin/roles")}
        >
          Back to roles
        </Button>
        <Button
          variant="outlined"
          color="error"
          size="small"
          startIcon={<Trash size={14} />}
          disabled={role.builtIn}
          title={
            role.builtIn ? "Built-in roles cannot be deleted." : undefined
          }
          onClick={() => setConfirmDelete(true)}
        >
          Delete role
        </Button>
      </Box>
      <ConfirmDialog
        open={confirmDelete}
        title="Delete role?"
        description={
          <>
            <Typography variant="body2" sx={{ mb: 1 }}>
              Delete <strong>{role.name}</strong>?
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {usersWithRole.length === 0 && groupsWithRole.length === 0
                ? "The role isn't assigned to any users or groups. This action cannot be undone."
                : `This role is currently held by ${usersWithRole.length} user${usersWithRole.length === 1 ? "" : "s"} and ${groupsWithRole.length} group${groupsWithRole.length === 1 ? "" : "s"}. The role will be removed from each. This action cannot be undone.`}
            </Typography>
          </>
        }
        confirmLabel="Delete role"
        confirmColor="error"
        pending={remove.isPending}
        onCancel={() => setConfirmDelete(false)}
        onConfirm={() => {
          remove.mutate(role.id, {
            onSuccess: () => navigate("/admin/roles"),
          });
        }}
      />

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Typography variant="h5">{role.name}</Typography>
          <Chip
            size="small"
            variant="outlined"
            label={role.builtIn ? "Built-in" : "Custom"}
            color={role.builtIn ? "default" : "primary"}
          />
          <Chip
            size="small"
            label={`${draftPermissionIds.length} permissions`}
            variant="outlined"
          />
        </Box>
        <Typography variant="body2" color="text.secondary">
          {role.description}
        </Typography>
      </Box>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Permissions</Typography>
        <Typography variant="caption" color="text.secondary">
          Add or remove permissions granted by this role. Users assigned this role inherit the changes.
        </Typography>
        <AssignmentEditor
          value={draftPermissionIds}
          options={options}
          onChange={setDraftPermissionIds}
          emptyLabel="No permissions granted. Users with only this role will have no access."
          itemNoun="permissions"
          disabled={update.isPending}
        />
      </Card>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
        <Typography variant="subtitle2">Breakdown by category</Typography>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "repeat(2, minmax(0, 1fr))" },
            gap: 1.5,
          }}
        >
          {CATEGORY_ORDER.map((cat) => {
            const items = grantedByCategory[cat];
            return (
              <Box
                key={cat}
                sx={{
                  border: 1,
                  borderColor: "divider",
                  borderRadius: 1,
                  p: 1.25,
                  minHeight: 56,
                }}
              >
                <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
                  {CATEGORY_LABEL[cat]} · {items.length}
                </Typography>
                {items.length === 0 ? (
                  <Typography variant="body2" color="text.secondary">
                    —
                  </Typography>
                ) : (
                  <Typography variant="body2">{items.join(", ")}</Typography>
                )}
              </Box>
            );
          })}
        </Box>
      </Card>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Users with this role</Typography>
        <Typography variant="caption" color="text.secondary">
          Edit from the role side. Adding a user here adds this role to their direct role assignments.
        </Typography>
        <AssignmentEditor
          value={draftUserIds}
          options={userOptions}
          onChange={setDraftUserIds}
          emptyLabel="No users have this role directly. Members of groups carrying this role still inherit it."
          itemNoun="users"
          disabled={update.isPending}
        />
      </Card>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Groups with this role</Typography>
        <Typography variant="caption" color="text.secondary">
          Edit from the role side. Every member of these groups inherits this role.
        </Typography>
        <AssignmentEditor
          value={draftGroupIds}
          options={groupOptions}
          onChange={setDraftGroupIds}
          emptyLabel="No groups carry this role."
          itemNoun="groups"
          disabled={update.isPending}
        />
      </Card>

      <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1 }}>
        <Button
          variant="text"
          disabled={!dirty || update.isPending}
          onClick={() => {
            if (role) {
              setDraftPermissionIds(role.permissionIds);
              setDraftUserIds(usersWithRole);
              setDraftGroupIds(groupsWithRole);
            }
          }}
        >
          Revert
        </Button>
        <Button
          variant="contained"
          color="primary"
          disabled={!dirty || update.isPending}
          onClick={() =>
            update.mutate({
              roleId: role.id,
              permissionIds: draftPermissionIds,
              userIds: draftUserIds,
              groupIds: draftGroupIds,
            })
          }
        >
          {update.isPending ? "Saving…" : "Save changes"}
        </Button>
      </Box>
    </Box>
  );
}
