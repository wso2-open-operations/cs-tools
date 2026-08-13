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

/**
 * A conversation number — always a "CHAT" prefix followed by a run of
 * digits. Deliberately NOT pinned to one fixed digit count: the two real
 * examples found in this codebase (the entity-service/BFF openapi additions
 * for `filters.number`, "CHAT0000012345", and the customer portal's own
 * live-production comment referencing "CHAT000002755") disagree on the
 * padding (10 digits vs. 9), so matching a fixed `\d{N}` would risk
 * rejecting a real number. "CHAT" plus one or more digits is the one thing
 * both sources agree on.
 */
const CONVERSATION_NUMBER_RE = /^CHAT\d+$/;

/**
 * What kind of lookup a typed conversation-search string should run as —
 * mirrors the shape of {@link
 * "@features/csm-cases/utils/caseQueryScope".CaseQueryScope}, just without
 * an internalId variant (conversations have only one exact-match
 * identifier).
 */
export type ConversationQueryScope = "number" | "text";

/**
 * Classifies a typed (already-trimmed) conversation-search string into the
 * scope it should be routed through: a `CHAT`-shaped string goes through as
 * an exact-match `filters.number` lookup; anything else falls back to the
 * free-text `filters.searchQuery` scan.
 *
 * Shared deliberately between the project Work-items "Chats" tab's own
 * search box (`useSearchConversations`) and the global quick-nav palette
 * (`useQuickConversationSearch`), so the same typed string always resolves
 * to the same conversation in both places.
 */
export function classifyConversationQuery(query: string): ConversationQueryScope {
  return CONVERSATION_NUMBER_RE.test(query) ? "number" : "text";
}
