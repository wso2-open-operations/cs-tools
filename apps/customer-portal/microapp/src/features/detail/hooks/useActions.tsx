import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CheckIcon, PlusIcon, RotateCcwIcon } from "@wso2/oxygen-ui-icons-react";

import { useNotify } from "@context/snackbar";

import { cases } from "@features/cases/api/cases.queries";
import { useCase, useRequiredParams } from "@features/detail/hooks";

import type { MenuOptionProps } from "@shared/components/detail";

import { STATUS_ALLOWING_REJECTION, STATUS_ALLOWING_RELATED_CASE, STATUS_ALLOWING_RESOLUTION } from "@shared/constants";
import { useNavigation } from "@shared/hooks";

export function useActions(): MenuOptionProps[] {
  const notify = useNotify();
  const queryClient = useQueryClient();

  const { data } = useCase();
  const { id } = useRequiredParams();
  const { toRelativeCaseCreate } = useNavigation();

  const mutation = useMutation({
    ...cases.edit(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: cases.get(id).queryKey }),
    onError: () => notify.error("Failed to update case. Please try again."),
  });

  if (!data) return [];

  return [
    {
      label: "Mark as Resolved",
      color: "success",
      icon: <CheckIcon />,
      hidden: !STATUS_ALLOWING_RESOLUTION.includes(data.statusId),
      onClick: () => mutation.mutate({ stateKey: 3 }),
    },
    {
      label: "Reject Solution",
      color: "error",
      icon: <RotateCcwIcon />,
      hidden: !STATUS_ALLOWING_REJECTION.includes(data.statusId),
      onClick: () => mutation.mutate({ stateKey: 1003 }),
    },
    {
      label: "Created Related Case",
      icon: <PlusIcon />,
      hidden: !STATUS_ALLOWING_RELATED_CASE.includes(data.statusId),
      onClick: () => toRelativeCaseCreate(data),
    },
  ];
}
