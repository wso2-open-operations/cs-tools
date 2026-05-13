import { useEffect, useState } from "react";

import type { ChatMessage } from "@features/chats/components";
import type { FinalNoveraResponse, NoveraResponse } from "@features/chats/types/novera.dto";

import { useDateTime } from "@shared/hooks/useDateTime";

export function useStream() {
  const { fromNow } = useDateTime();

  const [draft, setDraft] = useState<ChatMessage | null>(null); // Message currently being displayed
  const [pending, setPending] = useState<FinalNoveraResponse | null>(null);
  const [committed, setCommitted] = useState<ChatMessage | null>(null); // ChatMessage object awaiting to be appended
  const [animationComplete, setAnimationComplete] = useState(false); // Animation has finished rendering the text

  useEffect(() => {
    if (!pending || !draft || !animationComplete) return;

    setCommitted({
      animated: false,
      thinking: false,
      author: "assistant",
      blocks: [{ type: "text", value: pending.payload.message }],
      timestamp: fromNow(new Date()),
    });

    setDraft(null);
    setPending(null);
    setAnimationComplete(false);
  }, [pending, draft, animationComplete]);

  const handleResponse = (response: NoveraResponse) => {
    switch (response.type) {
      case "thinking_start":
        setDraft({
          author: "assistant",
          blocks: [{ type: "text", value: "" }],
          thinking: true,
          animated: true,
        });

        setPending(null);
        setCommitted(null);
        setAnimationComplete(false);

        break;

      case "thinking_step":
        setDraft((prev) => (prev ? { ...prev, thinking: response.label } : null));
        break;

      case "token":
        setAnimationComplete(false);
        setDraft((prev) => {
          if (!prev) return null;
          return {
            ...prev,
            blocks: prev.blocks.map((b) => (b.type === "text" ? { ...b, value: b.value + response.content } : b)),
          };
        });
        break;

      case "final":
        setPending(response as FinalNoveraResponse);
        break;
    }
  };

  const reset = () => {
    setDraft(null);
    setPending(null);
    setCommitted(null);
  };

  const finish = () => setAnimationComplete(true);

  return { draft, committed, pending: Boolean(draft || pending), stream: handleResponse, finish, reset };
}
