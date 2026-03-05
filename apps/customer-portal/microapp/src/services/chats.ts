import apiClient from "@src/services/apiClient";
import { mutationOptions, queryOptions } from "@tanstack/react-query";
import type {
  ChatDTO,
  ChatsDTO,
  GetChatsRequestDTO,
  MessageDispatchDTO,
  MessageResponseDTO,
} from "@src/types/chat.dto";
import type { Chat, Message } from "@src/types/chat.model";

import {
  CHAT_ADD_MESSAGE_ENDPOINT,
  CHAT_COMMENTS_ENDPOINT,
  CHAT_INITIATE_ENDPOINT,
  PROJECT_CHATS_ENDPOINT,
} from "@config/endpoints";
import type { Comment, CommentsDTO } from "../types";
import { toComment } from "./cases";

const initiate = async (id: string, data: Omit<MessageDispatchDTO, "region" | "tier">): Promise<Message> => {
  const response = (
    await apiClient.post<MessageResponseDTO>(CHAT_INITIATE_ENDPOINT(id), {
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
  data: Omit<MessageDispatchDTO, "region" | "tier">,
): Promise<Message> => {
  const response = (
    await apiClient.post<MessageResponseDTO>(CHAT_ADD_MESSAGE_ENDPOINT(id, conversationId), {
      ...data,
      region: "EU", // TODO: Remove hardcoded
      tier: "Tier 1", // TODO: Remove hardcoded
    })
  ).data;

  return toMessage(response, "incoming");
};

const getAllChats = async (id: string, body: GetChatsRequestDTO = {}): Promise<Chat[]> => {
  const cases = (await apiClient.post<ChatsDTO>(PROJECT_CHATS_ENDPOINT(id), body)).data.conversations;
  return cases.map(toChat);
};

const getComments = async (id: string): Promise<Comment[]> => {
  const response = (await apiClient.get<CommentsDTO>(CHAT_COMMENTS_ENDPOINT(id))).data;
  return response.comments.map(toComment);
};

/* Mappers */
function toMessage(dto: MessageResponseDTO, direction: "outgoing" | "incoming"): Message {
  return {
    content: dto.message,
    direction: direction,
    conversationId: dto.conversationId,
    timestamp: new Date(),
  };
}

function toChat(dto: ChatDTO): Chat {
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
      mutationFn: (body: Omit<MessageDispatchDTO, "region" | "tier">) => initiate(id, body),
    }),

  send: (id: string, conversationId: string) =>
    mutationOptions({
      mutationFn: (body: Omit<MessageDispatchDTO, "region" | "tier">) => send(id, conversationId, body),
    }),

  all: (id: string, body: GetChatsRequestDTO = {}) =>
    queryOptions({
      queryKey: ["chats", id, body],
      queryFn: () => getAllChats(id, body),
    }),

  comments: (id: string) => queryOptions({ queryKey: ["chat-comments", id], queryFn: () => getComments(id) }),
};
