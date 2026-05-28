import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { cases } from "@features/case-types/cases/api/cases.queries";
import { useCase } from "@features/detail/hooks";

export function useComments() {
  const id = useCase().data?.id;
  const queryClient = useQueryClient();

  const {
    data: comments,
    isFetching,
    isLoading,
  } = useQuery({
    ...cases.comments(id!),
    enabled: !!id,
    select: (data) => [...data].sort((a, b) => a.createdOn.getTime() - b.createdOn.getTime()),
  });

  const mutation = useMutation({
    ...cases.createComment(id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: cases.comments(id!).queryKey });
    },
  });

  const isPending = mutation.status !== "idle" && mutation.isPending && isFetching;

  const submit = (value: string) => {
    if (!id || !value.trim()) return;
    mutation.mutate({ content: value, type: "comments" });
  };

  return { comments, isLoading, submit, isPending };
}
