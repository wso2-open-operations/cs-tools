import { useEffect, useState } from "react";

import type { BubbleProps } from "@features/chats/components";
import type { MessageAuthor } from "@features/chats/types";

import { useDateTime } from "@shared/hooks/useDateTime";

import { MESSAGE_AUTHOR_TYPES, NOVERA_INITIAL_MESSAGE } from "@shared/constants";

export function useConversation(committed: BubbleProps | null, reset: () => void) {
  const { fromNow } = useDateTime();
  const [messages, setMessages] = useState<BubbleProps[]>([
    { ...NOVERA_INITIAL_MESSAGE, animated: false, thinking: false },
  ]);

  useEffect(() => {
    if (committed) {
      append(committed, MESSAGE_AUTHOR_TYPES.AGENT);
      reset();
    }
  }, [committed]);

  const append = (message: string | BubbleProps, author: MessageAuthor = MESSAGE_AUTHOR_TYPES.USER) => {
    const next: BubbleProps =
      typeof message === "string"
        ? {
            author,
            content: message,
            timestamp: fromNow(new Date()),
            animated: false,
            thinking: false,
          }
        : message;

    setMessages((prev) => [...prev, next]);
  };

  return { messages, append };
}
