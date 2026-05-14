export type MessageBlock =
  | { type: "text"; value: string }
  | { type: "checklist"; items: string[] }
  | { type: "kb"; items: { id: string; title: string }[] };

export type MessageAuthor = "you" | "assistant";

export interface ChatMessage {
  author: MessageAuthor;
  blocks: MessageBlock[];
  timestamp?: string;
  animated?: boolean;
  /** Pass a string to show as the thinking label, or false to hide. */
  thinking?: string | false;
  onAnimationComplete?: () => void;
}