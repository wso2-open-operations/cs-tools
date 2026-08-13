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

import { Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import CsmCaseCommentBubble from "@features/csm-cases/components/CsmCaseCommentBubble";
import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";

interface ConversationMessagesListProps {
  messages: CsmCaseComment[] | undefined;
  isLoading: boolean;
  isError: boolean;
  /** Denser bubble rendering, for the quick-preview drawer. */
  compact?: boolean;
  /** Copy shown for the empty state — differs between the full transcript
   * page and the "recent messages" preview. */
  emptyMessage?: string;
}

/**
 * The message-list rendering for a chat session's transcript — loading
 * skeletons, an error state, an empty state, or the messages themselves as
 * `CsmCaseCommentBubble`s. Extracted from the old `ConversationTranscriptDialog`
 * so both `ConversationPreviewDrawer` (a handful of recent messages) and
 * `ConversationDetailPage` (the full transcript) render messages identically
 * — same bubble, same sanitisation, same author-role handling — without
 * duplicating this block.
 */
export default function ConversationMessagesList({
  messages,
  isLoading,
  isError,
  compact = false,
  emptyMessage = "No messages in this conversation.",
}: ConversationMessagesListProps): JSX.Element {
  if (isLoading) {
    return (
      <Stack gap={1.5}>
        <Skeleton variant="rounded" height={64} />
        <Skeleton variant="rounded" height={64} />
        <Skeleton variant="rounded" height={64} />
      </Stack>
    );
  }

  if (isError) {
    return (
      <Typography variant="body2" color="error">
        Could not load this conversation.
      </Typography>
    );
  }

  if (!messages || messages.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        {emptyMessage}
      </Typography>
    );
  }

  return (
    <Stack gap={1.5}>
      {messages.map((message) => (
        <CsmCaseCommentBubble key={message.id} comment={message} compact={compact} />
      ))}
    </Stack>
  );
}
