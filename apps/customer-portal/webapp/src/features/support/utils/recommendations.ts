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

import type { CaseComment, CaseDetails } from "@features/support/types/cases";
import type { ConversationMessage } from "@features/support/types/conversations";
import type {
  ConversationDataForRecommendation,
  RecommendationApiMessage,
  RecommendationSearchRequest,
} from "@features/support/types/recommendations";

/**
 * Maps a similarity score (0–1) to a whole-number percentage label.
 *
 * @param {number} score - Raw score from the API.
 * @returns {number} Percentage between 0 and 100.
 */
export function recommendationScoreToPercent(score: number): number {
  if (Number.isNaN(score)) {
    return 0;
  }
  if (score > 1) {
    return Math.max(0, Math.min(100, Math.round(score)));
  }
  return Math.round(Math.max(0, Math.min(1, score)) * 100);
}

/**
 * Builds envProducts map from case deployment and product labels.
 *
 * @param {CaseDetails | undefined} data - Case details.
 * @returns {Record<string, string[]>} OpenAPI envProducts shape.
 */
export function buildEnvProductsFromCase(
  data: CaseDetails | undefined,
): Record<string, string[]> {
  if (!data?.deployment?.label) {
    return {};
  }
  const products: string[] = [];
  if (data.deployedProduct?.label) {
    const label = data.deployedProduct.version
      ? `${data.deployedProduct.label} ${data.deployedProduct.version}`
      : data.deployedProduct.label;
    products.push(label);
  } else if (data.product?.label) {
    products.push(data.product.label);
  }
  return products.length > 0 ? { [data.deployment.label]: products } : {};
}

/**
 * Derives recommendation API message role from a case comment.
 *
 * @param {CaseComment} comment - Case comment row.
 * @returns {"assistant" | "user"} Role for the recommendations API.
 */
export function roleFromCaseComment(
  comment: CaseComment,
): "assistant" | "user" {
  const type = comment.type?.toLowerCase() ?? "";
  const by = comment.createdBy?.toLowerCase() ?? "";
  if (
    type.includes("bot") ||
    type.includes("system") ||
    by.includes("novera")
  ) {
    return "assistant";
  }
  return "user";
}

/**
 * Builds recommendation request payload from case details and comments.
 *
 * @param {CaseDetails | undefined} data - Case details.
 * @param {CaseComment[]} comments - Case comments (newest-first or arbitrary order).
 * @returns {RecommendationSearchRequest | null} Payload or null if nothing to send.
 */
export function buildRecommendationRequestFromCase(
  data: CaseDetails | undefined,
  comments: CaseComment[],
): RecommendationSearchRequest | null {
  if (!data) {
    return null;
  }

  const chatHistory: RecommendationApiMessage[] = [];
  const title = data.title?.trim() ?? "";
  const description = data.description?.trim() ?? "";
  const createdOn = data.createdOn ?? "";

  if (title.length > 0) {
    chatHistory.push({
      role: "user",
      content: title,
      timestamp: createdOn,
    });
  }
  if (description.length > 0) {
    chatHistory.push({
      role: "user",
      content: description,
      timestamp: createdOn,
    });
  }

  const sortedComments = [...comments].sort((a, b) => {
    const ta = a.createdOn ?? "";
    const tb = b.createdOn ?? "";
    return ta.localeCompare(tb);
  });

  for (const c of sortedComments) {
    const content = c.content?.trim() ?? "";
    if (!content) continue;
    chatHistory.push({
      role: roleFromCaseComment(c),
      content,
      timestamp: c.createdOn ?? createdOn,
    });
  }

  if (chatHistory.length === 0) {
    return null;
  }

  const textBlob = chatHistory.map((m) => m.content).join("\n\n");
  const conversationData: ConversationDataForRecommendation = {
    chatHistory: textBlob,
    envProducts: buildEnvProductsFromCase(data),
    region: "",
    tier: "",
  };

  return { chatHistory, conversationData };
}

/**
 * Builds recommendation request payload from Novera conversation messages.
 *
 * @param {ConversationMessage[]} messages - Messages from GET conversation messages.
 * @returns {RecommendationSearchRequest | null} Payload or null if empty.
 */
export function buildRecommendationRequestFromConversationMessages(
  messages: ConversationMessage[],
): RecommendationSearchRequest | null {
  const sorted = [...messages].sort((a, b) => {
    const ta = a.createdOn ?? "";
    const tb = b.createdOn ?? "";
    return ta.localeCompare(tb);
  });

  const chatHistory: RecommendationApiMessage[] = [];
  for (const m of sorted) {
    const content = m.content?.trim() ?? "";
    if (!content) continue;
    const isBot =
      m.type?.toLowerCase() === "bot" ||
      m.createdBy?.toLowerCase() === "novera";
    chatHistory.push({
      role: isBot ? "assistant" : "user",
      content,
      timestamp: m.createdOn ?? "",
    });
  }

  if (chatHistory.length === 0) {
    return null;
  }

  const textBlob = chatHistory.map((m) => m.content).join("\n\n");
  const conversationData: ConversationDataForRecommendation = {
    chatHistory: textBlob,
    envProducts: {},
    region: "",
    tier: "",
  };

  return { chatHistory, conversationData };
}
