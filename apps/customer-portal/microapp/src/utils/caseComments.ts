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

import type { CommentDto } from "@src/types";
import { commentContentToHtml } from "./commentContent";
import { stripStaffNameMarker } from "./staffNameMarker";

/** The author shown for a comment that has none, as in the webapp (SYSTEM_AUTHOR_LABEL). */
export const SYSTEM_AUTHOR_LABEL = "System";

/** One entry of a case's activity feed (POST /cases/{id}/activities/search). */
export interface CaseActivityEntry {
  id: string;
  /** "comment", "attachment" or "field_change". */
  type: string;
  content?: string | null;
  createdOn: string;
  createdBy?: string | null;
  createdByFullName?: string | null;
  /** The backend resolved the author as an internal (WSO2 support) user. */
  createdByIsInternal?: boolean;
}

/** One page of the activity feed. */
export interface CaseActivitiesPage {
  activities?: CaseActivityEntry[] | null;
  hasMore?: boolean;
}

/** The comment entries across the pages: attachments and field changes in the feed are not comments. */
export function commentEntries(pages: CaseActivitiesPage[]): CaseActivityEntry[] {
  return pages.flatMap((page) => (page.activities ?? []).filter((entry) => entry.type === "comment"));
}

/** A feed entry in the shape the comment mapper takes, so comments are mapped in one place. */
export function toCommentDto(entry: CaseActivityEntry): CommentDto {
  return {
    id: entry.id,
    content: commentContentToHtml(entry.content),
    type: "comment",
    createdOn: entry.createdOn,
    // Each name is cleaned before it is chosen: a full name that is only the staff marker is no name at
    // all, so it must not shadow a real createdBy, and two empty names mean the system.
    createdBy:
      stripStaffNameMarker(entry.createdByFullName) || stripStaffNameMarker(entry.createdBy) || SYSTEM_AUTHOR_LABEL,
    isEscalated: "false",
    hasInlineAttachments: false,
    inlineAttachments: [],
  };
}

/** What backend-v2 answers to POST /cases/{id}/comments: only the new comment's id, time and author. */
export interface CreatedCommentResponse {
  id: string;
  createdOn: string;
  createdBy: string;
}

/**
 * The comment just posted, in the shape the comment mapper takes. The response carries no text or
 * attachments, so the text is what was sent; without this the mapper fails on the missing
 * attachments after the post has already succeeded, and the page never refreshes its timeline.
 */
export function createdCommentDto(response: CreatedCommentResponse, content: string): CommentDto {
  return {
    id: response.id,
    content,
    type: "comment",
    createdOn: response.createdOn,
    createdBy: response.createdBy,
    isEscalated: "false",
    hasInlineAttachments: false,
    inlineAttachments: [],
  };
}
