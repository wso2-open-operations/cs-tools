import { useMutation, useQueryClient } from "@tanstack/react-query";

import { useProject } from "@context/project";
import { useNotify } from "@context/snackbar";

import { users } from "@features/users/api/users.queries";
import { useMode } from "@features/users/hooks";

import { useNavigation } from "@shared/hooks";

export function useUserMutations() {
  const notify = useNotify();
  const queryClient = useQueryClient();
  const { back } = useNavigation();
  const { projectId } = useProject();
  const { initial } = useMode();

  const handleSuccess = () => {
    queryClient.resetQueries({ queryKey: ["users", projectId] });
    back();
  };

  const create = useMutation({
    ...users.create(projectId!),
    onSuccess: handleSuccess,
    onError: () => notify.error("Failed to invite user. Please try again."),
  });

  const edit = useMutation({
    ...users.edit(projectId!, initial!.email),
    onSuccess: handleSuccess,
    onError: () => notify.error("Failed to edit user. Please try again."),
  });

  const remove = useMutation({
    ...users.delete(projectId!, initial!.email),
    onSuccess: handleSuccess,
    onError: () => notify.error("Failed to delete user. Please try again."),
  });

  return { create, edit, remove };
}
