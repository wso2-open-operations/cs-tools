// /// chats listing [main]
// export interface ChatsDTO extends Pagination {
//   conversations: ChatSummaryDTO[];
// }

// export interface ChatSummaryDTO {
//   id: string;
//   number: string;
//   initialMessage: string;
//   messageCount: number;
//   createdOn: string;
//   createdBy: string;
//   project: EntityReference | null;
//   case: EntityReference | null;
//   state: EntityReference | null;
// }

// // chat detail [main]
// export interface ChatDTO {
//   id: string;
//   number: string;
//   initialMessage: string;
//   messageCount: number;
//   updatedOn: string;
//   updatedBy: string;
//   createdOn: string;
//   createdBy: string;
//   project: EntityReference | null;
//   case: EntityReference | null;
//   state: EntityReference | null;
// }

// /// comments listing
// export interface CommentsDTO extends Pagination {
//   comments: CommentDTO[];
// }

// export interface CommentDTO {
//   id: string;
//   content: string;
//   type: string;
//   createdOn: string;
//   createdBy: string;
//   isEscalated: boolean;
//   hasInlineAttachments: boolean;
//   inlineAttachments: {
//     id: string;
//     fileName: string;
//     contentType: string;
//     downloadUrl: string;
//     createdOn: string;
//     createdBy: string;
//   }[];
// }

// message sending
export interface MessageDispatchDTO {
  message: string;
  envProducts: Record<string, string[]>;
  region: string;
  tier: string;
}

// message receiving
export interface MessageResponseDTO {
  message: string;
  sessionId: string;
  conversationId: string;
  resolved: boolean | null;
  //   recommendations: { query: string; recommendations: { title: string; articleId: string; score: number }[] } | null;
}
