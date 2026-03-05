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
