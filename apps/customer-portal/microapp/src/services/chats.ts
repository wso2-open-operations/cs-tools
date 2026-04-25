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

import apiClient from "@src/services/apiClient";
import { infiniteQueryOptions, mutationOptions, queryOptions } from "@tanstack/react-query";
import type {
  ChatDto,
  ChatsDto,
  GetChatsRequestDto,
  MessageDispatchDto,
  MessageResponseDto,
} from "@src/types/chat.dto";
import type { Chat, Message } from "@src/types/chat.model";

import {
  CHAT_ADD_MESSAGE_ENDPOINT,
  CHAT_COMMENTS_ENDPOINT,
  CHAT_DETAILS_ENDPOINT,
  CHAT_INITIATE_ENDPOINT,
  PROJECT_CHATS_ENDPOINT,
} from "@config/endpoints";
import type { Comment, CommentsDto, PaginatedArray } from "../types";
import { toComment } from "./cases";

const initiate = async (id: string, data: Omit<MessageDispatchDto, "region" | "tier">): Promise<Message> => {
  const response = (
    await apiClient.post<MessageResponseDto>(CHAT_INITIATE_ENDPOINT(id), {
      ...data,
      region: "EU", // TODO: Remove hardcoded
      tier: "Tier 1", // TODO: Remove hardcoded
    })
  ).data;

  return toMessage(response, "incoming");
};

const send = async (
  id: string,
  conversationId: string,
  data: Omit<MessageDispatchDto, "region" | "tier">,
): Promise<Message> => {
  const response = (
    await apiClient.post<MessageResponseDto>(CHAT_ADD_MESSAGE_ENDPOINT(id, conversationId), {
      ...data,
      region: "EU", // TODO: Remove hardcoded
      tier: "Tier 1", // TODO: Remove hardcoded
    })
  ).data;

  return toMessage(response, "incoming");
};

const getChat = async (id: string): Promise<Chat> => {
  const response = (await apiClient.get<ChatDto>(CHAT_DETAILS_ENDPOINT(id))).data;
  return toChat(response);
};

const getAllChats = async (id: string, body: GetChatsRequestDto = {}): Promise<PaginatedArray<Chat>> => {
  const response = (await apiClient.post<ChatsDto>(PROJECT_CHATS_ENDPOINT(id), body)).data;
  const result = response.conversations.map(toChat) as PaginatedArray<Chat>;
  result.pagination = {
    totalRecords: response.totalRecords,
    offset: response.offset,
    limit: response.limit,
  };

  return result;
};

const getComments = async (id: string): Promise<Comment[]> => {
  const response = (await apiClient.get<CommentsDto>(CHAT_COMMENTS_ENDPOINT(id))).data;
  return response.comments.map(toComment);
};

/* Mappers */
function toMessage(dto: MessageResponseDto, direction: "outgoing" | "incoming"): Message {
  return {
    content: dto.message,
    direction: direction,
    conversationId: dto.conversationId,
    timestamp: new Date(),
  };
}

function toChat(dto: ChatDto): Chat {
  return {
    id: dto.id,
    number: dto.number,
    description: dto.initialMessage,
    count: dto.messageCount,
    createdOn: new Date(dto.createdOn.replace(" ", "T")),
    createdBy: dto.createdBy,
    statusId: dto.state?.id,
  };
}

/* Query Options */
export const chats = {
  initiate: (id: string) =>
    mutationOptions({
      mutationFn: (body: Omit<MessageDispatchDto, "region" | "tier">) => initiate(id, body),
    }),

  send: (id: string, conversationId: string) =>
    mutationOptions({
      mutationFn: (body: Omit<MessageDispatchDto, "region" | "tier">) => send(id, conversationId, body),
    }),

  get: (id: string) =>
    queryOptions({
      queryKey: ["chat", id],
      queryFn: () => getChat(id),
    }),

  all: (id: string, body: GetChatsRequestDto = {}) =>
    queryOptions({
      queryKey: ["chats", id, body],
      queryFn: () => getAllChats(id, body),
    }),

  paginated: (id: string, body: GetChatsRequestDto = {}) =>
    infiniteQueryOptions({
      queryKey: ["chats", "paginated", id, body],
      queryFn: ({ pageParam }) => getAllChats(id, { ...body, pagination: { ...body.pagination, offset: pageParam } }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => {
        const { offset, limit, totalRecords } = lastPage.pagination;
        const nextOffset = offset + 1;
        const totalPages = Math.ceil(totalRecords / limit);
        return nextOffset >= totalPages ? undefined : nextOffset;
      },
    }),

  comments: (id: string) => queryOptions({ queryKey: ["chat-comments", id], queryFn: () => getComments(id) }),
};
