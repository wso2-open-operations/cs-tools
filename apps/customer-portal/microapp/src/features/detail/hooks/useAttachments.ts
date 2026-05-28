import { useQuery } from "@tanstack/react-query";

import { cases } from "@features/case-types/cases/api/cases.queries";
import { useCase } from "@features/detail/hooks";

export function useAttachments() {
  const { data } = useCase();

  return useQuery({
    ...cases.attachments(data?.id!),
    enabled: !!data?.id,
  });
}
