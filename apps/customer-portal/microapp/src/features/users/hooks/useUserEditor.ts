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

import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useMutation, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { users } from "@features/users/api/users.queries";
import { projects } from "@features/projects/api/projects.queries";
import { useProject } from "@context/project";
import { useNotify } from "@context/snackbar";
import type { Role } from "@features/users/types/user.model";

const DEFAULT_USER_ROLE: Role = "Portal User";

export function useUserEditor(mode: "invite" | "edit") {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { projectId } = useProject();
  const notify = useNotify();

  const state = location.state as { email?: string; role?: Role; firstName?: string; lastName?: string };

  const [role, setRole] = useState<Role>(
    state?.role ? (state.role === "Admin" ? DEFAULT_USER_ROLE : state.role) : DEFAULT_USER_ROLE,
  );
  const [email, setEmail] = useState(state?.email ?? "");
  const [firstName, setFirstName] = useState(state?.firstName ?? "");
  const [lastName, setLastName] = useState(state?.lastName ?? "");

  const project = useSuspenseQuery(projects.all()).data.find((p) => p.id === projectId);

  const resetAndGoBack = () => {
    queryClient.resetQueries({ queryKey: ["users", projectId] });
    navigate(-1);
  };

  const createUserMutation = useMutation({
    ...users.create(projectId!),
    onSuccess: resetAndGoBack,
    onError: () => notify.error("Failed to invite user. Please try again."),
  });

  const editUserMutation = useMutation({
    ...users.edit(projectId!, email),
    onSuccess: resetAndGoBack,
    onError: () => notify.error("Failed to edit user. Please try again."),
  });

  const deleteUserMutation = useMutation({
    ...users.delete(projectId!, email),
    onSuccess: resetAndGoBack,
    onError: () => notify.error("Failed to delete user. Please try again."),
  });

  const handleSubmit = () => {
    if (mode === "invite") {
      createUserMutation.mutate({
        contactEmail: email,
        contactFirstName: firstName,
        contactLastName: lastName,
        isCsIntegrationUser: false,
        isSecurityContact: role === "System User",
      });
    } else {
      editUserMutation.mutate({ isSecurityContact: role === "System User" });
    }
  };

  const initialRole = state?.role ? (state.role === "Admin" ? DEFAULT_USER_ROLE : state.role) : DEFAULT_USER_ROLE;
  const isRoleUnchanged = role === initialRole;

  return {
    email,
    setEmail,
    firstName,
    setFirstName,
    lastName,
    setLastName,
    role,
    setRole,
    project,
    createUserMutation,
    editUserMutation,
    deleteUserMutation,
    handleSubmit,
    isRoleUnchanged,
  };
}
