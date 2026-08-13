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

import type { SemanticRole } from "@components/SemanticChip";
import type { BeConversationState } from "@api/backend/types";

/** All 5 raw backend conversation states, for the filter multi-select. */
export const ALL_CONVERSATION_STATES: BeConversationState[] = [
  "ACTIVE",
  "RESOLVED",
  "CONVERTED",
  "ABANDONED",
  "CLOSED",
];

/** Human-readable label for each raw backend state — used by the filter
 * multi-select, where every state must be individually pickable. */
export const CONVERSATION_STATE_LABEL: Record<BeConversationState, string> = {
  ACTIVE: "Active",
  RESOLVED: "Resolved",
  CONVERTED: "Converted",
  ABANDONED: "Abandoned",
  CLOSED: "Closed",
};

/**
 * The 3 chip groups a raw state collapses into for display (table rows,
 * preview drawer, detail page): `ACTIVE` stays on its own; `CONVERTED` — a
 * chat that became a real case — is its own positive/success group, distinct
 * from the "closed" bucket even though `CLOSED` is one of the raw values; the
 * remaining terminal-but-not-converted states (`RESOLVED`, `ABANDONED`,
 * `CLOSED`) collapse into one neutral "Closed" group, mirroring how
 * `announcementStateRole` (see `csm-announcements/utils/announcementState.ts`)
 * paints a lifecycle's closed/inactive state `default` (grey), not `success`
 * — unlike the case-state palette, where `closed` is green because a closed
 * case is a completed positive outcome. Here `CONVERTED` is the positive
 * outcome instead, so "Closed" stays neutral.
 */
export type ConversationStateGroup = "active" | "converted" | "closed";

export function conversationStateGroup(
  state: BeConversationState,
): ConversationStateGroup {
  switch (state) {
    case "ACTIVE":
      return "active";
    case "CONVERTED":
      return "converted";
    case "RESOLVED":
    case "ABANDONED":
    case "CLOSED":
    default:
      return "closed";
  }
}

const GROUP_META: Record<ConversationStateGroup, { label: string; role: SemanticRole }> = {
  active: { label: "Active", role: "info" },
  converted: { label: "Converted", role: "success" },
  closed: { label: "Closed", role: "default" },
};

/** Chip label + {@link SemanticRole} for a raw state's display group. */
export function conversationStateChipMeta(
  state: BeConversationState,
): { label: string; role: SemanticRole } {
  return GROUP_META[conversationStateGroup(state)];
}

/**
 * Filter state for `ConversationsFilterBar` — the subset of
 * `BeSearchConversationsFilters` exposed in the Conversations tab UI
 * (`projectIds` is fixed to the surrounding project, not a user-facing
 * control). No date-range field: the conversations endpoint doesn't filter
 * on it. `number` is an explicit exact-match field, distinct from `search`
 * (which the top search box may also route to `filters.number` when it looks
 * like a chat number — see `classifyConversationQuery`); `createdBy` is an
 * explicit initiator-email multi-select, distinct from the `createdByMe`
 * "my conversations" checkbox.
 */
export interface ConversationsFilters {
  search: string;
  states: BeConversationState[];
  createdByMe: boolean;
  number: string;
  createdBy: string[];
}

export const DEFAULT_CONVERSATION_FILTERS: ConversationsFilters = {
  search: "",
  states: [],
  createdByMe: false,
  number: "",
  createdBy: [],
};

/** Count non-search active filters (used for the badge on the Filters button,
 * matching `countActiveIncidentFilters`'s convention). */
export function countActiveConversationFilters(filters: ConversationsFilters): number {
  return (
    (filters.states.length > 0 ? 1 : 0) +
    (filters.createdByMe ? 1 : 0) +
    (filters.number.trim() ? 1 : 0) +
    (filters.createdBy.length > 0 ? 1 : 0)
  );
}
