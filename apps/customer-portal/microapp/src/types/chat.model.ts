export interface Chat {
  id: string;
  number: string;
  description: string;
  count: number;
  createdOn: Date;
  createdBy: string;
  statusId?: string;
}

export interface Message {
  content: string;
  direction: "outgoing" | "incoming";
  conversationId: string;
  timestamp: Date;
}

// interface KB {
//   id: string;
//   title: string;
//   score: number;
// }
