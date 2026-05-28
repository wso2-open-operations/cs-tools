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
import { MESSAGE_AUTHOR_TYPES } from "@shared/constants";

import { BubbleAgent, type BubbleAgentProps } from "./BubbleAgent";
import { BubbleAgentSkeleton } from "./BubbleAgentSkeleton";
import { BubbleUser, type BubbleUserProps } from "./BubbleUser";
import { BubbleUserSkeleton } from "./BubbleUserSkeleton";

export type BubbleProps = BubbleUserProps | BubbleAgentProps;

export function Bubble(props: BubbleProps) {
  if (props.author === MESSAGE_AUTHOR_TYPES.USER) return <BubbleUser {...props} />;
  if (props.author === MESSAGE_AUTHOR_TYPES.AGENT) return <BubbleAgent {...props} />;
}

export function BubbleSkeleton({ author }: { author: BubbleProps["author"] }) {
  if (author === MESSAGE_AUTHOR_TYPES.USER) return <BubbleUserSkeleton />;
  if (author === MESSAGE_AUTHOR_TYPES.AGENT) return <BubbleAgentSkeleton />;
}

export type { BubbleUserProps } from "./BubbleUser";
export type { BubbleAgentProps } from "./BubbleAgent";
