import { useQuery } from "@tanstack/react-query";

import { useRequiredParams } from "@features/detail/hooks";
import { engagements } from "@features/engagements/api/engagements.queries";

export function useCallRequests() {
  const { id } = useRequiredParams();
  const { data, ...query } = useQuery(engagements.callRequests(id));

  return { ...query, data: data?.callRequests };
}
