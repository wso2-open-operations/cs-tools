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

import type { BubbleProps } from "@features/case-types/conversations/components";
import type { MessageAuthor } from "@features/case-types/conversations/types";

import { MESSAGE_AUTHOR_TYPES, NOVERA_INITIAL_MESSAGE } from "@shared/constants";

export function useConversation(committed: BubbleProps | null, reset: () => void) {
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
            timestamp: new Date(),
            animated: false,
            thinking: false,
          }
        : message;

    setMessages((prev) => [...prev, next]);
  };

  return { messages, append };
}
