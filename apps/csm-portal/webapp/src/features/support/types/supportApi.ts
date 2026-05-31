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

import type { CommentType } from "@features/support/constants/supportConstants";
import type {
  PostCaseAttachmentRequest,
  PatchAttachmentRequest,
} from "@features/support/types/attachments";
export type PostCommentRequest = {
  content: string;
  type: CommentType;
};

export type PostCommentVariables = {
  caseId: string;
  body: PostCommentRequest;
};

export type PostAttachmentsVariables = {
  caseId: string;
  body: PostCaseAttachmentRequest;
};

export type PatchCaseAttachmentVariables = {
  caseId: string;
  attachmentId: string;
  body: PatchAttachmentRequest;
};

/** Minimal response from PATCH /cases/:caseId. */
export type PatchCaseResponse = {
  id: string;
  updatedOn: string;
  updatedBy?: string;
  state?: { id: number; label: string };
  type?: { id: string; name: string };
};

export type UseGetProjectSupportStatsOptions = {
  caseTypes?: string[];
  query?: string;
};

export type ConversationSummaryResponse = {
  accountId: string;
  conversationId: string;
  messagesExchanged: number;
  troubleshootingAttempts: number;
  kbArticlesReviewed: number;
};

export type UseGetConversationStatsOptions = {
  createdByMe?: boolean;
  enabled?: boolean;
};

export type UseGetConversationMessagesOptions = {
  pageSize?: number;
};

export type UseGetCaseCommentsOptions = {
  offset?: number;
  limit?: number;
};

export type DeleteAttachmentVariables = {
  attachmentId: string;
  caseId?: string;
  deploymentId?: string;
};
