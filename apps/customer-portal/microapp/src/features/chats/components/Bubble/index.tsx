import { BubbleUser, type BubbleUserProps } from "./BubbleUser";
import { BubbleAgent, type BubbleAgentProps } from "./BubbleAgent";
import { MESSAGE_AUTHOR_TYPES } from "@root/src/shared/constants";
import { BubbleUserSkeleton } from "./BubbleUserSkeleton";
import { BubbleAgentSkeleton } from "./BubbleAgentSkeleton";

export type BubbleProps = BubbleUserProps | BubbleAgentProps;

export function Bubble(props: BubbleProps) {
  if (props.author === MESSAGE_AUTHOR_TYPES.USER) return <BubbleUser {...props} />
  if (props.author === MESSAGE_AUTHOR_TYPES.AGENT) return <BubbleAgent {...props} />
}

export function BubbleSkeleton({author}: {author: BubbleProps["author"]}) {
  if (author === MESSAGE_AUTHOR_TYPES.USER) return <BubbleUserSkeleton />
  if (author === MESSAGE_AUTHOR_TYPES.AGENT) return <BubbleAgentSkeleton />
}

export type { BubbleUserProps } from "./BubbleUser"; 
export type { BubbleAgentProps } from "./BubbleAgent"; 