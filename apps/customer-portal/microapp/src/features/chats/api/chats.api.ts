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

import apiClient from "@infrastructure/api/client";
import type { PaginatedArray } from "@shared/types";
import type {
  ChatDto,
  ChatsDto,
  GetChatsRequestDto,
  MessageDispatchDto,
  MessageResponseDto,
} from "@features/chats/types/chat.dto";
import type { Chat, Message } from "@features/chats/types/chat.model";
import { toChat, toMessage } from "@features/chats/mappers/chat.mapper";
import { toComment } from "@features/cases/mappers/case.mapper";
import type { Comment } from "@features/cases/types/case.model";
import type { CommentsDto } from "@features/cases/types/case.dto";
import {
  CHAT_ADD_MESSAGE_ENDPOINT,
  CHAT_COMMENTS_ENDPOINT,
  CHAT_DETAILS_ENDPOINT,
  CHAT_INITIATE_ENDPOINT,
  PROJECT_CHATS_ENDPOINT,
} from "@config/endpoints";

export const initiateChat = async (id: string, data: Omit<MessageDispatchDto, "region" | "tier">): Promise<Message> => {
  const response = (
    await apiClient.post<MessageResponseDto>(CHAT_INITIATE_ENDPOINT(id), {
      ...data,
      region: "EU", // TODO: Remove hardcoded
      tier: "Tier 1", // TODO: Remove hardcoded
    })
  ).data;
  return toMessage(response, "incoming");
};

export const sendChatMessage = async (
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

export const getChat = async (id: string): Promise<Chat> => {
  const response = (await apiClient.get<ChatDto>(CHAT_DETAILS_ENDPOINT(id))).data;
  return toChat(response);
};

export const getAllChats = async (id: string, body: GetChatsRequestDto = {}): Promise<PaginatedArray<Chat>> => {
  const response = (await apiClient.post<ChatsDto>(PROJECT_CHATS_ENDPOINT(id), body)).data;
  const result = response.conversations.map(toChat) as PaginatedArray<Chat>;
  result.pagination = { totalRecords: response.totalRecords, offset: response.offset, limit: response.limit };
  return result;
};

export const getChatComments = async (id: string): Promise<Comment[]> => {
  const response = (await apiClient.get<CommentsDto>(CHAT_COMMENTS_ENDPOINT(id))).data;
  return response.comments.map(toComment);
};
