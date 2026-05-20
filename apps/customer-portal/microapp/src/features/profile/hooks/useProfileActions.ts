import { useMutation, useQueryClient } from "@tanstack/react-query";

import { useProject } from "@context/project";
import { useNotify } from "@context/snackbar";

import { projects } from "@features/projects/api/projects.queries";
import type { ProjectInfo } from "@features/projects/types/project.model";

export function useProfileMutations() {
  const notify = useNotify();
  const queryClient = useQueryClient();
  const { projectId } = useProject();

  const edit = useMutation({
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

  return { edit };
}
