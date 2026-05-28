// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
import { useEffect, useState } from "react";

import type { BubbleAgentProps } from "@features/case-types/conversations/components";
import type { FinalNoveraResponse, NoveraResponse } from "@features/case-types/conversations/types";

import { MESSAGE_AUTHOR_TYPES } from "@shared/constants";

export function useStream() {
  const [draft, setDraft] = useState<BubbleAgentProps | null>(null); // Message currently being displayed
  const [pending, setPending] = useState<FinalNoveraResponse | null>(null);
  const [committed, setCommitted] = useState<BubbleAgentProps | null>(null); // ChatMessage object awaiting to be appended
  const [animationComplete, setAnimationComplete] = useState(false); // Animation has finished rendering the text

  useEffect(() => {
    if (!pending || (draft && !animationComplete)) return;

    setCommitted({
      author: MESSAGE_AUTHOR_TYPES.AGENT,
      content: pending.payload.message,
      timestamp: new Date(),
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
