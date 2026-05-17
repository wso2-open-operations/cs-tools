import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowLeftRightIcon, CheckIcon, PlusIcon } from "@wso2/oxygen-ui-icons-react";

import { useNotify } from "@context/snackbar";

import { cases } from "@features/cases/api/cases.queries";
import { useCase, useRequiredParams } from "@features/detail/hooks";

import type { MenuOptionProps } from "@shared/components/detail";

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
      hidden: !["1", "10", "1003", "6", "18", "1006"].includes(data.statusId),
      onClick: () => mutation.mutate({ stateKey: 3 }),
    },
    {
      label: "Mark as Waiting on WSO2",
      color: "warning",
      icon: <ArrowLeftRightIcon />,
      hidden: !["6", "18"].includes(data.statusId),
      onClick: () => mutation.mutate({ stateKey: 1003 }),
    },
    {
      label: "Created Related Case",
      icon: <PlusIcon />,
      hidden: !["3"].includes(data.statusId),
      onClick: () => toRelativeCaseCreate(data),
    },
  ];
}
