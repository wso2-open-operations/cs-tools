import { useQuery } from "@tanstack/react-query";

import { chats } from "@features/chats/api/chats.queries";
import { useRequiredParams } from "@features/detail/hooks";

export function useConversation() {
  const { id } = useRequiredParams();
  const query = useQuery(chats.comments(id));

  return query;
}
