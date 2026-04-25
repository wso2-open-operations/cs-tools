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

import type { CaseCommentInlineAttachment } from "@features/support/types/attachments";
import type {
  AuditMetadata,
  IdLabelRef,
  PaginationResponse,
  SearchRequestBase,
  SharedEnvContext,
} from "@/types/common";

// Item type for a support chat session summary.
export type ChatHistoryItem = {
  chatId: string;
  chatNumber?: string;
  title: string;
  startedTime: string;
  messages: number;
  kbArticles: number;
  status: string;
};

// Response type for project chat history list.
export type ChatHistoryResponse = {
  chatHistory: ChatHistoryItem[];
};

// Response type for conversation statistics.
export type ConversationStats = {
  abandonedCount: number;
  openCount: number;
  resolvedCount: number;
};

// Item type for a single conversation.
export type Conversation = AuditMetadata & {
  id: string;
  number: string | null;
  initialMessage: string | null;
  messageCount: number;
  project: IdLabelRef | null;
  case: IdLabelRef | null;
  state: IdLabelRef | null;
};

// Response type for conversations search results.
export type ConversationSearchResponse = PaginationResponse & {
  conversations: Conversation[];
};

// Item type for a single conversation message.
export type ConversationMessage = AuditMetadata & {
  id: string;
  content: string;
  type: string;
  isEscalated: boolean;
  hasInlineAttachments: boolean;
  inlineAttachments: CaseCommentInlineAttachment[];
  createdByFirstName?: string | null;
  createdByLastName?: string | null;
};

// Response type for conversation messages list.
export type ConversationMessagesResponse = PaginationResponse & {
  comments: ConversationMessage[];
};

// Model type for all conversations search filter values state.
export type AllConversationsFilterValues = {
  stateId?: string;
};

/**
 * Row CTA for a conversation card (list view).
 */
export enum ConversationListRowAction {
  View = "view",
  Resume = "resume",
}

export type AllConversationsListProps = {
  conversations: Conversation[];
  isLoading: boolean;
  isError?: boolean;
  hasListRefinement?: boolean;
  onConversationClick?: (conversation: Conversation) => void;
};

// Item type for select-type user input collection slot option.
export type SelectSlotOption = {
  slot: string;
  label: string;
  options: string[];
  type: "select";
};

// Item type for free-text user input collection slot option.
export type TextSlotOption = {
  slot: string;
  label: string;
  type: "text";
  freeText?: true;
};

// Model type for user input collection slot option union.
export type SlotOption = SelectSlotOption | TextSlotOption;

// Model type for slot state containing filled/missing slots and available options.
export type SlotState = {
  intentId?: string;
  filledSlots?: Record<string, string>;
  missingSlots?: string[];
  isComplete?: boolean;
  slotOptions?: SlotOption[];
};

// Item type for intent information from conversation response.
export type ConversationIntent = {
  intentId?: string;
  intentLabel?: string;
  confidence?: number;
  severity?: string;
  caseType?: string;
};

// Response type for recommendation results.
export type RecommendationResult = {
  query: string;
  recommendations: Recommendation[];
};

export enum NoveraActionType {
  SolutionProposed = "solutionProposed",
  SolutionWorked = "solutionWorked",
}

// Item type for a UI action button returned in a Novera final event.
export type NoveraAction = {
  type: NoveraActionType | string;
  label: string;
  style?: "primary" | "danger";
  payload?: Record<string, unknown>;
};

// Response type for chat conversation (Novera).
export type ConversationResponse = {
  message: string;
  sessionId: string;
  conversationId: string;
  intent?: ConversationIntent;
  slotState?: SlotState;
  actions: unknown;
  recommendations?: RecommendationResult | null;
  resolved: unknown;
};

// Filter type for searching conversations.
export type ConversationSearchFilters = {
  searchQuery?: string;
  stateKeys?: number[];
  createdByMe?: boolean;
};

// Request type for searching conversations.
export type ConversationSearchRequest = SearchRequestBase & {
  filters?: ConversationSearchFilters;
};

// Request type for chat conversation (Novera).
export type ConversationRequest = SharedEnvContext & {
  message: string;
};

// Enum for chat sender type.
export enum ChatSender {
  USER = "user",
  BOT = "bot",
}

// Item type for a recommendation article.
export type Recommendation = {
  title: string;
  articleId: string;
  score: number;
};

// Item type for a single UI chat message.
export type Message = {
  id: string;
  text: string;
  sender: ChatSender;
  isCurrentUser?: boolean;
  timestamp: Date;
  createdBy?: string;
  createdOnRaw?: string;
  showFeedbackActions?: boolean;
  showCreateCaseAction?: boolean;
  isLoading?: boolean;
  isError?: boolean;
  slotState?: SlotState;
  recommendations?: Recommendation[];
  thinkingSteps?: string[];
  thinkingLabel?: string | null;
  isStreaming?: boolean;
  actions?: NoveraAction[];
};

// Model type for chat navigation state.
export type ChatNavState = {
  initialUserMessage?: string;
  conversationResponse?: ConversationResponse;
  initialEnvProducts?: Record<string, string[]>;
  accountId?: string;
  chatNumber?: string;
  messages?: Message[];
};

// Model type for a generic chat WebSocket event.
export type ChatWebSocketEvent = {
  type: string;
  [key: string]: unknown;
};

// Request type for chat WebSocket user message payload.
export type ChatWebSocketPayload = {
  type: "user_message";
  accountId: string;
  conversationId: string;
  message: string;
  envProducts: Record<string, string[]>;
};

// Model type for chat WebSocket hook options.
export type UseChatWebSocketOptions = {
  onEvent: (event: ChatWebSocketEvent) => void;
  onClose?: () => void;
  onError?: (message: string) => void;
};
