import type { BubbleProps } from "@features/chats/components";

import { MESSAGE_AUTHOR_TYPES } from "@shared/constants";

export const toTranscript = (messages: BubbleProps[]): string => {
  return messages
    .map((msg) => {
      const role = msg.author === MESSAGE_AUTHOR_TYPES.USER ? "User" : "Assistant";
      return `${role}: ${msg.content}`;
    })
    .join("\n");
};
