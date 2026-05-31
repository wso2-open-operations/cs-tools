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
  useDeleteCsmGroup,
  useGetCsmGroupDetail,
  useGetCsmRoles,
  useGetCsmUsers,
  useUpdateGroupAssignments,
} from "@features/csm-admin/api/useCsmAdmin";

function arraysEqualAsSets(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  const set = new Set(a);
  return b.every((v) => set.has(v));
}

export default function CsmAdminGroupDetailPage(): JSX.Element {
  const { groupId } = useParams<{ groupId: string }>();
  const navigate = useNavigate();
  const { data: group, isLoading, isError } = useGetCsmGroupDetail(groupId);
  const { data: users } = useGetCsmUsers();
  const { data: roles } = useGetCsmRoles();
  const update = useUpdateGroupAssignments();
  const remove = useDeleteCsmGroup();
  const [confirmDelete, setConfirmDelete] = useState(false);

  const [draftMemberIds, setDraftMemberIds] = useState<string[]>([]);
  const [draftRoleIds, setDraftRoleIds] = useState<string[]>([]);
  useEffect(() => {
    if (group) {
      setDraftMemberIds(group.memberIds);
      setDraftRoleIds(group.roleIds);
    }
  }, [group]);

  const userOptions: AssignmentOption[] = useMemo(
    () =>
      (users ?? []).map((u) => ({
        id: u.id,
        label: u.name,
        description: u.email,
      })),
    [users],
  );

  const roleOptions: AssignmentOption[] = useMemo(
    () =>
      (roles ?? []).map((r) => ({
        id: r.id,
        label: r.name,
        description: r.description,
      })),
    [roles],
  );

  const dirty =
    !!group &&
    (!arraysEqualAsSets(draftMemberIds, group.memberIds) ||
      !arraysEqualAsSets(draftRoleIds, group.roleIds));

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
          Could not load group.
        </Typography>
      </Box>
    );
  }
  if (!group) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <AdminTabs />
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate("/admin/groups")}
          sx={{ alignSelf: "flex-start" }}
        >
          Back to groups
        </Button>
        <Typography variant="h5">Group not found</Typography>
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
          onClick={() => navigate("/admin/groups")}
        >
          Back to groups
        </Button>
        <Button
          variant="outlined"
          color="error"
          size="small"
          startIcon={<Trash size={14} />}
          onClick={() => setConfirmDelete(true)}
        >
          Delete group
        </Button>
      </Box>
      <ConfirmDialog
        open={confirmDelete}
        title="Delete group?"
        description={
          <>
            <Typography variant="body2" sx={{ mb: 1 }}>
              Delete <strong>{group.name}</strong>?
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {group.memberIds.length > 0
                ? `This group has ${group.memberIds.length} member${group.memberIds.length === 1 ? "" : "s"}. Each will have this group removed from their membership. The action cannot be undone.`
                : "The group has no members. This action cannot be undone."}
            </Typography>
          </>
        }
        confirmLabel="Delete group"
        confirmColor="error"
        pending={remove.isPending}
        onCancel={() => setConfirmDelete(false)}
        onConfirm={() => {
          remove.mutate(group.id, {
            onSuccess: () => navigate("/admin/groups"),
          });
        }}
      />

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Typography variant="h5">{group.name}</Typography>
          <Chip
            size="small"
            variant="outlined"
            label={`${draftMemberIds.length} members`}
          />
          <Chip
            size="small"
            variant="outlined"
            color="primary"
            label={`${draftRoleIds.length} roles`}
          />
        </Box>
        <Typography variant="body2" color="text.secondary">
          {group.description}
        </Typography>
      </Box>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Members</Typography>
        <Typography variant="caption" color="text.secondary">
          Add or remove users from this group. The user's group list updates accordingly (relation is bidirectional).
        </Typography>
        <AssignmentEditor
          value={draftMemberIds}
          options={userOptions}
          onChange={setDraftMemberIds}
          emptyLabel="No members. Group has no scope."
          itemNoun="users"
          disabled={update.isPending}
        />
      </Card>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Roles</Typography>
        <Typography variant="caption" color="text.secondary">
          Roles granted to every member of this group. Members get the union of these roles plus any roles assigned to them directly.
        </Typography>
        <AssignmentEditor
          value={draftRoleIds}
          options={roleOptions}
          onChange={setDraftRoleIds}
          emptyLabel="No roles. Members inherit nothing from this group."
          itemNoun="roles"
          disabled={update.isPending}
        />
      </Card>

      <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1 }}>
        <Button
          variant="text"
          disabled={!dirty || update.isPending}
          onClick={() => {
            if (group) {
              setDraftMemberIds(group.memberIds);
              setDraftRoleIds(group.roleIds);
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
              groupId: group.id,
              memberIds: draftMemberIds,
              roleIds: draftRoleIds,
            })
          }
        >
          {update.isPending ? "Saving…" : "Save changes"}
        </Button>
      </Box>
    </Box>
  );
}
