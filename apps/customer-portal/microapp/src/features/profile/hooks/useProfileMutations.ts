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
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { useProject } from "@context/project";
import { useNotify } from "@context/snackbar";

import { projects } from "@features/projects/api/projects.queries";
import type { ProjectInfo } from "@features/projects/types/project.model";
import { users } from "@features/users/api/users.queries";

export function useProfileMutations() {
  const notify = useNotify();
  const queryClient = useQueryClient();
  const { projectId } = useProject();

  const editProject = useMutation({
    ...projects.edit(projectId!),
    onMutate: (variables) => {
      queryClient.setQueryData(projects.get(projectId!).queryKey, (old: ProjectInfo | undefined) => {
        if (!old) return old;

        return {
          ...old,
          ...(variables.hasAgent !== undefined && { agentEnabled: variables.hasAgent }),
          ...(variables.hasKbReferences !== undefined && { kbReferencesEnabled: variables.hasKbReferences }),
        };
      });
    },
    onError: () => {
      notify.error("Failed to update project. Please try again.");
      queryClient.invalidateQueries({ queryKey: projects.get(projectId!).queryKey });
    },
  });

  const editMe = useMutation({
    ...users.editMe(),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: users.me().queryKey });
    },
    onError: () => notify.error("Failed to update profile. Please try again."),
  });

  return { editProject, editMe };
}
