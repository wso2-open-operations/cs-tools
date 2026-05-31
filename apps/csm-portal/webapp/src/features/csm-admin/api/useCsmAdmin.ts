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
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import {
  createGroup,
  createRole,
  createUser,
  deleteGroup,
  deleteRole,
  deleteUser,
  getMockGroupById,
  getMockGroups,
  getMockPermissions,
  getMockRoleById,
  getMockRoles,
  getMockUserById,
  getMockUsers,
  setGroupMembers,
  setGroupRoles,
  setRoleGroups,
  setRolePermissions,
  setRoleUsers,
  setUserGroups,
  setUserRoles,
  type CreateGroupInput,
  type CreateRoleInput,
  type CreateUserInput,
} from "@features/csm-admin/api/mocks/adminMocks";
import type {
  CsmGroup,
  CsmPermission,
  CsmRole,
  CsmUser,
} from "@features/csm-admin/types/csmAdmin";

const MOCK_LATENCY_MS = 120;

function isMock(): boolean {
  return !!window.config?.CSM_PORTAL_USE_MOCKS;
}

function backendBaseUrl(): string {
  const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
  if (!baseUrl) throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
  return baseUrl;
}

// ---------- Users ----------

export function useGetCsmUsers(): UseQueryResult<CsmUser[], Error> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();

  return useQuery<CsmUser[], Error>({
    queryKey: [ApiQueryKeys.CSM_ADMIN_USERS],
    queryFn: async () => {
      if (isMock()) {
        logger.debug("[useGetCsmUsers] mock");
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockUsers();
      }
      const r = await authFetch(`${backendBaseUrl()}/csm/admin/users`, { method: "GET" });
      if (!r.ok) throw new Error(`Failed to load users: ${r.statusText}`);
      return (await r.json()) as CsmUser[];
    },
    staleTime: 30_000,
  });
}

export function useGetCsmUserDetail(
  userId: string | undefined,
): UseQueryResult<CsmUser | null, Error> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();

  return useQuery<CsmUser | null, Error>({
    queryKey: [ApiQueryKeys.CSM_ADMIN_USER_DETAIL, userId ?? ""],
    queryFn: async () => {
      if (!userId) return null;
      if (isMock()) {
        logger.debug(`[useGetCsmUserDetail] mock ${userId}`);
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockUserById(userId) ?? null;
      }
      const r = await authFetch(
        `${backendBaseUrl()}/csm/admin/users/${encodeURIComponent(userId)}`,
        { method: "GET" },
      );
      if (r.status === 404) return null;
      if (!r.ok) throw new Error(`Failed to load user: ${r.statusText}`);
      return (await r.json()) as CsmUser;
    },
    enabled: !!userId,
    staleTime: 30_000,
  });
}

export interface SetUserAssignmentsInput {
  userId: string;
  roleIds: string[];
  groupIds: string[];
}

export function useUpdateUserAssignments(): UseMutationResult<
  CsmUser,
  Error,
  SetUserAssignmentsInput
> {
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();
  return useMutation<CsmUser, Error, SetUserAssignmentsInput>({
    mutationFn: async (input) => {
      if (isMock()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        setUserRoles(input.userId, input.roleIds);
        return setUserGroups(input.userId, input.groupIds);
      }
      const r = await authFetch(
        `${backendBaseUrl()}/csm/admin/users/${encodeURIComponent(input.userId)}/assignments`,
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            roleIds: input.roleIds,
            groupIds: input.groupIds,
          }),
        },
      );
      if (!r.ok) throw new Error(`Failed to save assignments: ${r.statusText}`);
      return (await r.json()) as CsmUser;
    },
    onSuccess: (user) => {
      queryClient.setQueryData<CsmUser | null | undefined>(
        [ApiQueryKeys.CSM_ADMIN_USER_DETAIL, user.id],
        user,
      );
      // The list view shows role/group counts derived from the user — refetch.
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_USERS] });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_GROUPS] });
    },
  });
}

// ---------- Roles ----------

export function useGetCsmRoles(): UseQueryResult<CsmRole[], Error> {
  const authFetch = useAuthApiClient();
  return useQuery<CsmRole[], Error>({
    queryKey: [ApiQueryKeys.CSM_ADMIN_ROLES],
    queryFn: async () => {
      if (isMock()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockRoles();
      }
      const r = await authFetch(`${backendBaseUrl()}/csm/admin/roles`, { method: "GET" });
      if (!r.ok) throw new Error(`Failed to load roles: ${r.statusText}`);
      return (await r.json()) as CsmRole[];
    },
    staleTime: 30_000,
  });
}

export function useGetCsmRoleDetail(
  roleId: string | undefined,
): UseQueryResult<CsmRole | null, Error> {
  const authFetch = useAuthApiClient();
  return useQuery<CsmRole | null, Error>({
    queryKey: [ApiQueryKeys.CSM_ADMIN_ROLE_DETAIL, roleId ?? ""],
    queryFn: async () => {
      if (!roleId) return null;
      if (isMock()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockRoleById(roleId) ?? null;
      }
      const r = await authFetch(
        `${backendBaseUrl()}/csm/admin/roles/${encodeURIComponent(roleId)}`,
        { method: "GET" },
      );
      if (r.status === 404) return null;
      if (!r.ok) throw new Error(`Failed to load role: ${r.statusText}`);
      return (await r.json()) as CsmRole;
    },
    enabled: !!roleId,
    staleTime: 30_000,
  });
}

export interface SetRoleAssignmentsInput {
  roleId: string;
  permissionIds: string[];
  /** Set of user ids that should hold this role (edited from the role side). */
  userIds: string[];
  /** Set of group ids that should hold this role. */
  groupIds: string[];
}

export function useUpdateRoleAssignments(): UseMutationResult<
  CsmRole,
  Error,
  SetRoleAssignmentsInput
> {
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();
  return useMutation<CsmRole, Error, SetRoleAssignmentsInput>({
    mutationFn: async (input) => {
      if (isMock()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        const updated = setRolePermissions(input.roleId, input.permissionIds);
        setRoleUsers(input.roleId, input.userIds);
        setRoleGroups(input.roleId, input.groupIds);
        return updated;
      }
      const r = await authFetch(
        `${backendBaseUrl()}/csm/admin/roles/${encodeURIComponent(input.roleId)}/assignments`,
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            permissionIds: input.permissionIds,
            userIds: input.userIds,
            groupIds: input.groupIds,
          }),
        },
      );
      if (!r.ok) throw new Error(`Failed to save role assignments: ${r.statusText}`);
      return (await r.json()) as CsmRole;
    },
    onSuccess: (role) => {
      queryClient.setQueryData<CsmRole | null | undefined>(
        [ApiQueryKeys.CSM_ADMIN_ROLE_DETAIL, role.id],
        role,
      );
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_ROLES] });
      // Side-effects on users and groups (they reference this role).
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_USERS] });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_GROUPS] });
    },
  });
}

// ---------- Groups ----------

export function useGetCsmGroups(): UseQueryResult<CsmGroup[], Error> {
  const authFetch = useAuthApiClient();
  return useQuery<CsmGroup[], Error>({
    queryKey: [ApiQueryKeys.CSM_ADMIN_GROUPS],
    queryFn: async () => {
      if (isMock()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockGroups();
      }
      const r = await authFetch(`${backendBaseUrl()}/csm/admin/groups`, { method: "GET" });
      if (!r.ok) throw new Error(`Failed to load groups: ${r.statusText}`);
      return (await r.json()) as CsmGroup[];
    },
    staleTime: 30_000,
  });
}

export function useGetCsmGroupDetail(
  groupId: string | undefined,
): UseQueryResult<CsmGroup | null, Error> {
  const authFetch = useAuthApiClient();
  return useQuery<CsmGroup | null, Error>({
    queryKey: [ApiQueryKeys.CSM_ADMIN_GROUP_DETAIL, groupId ?? ""],
    queryFn: async () => {
      if (!groupId) return null;
      if (isMock()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockGroupById(groupId) ?? null;
      }
      const r = await authFetch(
        `${backendBaseUrl()}/csm/admin/groups/${encodeURIComponent(groupId)}`,
        { method: "GET" },
      );
      if (r.status === 404) return null;
      if (!r.ok) throw new Error(`Failed to load group: ${r.statusText}`);
      return (await r.json()) as CsmGroup;
    },
    enabled: !!groupId,
    staleTime: 30_000,
  });
}

export interface SetGroupAssignmentsInput {
  groupId: string;
  memberIds: string[];
  roleIds: string[];
}

export function useUpdateGroupAssignments(): UseMutationResult<
  CsmGroup,
  Error,
  SetGroupAssignmentsInput
> {
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();
  return useMutation<CsmGroup, Error, SetGroupAssignmentsInput>({
    mutationFn: async (input) => {
      if (isMock()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        setGroupMembers(input.groupId, input.memberIds);
        return setGroupRoles(input.groupId, input.roleIds);
      }
      const r = await authFetch(
        `${backendBaseUrl()}/csm/admin/groups/${encodeURIComponent(input.groupId)}/assignments`,
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            memberIds: input.memberIds,
            roleIds: input.roleIds,
          }),
        },
      );
      if (!r.ok) throw new Error(`Failed to save group assignments: ${r.statusText}`);
      return (await r.json()) as CsmGroup;
    },
    onSuccess: (group) => {
      queryClient.setQueryData<CsmGroup | null | undefined>(
        [ApiQueryKeys.CSM_ADMIN_GROUP_DETAIL, group.id],
        group,
      );
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_GROUPS] });
      // User detail / user list show group memberships — invalidate them.
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_USERS] });
    },
  });
}

// ---------- Create / Delete ----------

export function useCreateCsmUser(): UseMutationResult<
  CsmUser,
  Error,
  CreateUserInput
> {
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();
  return useMutation<CsmUser, Error, CreateUserInput>({
    mutationFn: async (input) => {
      if (isMock()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return createUser(input);
      }
      const r = await authFetch(`${backendBaseUrl()}/csm/admin/users`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(input),
      });
      if (!r.ok) throw new Error(`Failed to create user: ${r.statusText}`);
      return (await r.json()) as CsmUser;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_USERS] });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_GROUPS] });
    },
  });
}

export function useDeleteCsmUser(): UseMutationResult<void, Error, string> {
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: async (userId) => {
      if (isMock()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        deleteUser(userId);
        return;
      }
      const r = await authFetch(
        `${backendBaseUrl()}/csm/admin/users/${encodeURIComponent(userId)}`,
        { method: "DELETE" },
      );
      if (!r.ok) throw new Error(`Failed to delete user: ${r.statusText}`);
    },
    onSuccess: (_void, userId) => {
      queryClient.removeQueries({
        queryKey: [ApiQueryKeys.CSM_ADMIN_USER_DETAIL, userId],
      });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_USERS] });
      // Group membership references the deleted user, so refetch groups.
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_GROUPS] });
    },
  });
}

export function useCreateCsmRole(): UseMutationResult<
  CsmRole,
  Error,
  CreateRoleInput
> {
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();
  return useMutation<CsmRole, Error, CreateRoleInput>({
    mutationFn: async (input) => {
      if (isMock()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return createRole(input);
      }
      const r = await authFetch(`${backendBaseUrl()}/csm/admin/roles`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(input),
      });
      if (!r.ok) throw new Error(`Failed to create role: ${r.statusText}`);
      return (await r.json()) as CsmRole;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_ROLES] });
    },
  });
}

export function useDeleteCsmRole(): UseMutationResult<void, Error, string> {
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: async (roleId) => {
      if (isMock()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        deleteRole(roleId);
        return;
      }
      const r = await authFetch(
        `${backendBaseUrl()}/csm/admin/roles/${encodeURIComponent(roleId)}`,
        { method: "DELETE" },
      );
      if (!r.ok) throw new Error(`Failed to delete role: ${r.statusText}`);
    },
    onSuccess: (_void, roleId) => {
      queryClient.removeQueries({
        queryKey: [ApiQueryKeys.CSM_ADMIN_ROLE_DETAIL, roleId],
      });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_ROLES] });
      // User role assignments reference the deleted role.
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_USERS] });
    },
  });
}

export function useCreateCsmGroup(): UseMutationResult<
  CsmGroup,
  Error,
  CreateGroupInput
> {
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();
  return useMutation<CsmGroup, Error, CreateGroupInput>({
    mutationFn: async (input) => {
      if (isMock()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return createGroup(input);
      }
      const r = await authFetch(`${backendBaseUrl()}/csm/admin/groups`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(input),
      });
      if (!r.ok) throw new Error(`Failed to create group: ${r.statusText}`);
      return (await r.json()) as CsmGroup;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_GROUPS] });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_USERS] });
    },
  });
}

export function useDeleteCsmGroup(): UseMutationResult<void, Error, string> {
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: async (groupId) => {
      if (isMock()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        deleteGroup(groupId);
        return;
      }
      const r = await authFetch(
        `${backendBaseUrl()}/csm/admin/groups/${encodeURIComponent(groupId)}`,
        { method: "DELETE" },
      );
      if (!r.ok) throw new Error(`Failed to delete group: ${r.statusText}`);
    },
    onSuccess: (_void, groupId) => {
      queryClient.removeQueries({
        queryKey: [ApiQueryKeys.CSM_ADMIN_GROUP_DETAIL, groupId],
      });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_GROUPS] });
      // User memberships reference the deleted group.
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_ADMIN_USERS] });
    },
  });
}

// ---------- Permissions ----------

export function useGetCsmPermissions(): UseQueryResult<CsmPermission[], Error> {
  const authFetch = useAuthApiClient();
  return useQuery<CsmPermission[], Error>({
    queryKey: [ApiQueryKeys.CSM_ADMIN_PERMISSIONS],
    queryFn: async () => {
      if (isMock()) {
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockPermissions();
      }
      const r = await authFetch(`${backendBaseUrl()}/csm/admin/permissions`, { method: "GET" });
      if (!r.ok) throw new Error(`Failed to load permissions: ${r.statusText}`);
      return (await r.json()) as CsmPermission[];
    },
    staleTime: 5 * 60_000,
  });
}
