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
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, Trash } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useState, type JSX, type ReactNode } from "react";
import { useNavigate, useParams } from "react-router";
import AssignmentEditor, {
  type AssignmentOption,
} from "@features/csm-admin/components/AssignmentEditor";
import ConfirmDialog from "@features/csm-admin/components/ConfirmDialog";
import {
  useDeleteCsmUser,
  useGetCsmGroups,
  useGetCsmPermissions,
  useGetCsmRoles,
  useGetCsmUserDetail,
  useUpdateUserAssignments,
} from "@features/csm-admin/api/useCsmAdmin";
import type { CsmPermissionCategory } from "@features/csm-admin/types/csmAdmin";
import { formatRelativeTime } from "@features/csm-dashboard/utils/abtDashboard";

function MetaCell({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Box>{children}</Box>
    </Box>
  );
}

function arraysEqualAsSets(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  const set = new Set(a);
  return b.every((v) => set.has(v));
}

export default function CsmAdminUserDetailPage(): JSX.Element {
  const { userId } = useParams<{ userId: string }>();
  const navigate = useNavigate();
  const { data: user, isLoading, isError } = useGetCsmUserDetail(userId);
  const { data: roles } = useGetCsmRoles();
  const { data: groups } = useGetCsmGroups();
  const { data: permissions } = useGetCsmPermissions();
  const update = useUpdateUserAssignments();
  const remove = useDeleteCsmUser();
  const [confirmDelete, setConfirmDelete] = useState(false);

  const [draftRoleIds, setDraftRoleIds] = useState<string[]>([]);
  const [draftGroupIds, setDraftGroupIds] = useState<string[]>([]);

  // Hydrate draft when the user record loads / changes.
  useEffect(() => {
    if (user) {
      setDraftRoleIds(user.roleIds);
      setDraftGroupIds(user.groupIds);
    }
  }, [user]);

  const roleOptions: AssignmentOption[] = useMemo(
    () =>
      (roles ?? []).map((r) => ({
        id: r.id,
        label: r.name,
        description: r.description,
      })),
    [roles],
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
    !!user &&
    (!arraysEqualAsSets(draftRoleIds, user.roleIds) ||
      !arraysEqualAsSets(draftGroupIds, user.groupIds));

  // Compute the effective view from the *draft* state so the user can preview
  // changes before saving.
  const groupsById = useMemo(
    () => new Map((groups ?? []).map((g) => [g.id, g])),
    [groups],
  );
  const rolesById = useMemo(
    () => new Map((roles ?? []).map((r) => [r.id, r])),
    [roles],
  );

  // For each effective roleId, list the sources: "direct" or "via <GroupName>".
  const effectiveRoles = useMemo<
    { roleId: string; sources: string[] }[]
  >(() => {
    const sourceMap = new Map<string, string[]>();
    for (const rid of draftRoleIds) {
      const existing = sourceMap.get(rid) ?? [];
      existing.push("direct");
      sourceMap.set(rid, existing);
    }
    for (const gid of draftGroupIds) {
      const g = groupsById.get(gid);
      if (!g) continue;
      for (const rid of g.roleIds) {
        const existing = sourceMap.get(rid) ?? [];
        existing.push(`via ${g.name}`);
        sourceMap.set(rid, existing);
      }
    }
    return Array.from(sourceMap.entries()).map(([roleId, sources]) => ({
      roleId,
      sources,
    }));
  }, [draftRoleIds, draftGroupIds, groupsById]);

  const permissionsById = useMemo(
    () => new Map((permissions ?? []).map((p) => [p.id, p])),
    [permissions],
  );

  const effectivePermissionsByCategory = useMemo<
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
    const seenPermIds = new Set<string>();
    for (const { roleId } of effectiveRoles) {
      const role = rolesById.get(roleId);
      if (!role) continue;
      for (const pid of role.permissionIds) {
        if (seenPermIds.has(pid)) continue;
        seenPermIds.add(pid);
        const p = permissionsById.get(pid);
        if (!p) continue;
        out[p.category].push(p.name);
      }
    }
    return out;
  }, [effectiveRoles, rolesById, permissionsById]);

  const handleSave = () => {
    if (!user) return;
    update.mutate({
      userId: user.id,
      roleIds: draftRoleIds,
      groupIds: draftGroupIds,
    });
  };

  const handleRevert = () => {
    if (user) {
      setDraftRoleIds(user.roleIds);
      setDraftGroupIds(user.groupIds);
    }
  };

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rectangular" height={140} />
      </Box>
    );
  }
  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="body1" color="error">
          Could not load user.
        </Typography>
      </Box>
    );
  }
  if (!user) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate("/admin/users")}
          sx={{ alignSelf: "flex-start" }}
        >
          Back to users
        </Button>
        <Typography variant="h5">User not found</Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate("/admin/users")}
        >
          Back to users
        </Button>
        <Button
          variant="outlined"
          color="error"
          size="small"
          startIcon={<Trash size={14} />}
          onClick={() => setConfirmDelete(true)}
        >
          Delete user
        </Button>
      </Box>
      <ConfirmDialog
        open={confirmDelete}
        title="Delete user?"
        description={
          <>
            <Typography variant="body2" sx={{ mb: 1 }}>
              Delete <strong>{user.name}</strong> ({user.email})?
            </Typography>
            <Typography variant="caption" color="text.secondary">
              This removes the user from every group. Role assignments on the user record
              are removed. This action cannot be undone.
            </Typography>
          </>
        }
        confirmLabel="Delete user"
        confirmColor="error"
        pending={remove.isPending}
        onCancel={() => setConfirmDelete(false)}
        onConfirm={() => {
          remove.mutate(user.id, {
            onSuccess: () => navigate("/admin/users"),
          });
        }}
      />

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Typography variant="h5">{user.name}</Typography>
          <Chip
            size="small"
            variant="outlined"
            label={user.status}
            color={
              user.status === "Active"
                ? "success"
                : user.status === "Invited"
                  ? "warning"
                  : "default"
            }
          />
        </Box>
        <Typography variant="body2" color="text.secondary">
          {user.email} · IdP id <code>{user.externalId}</code>
        </Typography>
      </Box>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Profile</Typography>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr 1fr", md: "repeat(4, minmax(0, 1fr))" },
            gap: 2,
          }}
        >
          <MetaCell label="Status">
            <Typography variant="body2">{user.status}</Typography>
          </MetaCell>
          <MetaCell label="Roles">
            <Typography variant="body2">{user.roleIds.length}</Typography>
          </MetaCell>
          <MetaCell label="Groups">
            <Typography variant="body2">{user.groupIds.length}</Typography>
          </MetaCell>
          <MetaCell label="Last active">
            <Typography variant="body2">{formatRelativeTime(user.lastActiveAt)}</Typography>
          </MetaCell>
        </Box>
      </Card>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Roles</Typography>
        <Typography variant="caption" color="text.secondary">
          Roles grant permissions. A user gets the union of permissions across all assigned roles.
        </Typography>
        <AssignmentEditor
          value={draftRoleIds}
          options={roleOptions}
          onChange={setDraftRoleIds}
          emptyLabel="No roles assigned. User will have no permissions."
          itemNoun="roles"
          disabled={update.isPending}
        />
      </Card>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Groups</Typography>
        <Typography variant="caption" color="text.secondary">
          Groups are typically ABTs or cross-cutting collaborator pools. Membership is bidirectional.
        </Typography>
        <AssignmentEditor
          value={draftGroupIds}
          options={groupOptions}
          onChange={setDraftGroupIds}
          emptyLabel="Not a member of any group."
          itemNoun="groups"
          disabled={update.isPending}
        />
      </Card>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
          <Typography variant="subtitle2">Effective access</Typography>
          {dirty && (
            <Chip
              size="small"
              variant="outlined"
              color="warning"
              label="Preview — unsaved changes"
            />
          )}
        </Box>
        <Typography variant="caption" color="text.secondary">
          Computed from the role assignments above. Roles come from direct assignment plus the roles attached to each group the user belongs to.
        </Typography>

        <Box>
          <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
            Effective roles ({effectiveRoles.length})
          </Typography>
          {effectiveRoles.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No effective roles. User has no access.
            </Typography>
          ) : (
            <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.75, mt: 0.5 }}>
              {effectiveRoles.map(({ roleId, sources }) => {
                const role = rolesById.get(roleId);
                const label = role?.name ?? roleId;
                const inherited =
                  sources.length === 1 && sources[0] !== "direct";
                return (
                  <Tooltip
                    key={roleId}
                    title={sources.join(", ")}
                    arrow
                    placement="top"
                  >
                    <Chip
                      size="small"
                      label={label}
                      variant={inherited ? "outlined" : "filled"}
                      color={inherited ? "default" : "primary"}
                    />
                  </Tooltip>
                );
              })}
            </Box>
          )}
        </Box>

        <Box>
          <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600 }}>
            Effective permissions
          </Typography>
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", md: "repeat(2, minmax(0, 1fr))" },
              gap: 1.5,
              mt: 0.5,
            }}
          >
            {(
              [
                "cases",
                "projects",
                "accounts",
                "engagements",
                "updates",
                "security",
                "time_cards",
                "admin",
              ] as CsmPermissionCategory[]
            ).map((cat) => {
              const items = effectivePermissionsByCategory[cat];
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
                    {cat === "time_cards" ? "Time cards" : cat.charAt(0).toUpperCase() + cat.slice(1)} · {items.length}
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
        </Box>
      </Card>

      <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1 }}>
        <Button
          variant="text"
          disabled={!dirty || update.isPending}
          onClick={handleRevert}
        >
          Revert
        </Button>
        <Button
          variant="contained"
          color="primary"
          disabled={!dirty || update.isPending}
          onClick={handleSave}
        >
          {update.isPending ? "Saving…" : "Save changes"}
        </Button>
      </Box>
    </Box>
  );
}
