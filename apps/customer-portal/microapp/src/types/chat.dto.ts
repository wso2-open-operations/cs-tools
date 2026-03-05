import type { EntityReference } from "./case.dto";
import type { Pagination } from "./pagination.types";

export interface MessageDispatchDTO {
  message: string;
  envProducts: Record<string, string[]>;
  region: string;
  tier: string;
}

export interface MessageResponseDTO {
  message: string;
  sessionId: string;
  conversationId: string;
  resolved: boolean | null;
  // TODO: Add recommendations: { query: string; recommendations: { title: string; articleId: string; score: number }[] } | null;
}

export interface ChatsDTO {
  conversations: ChatDTO[];
}

export interface ChatDTO extends Pagination {
  id: string;
  number: string;
  initialMessage: string;
  messageCount: number;
  createdOn: string;
  createdBy: string;
  state: EntityReference | null;
}

export interface GetChatsRequestDTO {
  pagination?: {
    limit?: number;
    offset?: number;
  };
  sortBy?: {
    field?: string;
    order?: "asc" | "desc";
  };
}
