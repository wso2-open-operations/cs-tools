import { useQuery } from "@tanstack/react-query";

import { engagements } from "@features/case-types/engagements/api/engagements.queries";
import { useRequiredParams } from "@features/detail/hooks";

export function useCallRequests() {
  const { id } = useRequiredParams();
  const { data, ...query } = useQuery(engagements.callRequests(id));

  return { ...query, data: data?.callRequests };
}
