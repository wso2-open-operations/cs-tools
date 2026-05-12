export interface ConversationCreatedNoveraResponse {
  type: "conversation_created";
  conversationId: string;
}

export interface ThinkingStartNoveraResponse {
  type: "thinking_start";
}

export interface TokenNoveraResponse {
  type: "token";
  content: string;
}

export interface ThinkingStepNoveraResponse {
  type: "thinking_step";
  step: string;
  label: string;
}

export interface ThinkingEndNoveraResponse {
  type: "thinking_end";
}

export interface FinalNoveraResponse {
  type: "final";
  payload: {
    message: string;
    sessionId: string;
    conversationId: string;
  };
}

export type NoveraResponse =
  | ConversationCreatedNoveraResponse
  | ThinkingStartNoveraResponse
  | ThinkingEndNoveraResponse
  | ThinkingStepNoveraResponse
  | TokenNoveraResponse
  | FinalNoveraResponse;
