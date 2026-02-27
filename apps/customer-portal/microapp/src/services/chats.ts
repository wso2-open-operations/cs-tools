import apiClient from "@src/services/apiClient";
import { mutationOptions } from "@tanstack/react-query";
import type { MessageDispatchDTO, MessageResponseDTO } from "@src/types/chat.dto";
import type { Message } from "@src/types/chat.model";

import { CHAT_ADD_MESSAGE_ENDPOINT, CHAT_INITIATE_ENDPOINT } from "@config/endpoints";

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

/* Mappers */
function toMessage(dto: MessageResponseDTO, direction: "outgoing" | "incoming"): Message {
  return {
    content: dto.message,
    direction: direction,
    conversationId: dto.conversationId,
    timestamp: new Date(),
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
};
