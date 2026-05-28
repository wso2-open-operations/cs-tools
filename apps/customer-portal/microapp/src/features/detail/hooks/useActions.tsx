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
import { CheckIcon, PlusIcon, RotateCcwIcon } from "@wso2/oxygen-ui-icons-react";

import { useNotify } from "@context/snackbar";

import { cases } from "@features/case-types/cases/api/cases.queries";
import type { SlotActionsOptionProps } from "@features/detail/components";
import { useCase, useRequiredParams } from "@features/detail/hooks";

import { STATUS_ALLOWING_REJECTION, STATUS_ALLOWING_RELATED_CASE, STATUS_ALLOWING_RESOLUTION } from "@shared/constants";
import { useNavigation } from "@shared/hooks";

export function useActions(): SlotActionsOptionProps[] {
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
