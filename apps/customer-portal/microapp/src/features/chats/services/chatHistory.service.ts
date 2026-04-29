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

import type { ChatMessage } from "@features/chats/components";
import type { Comment } from "@features/cases/types/case.model";

export function messagesToString(messages: ChatMessage[]): string {
  return messages
    .map((msg) => {
      const role = msg.author === "you" ? "User" : "Assistant";
      const textContent = msg.blocks
        .filter((block) => block.type === "text")
        .map((block) => block.value)
        .join(" ");
      return `${role}: ${textContent}`;
    })
    .join("\n");
}

export function buildEnvProducts(
  deployments: { id: string; name: string }[],
  productQueries: { data?: { name: string; version: string }[] | undefined }[],
): Record<string, string[]> {
  return deployments.reduce(
    (acc, deployment, index) => {
      const products = productQueries[index]?.data ?? [];
      const productNames = products.map((p) => p.name + " " + p.version);
      return { ...acc, [deployment.name]: productNames };
    },
    {} as Record<string, string[]>,
  );
}

export function mapCommentsToChatMessages(
  comments: Comment[],
  fromNow: (date: Date) => string,
): (ChatMessage & { id: string })[] {
  return comments.map((comment) => ({
    id: comment.id,
    author: comment.createdBy === "Novera" ? "assistant" : "you",
    blocks: [{ type: "text" as const, value: comment.content || "" }],
    timestamp: fromNow(new Date(comment.createdOn)),
  }));
}
