import { useEffect, useState } from "react";

import type { ChatMessage, MessageAuthor } from "@features/chats/components";

import { useDateTime } from "@shared/hooks/useDateTime";

import { NOVERA_INITIAL_MESSAGE } from "@shared/constants";

export function useConversation(committed: ChatMessage | null, reset: () => void) {
  const { fromNow } = useDateTime();
  const [messages, setMessages] = useState<ChatMessage[]>([NOVERA_INITIAL_MESSAGE]);

  useEffect(() => {
    if (committed) {
      append(committed, "assistant");
      reset();
    }
  }, [committed]);

  const append = (message: string | ChatMessage, author: MessageAuthor = "you") => {
    const next: ChatMessage =
      typeof message === "string"
        ? {
            author,
            blocks: [{ type: "text", value: message }],
            animated: false,
            thinking: false,
            timestamp: fromNow(new Date()),
          }
        : message;

    setMessages((prev) => [...prev, next]);
  };

  return { messages, append };
}
