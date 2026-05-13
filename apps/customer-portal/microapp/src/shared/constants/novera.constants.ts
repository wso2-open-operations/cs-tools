import type { ChatMessage } from "@features/chats/components";

export const NOVERA_INITIAL_MESSAGE: ChatMessage = {
  animated: false,
  thinking: false,
  author: "assistant",
  blocks: [
    {
      type: "text",
      value:
        "Hi! I'm Novera, your AI-powered support assistant. How can I help you today? Please describe the issue you're experiencing.",
    },
  ],
};
