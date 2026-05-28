import type { MESSAGE_AUTHOR_TYPES } from "@shared/constants";

export type MessageAuthor = (typeof MESSAGE_AUTHOR_TYPES)[keyof typeof MESSAGE_AUTHOR_TYPES];

export interface ChatMessage {
  author: MessageAuthor;
  content: string;
  timestamp?: Date;
}
