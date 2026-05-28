import { useQuery } from "@tanstack/react-query";

import { chats } from "@features/case-types/conversations/api/chats.queries";
import { useRequiredParams } from "@features/detail/hooks";

export function useConversation() {
  const { id } = useRequiredParams();
  const query = useQuery(chats.comments(id));

  return query;
}
