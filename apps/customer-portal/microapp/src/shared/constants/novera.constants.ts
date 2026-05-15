import type { BubbleProps } from "@features/chats/components";
import { MESSAGE_AUTHOR_TYPES } from "@shared/constants";

export const NOVERA_INITIAL_MESSAGE: BubbleProps = {
  author: MESSAGE_AUTHOR_TYPES.AGENT,
  content: "Hi! I'm Novera, your AI-powered support assistant. How can I help you today? Please describe the issue you're experiencing.",
  animated: false,
  thinking: false,
};
