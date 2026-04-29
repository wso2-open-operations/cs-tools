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

import { parseApiDate } from "@shared/utils/date.utils";
import type { ChatDto, MessageResponseDto } from "@features/chats/types/chat.dto";
import type { Chat, Message } from "@features/chats/types/chat.model";

export function toMessage(dto: MessageResponseDto, direction: "outgoing" | "incoming"): Message {
  return {
    content: dto.message,
    direction,
    conversationId: dto.conversationId,
    timestamp: new Date(),
  };
}

export function toChat(dto: ChatDto): Chat {
  return {
    id: dto.id,
    number: dto.number,
    description: dto.initialMessage,
    count: dto.messageCount,
    createdOn: parseApiDate(dto.createdOn),
    createdBy: dto.createdBy,
    statusId: dto.state?.id,
  };
}
