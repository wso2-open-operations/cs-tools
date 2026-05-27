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
