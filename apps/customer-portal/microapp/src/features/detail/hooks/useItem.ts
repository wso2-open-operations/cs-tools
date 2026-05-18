import { useQuery } from "@tanstack/react-query";

import { cases } from "@features/cases/api/cases.queries";
import { useRequiredParams } from "@features/detail/hooks";

import { useIssueType } from "@shared/hooks";

import { chats } from "../../chats/api/chats.queries";

export function useCase() {
  const { id } = useRequiredParams();
  const query = useQuery(cases.get(id));
  const { label: issueType } = useIssueType(query.data?.issueTypeId);

  return {
    ...query,
    data: query.data ? { ...query.data, issueType } : undefined,
  };
}

export function useChat() {
  const { id } = useRequiredParams();
  const query = useQuery(chats.get(id));

  return query;
}
