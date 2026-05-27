import { useMutation } from "@tanstack/react-query";

import { cases } from "@features/cases/api/cases.queries";
import type { BubbleProps } from "@features/chats/components";

import { useNavigation } from "@shared/hooks";

export function useClassify(messages: BubbleProps[]) {
  const { toClassifiedCaseCreate } = useNavigation();

  return useMutation({
    ...cases.classify,
    onSuccess: (response) => toClassifiedCaseCreate(messages, response),
  });
}
