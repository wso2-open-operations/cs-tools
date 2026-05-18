import { useQuery } from "@tanstack/react-query";

import { cases } from "@features/cases/api/cases.queries";
import { changeRequests } from "@features/changes/api/changes.queries";
import { chats } from "@features/chats/api/chats.queries";
import { useRequiredParams } from "@features/detail/hooks";
import { serviceRequests } from "@features/service-requests/api/service-requests.queries";

import { useIssueType } from "@shared/hooks";

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

export function useServiceRequest() {
  const { id } = useRequiredParams();
  const query = useQuery(serviceRequests.get(id));

  return query;
}

export function useChangeRequest() {
  const { id } = useRequiredParams();
  const query = useQuery(changeRequests.get(id));

  return query;
}
