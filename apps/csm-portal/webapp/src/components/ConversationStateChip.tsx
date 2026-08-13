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

import type { JSX } from "react";
import SemanticChip from "@components/SemanticChip";
import { conversationStateChipMeta } from "@features/csm-projects/utils/conversationState";
import type { BeConversationState } from "@api/backend/types";

interface ConversationStateChipProps {
  state: BeConversationState;
  size?: "small" | "medium";
  variant?: "filled" | "outlined";
  clickable?: boolean;
}

/**
 * The single source of truth for rendering a conversation's state as a chip
 * — shared by the Conversations table, `ConversationPreviewDrawer`, and the
 * conversation detail page, so the grouping (see `conversationStateGroup`)
 * reads identically wherever a conversation's state is shown. Callers handle
 * the `null` state themselves (rendering "—") since this component only
 * accepts a real state.
 */
export default function ConversationStateChip({
  state,
  size = "small",
  variant,
  clickable = false,
}: ConversationStateChipProps): JSX.Element {
  const meta = conversationStateChipMeta(state);
  return (
    <SemanticChip
      role={meta.role}
      label={meta.label}
      size={size}
      variant={variant}
      clickable={clickable}
    />
  );
}
