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

import { infiniteQueryOptions, mutationOptions, queryOptions } from "@tanstack/react-query";
import type { GetChatsRequestDto, MessageDispatchDto } from "@features/chats/types/chat.dto";
import { getAllChats, getChat, getChatComments, initiateChat, sendChatMessage } from "@features/chats/api/chats.api";

export const chats = {
  initiate: (id: string) =>
    mutationOptions({
      mutationFn: (body: Omit<MessageDispatchDto, "region" | "tier">) => initiateChat(id, body),
    }),

  send: (id: string, conversationId: string) =>
    mutationOptions({
      mutationFn: (body: Omit<MessageDispatchDto, "region" | "tier">) => sendChatMessage(id, conversationId, body),
    }),

  get: (id: string) => queryOptions({ queryKey: ["chat", id], queryFn: () => getChat(id) }),

  all: (id: string, body: GetChatsRequestDto = {}) =>
    queryOptions({ queryKey: ["chats", id, body], queryFn: () => getAllChats(id, body) }),

  paginated: (id: string, body: GetChatsRequestDto = {}) =>
    infiniteQueryOptions({
      queryKey: ["chats", "paginated", id, body],
      queryFn: ({ pageParam }) => getAllChats(id, { ...body, pagination: { ...body.pagination, offset: pageParam } }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => {
        const { offset, limit, totalRecords } = lastPage.pagination;
        const nextOffset = offset + 1;
        return nextOffset >= Math.ceil(totalRecords / limit) ? undefined : nextOffset;
      },
    }),

  comments: (id: string) => queryOptions({ queryKey: ["chat-comments", id], queryFn: () => getChatComments(id) }),
};
