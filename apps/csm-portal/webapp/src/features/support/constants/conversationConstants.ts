// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

/** Default region for conversations and case classification APIs. TODO: Replace with projectDetails.account.region when multi-region support is added. */
export const DEFAULT_CONVERSATION_REGION = "EU";

/** Default tier for conversations and case classification APIs. TODO: Replace with dynamic values when multi-tier support is added. */
export const DEFAULT_CONVERSATION_TIER = "Tier 1";

// --- All conversations list --------------------------------------------------

/** `ErrorIndicator` entity name for failed load state. */
export const ALL_CONVERSATIONS_LIST_ERROR_ENTITY_NAME = "conversations";

export const ALL_CONVERSATIONS_LIST_ERROR_MESSAGE =
  "Failed to load conversations. Please try again.";

export const ALL_CONVERSATIONS_LIST_EMPTY_REFINED_MESSAGE =
  "No conversations found. Try adjusting your filters or search query.";

export const ALL_CONVERSATIONS_LIST_EMPTY_DEFAULT_MESSAGE =
  "No conversations yet.";

/** Empty-state / search illustration width (px). */
export const ALL_CONVERSATIONS_LIST_ILLUSTRATION_WIDTH_PX = 200;

/** Empty-state illustration bottom margin (px). */
export const ALL_CONVERSATIONS_LIST_ILLUSTRATION_MARGIN_BOTTOM_PX = 16;

/** Empty-state vertical padding (MUI spacing). */
export const ALL_CONVERSATIONS_LIST_EMPTY_CONTAINER_PY = 6;

export const ALL_CONVERSATIONS_LIST_ACTION_VIEW_LABEL = "View";

export const ALL_CONVERSATIONS_LIST_ACTION_RESUME_LABEL = "Resume";

export const ALL_CONVERSATIONS_LIST_MESSAGE_SINGULAR = "message";

export const ALL_CONVERSATIONS_LIST_MESSAGE_PLURAL = "messages";

export const ALL_CONVERSATIONS_LIST_CREATED_BY_PREFIX = "Created by ";
