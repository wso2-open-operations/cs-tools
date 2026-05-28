import { useEffect, useState } from "react";

import type { BubbleAgentProps } from "@features/case-types/conversations/components";
import type { FinalNoveraResponse, NoveraResponse } from "@features/case-types/conversations/types";

import { useDateTime } from "@shared/hooks/useDateTime";

import { MESSAGE_AUTHOR_TYPES } from "@shared/constants";

export function useStream() {
  const { fromNow } = useDateTime();

  const [draft, setDraft] = useState<BubbleAgentProps | null>(null); // Message currently being displayed
  const [pending, setPending] = useState<FinalNoveraResponse | null>(null);
  const [committed, setCommitted] = useState<BubbleAgentProps | null>(null); // ChatMessage object awaiting to be appended
  const [animationComplete, setAnimationComplete] = useState(false); // Animation has finished rendering the text

  useEffect(() => {
    if (!pending || (draft && !animationComplete)) return;

    setCommitted({
      author: MESSAGE_AUTHOR_TYPES.AGENT,
      content: pending.payload.message,
      timestamp: fromNow(new Date()),
      animated: !draft,
      thinking: false,
    });

    setDraft(null);
    setPending(null);
    setAnimationComplete(false);
  }, [pending, draft, animationComplete]);

  const handleResponse = (response: NoveraResponse) => {
    switch (response.type) {
      case "thinking_start":
        setDraft({
          author: MESSAGE_AUTHOR_TYPES.AGENT,
          content: "",
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
          if (!prev)
            return {
              author: MESSAGE_AUTHOR_TYPES.AGENT,
              content: response.content,
              thinking: true,
              animated: true,
            };

          return {
            ...prev,
            content: prev.content + response.content,
          };
        });
        break;

      case "final":
        setPending(response as FinalNoveraResponse);
        break;

      default:
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
