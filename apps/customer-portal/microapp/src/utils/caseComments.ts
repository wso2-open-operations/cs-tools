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
    createdBy: entry.createdByFullName?.trim() || entry.createdBy?.trim() || "",
    isEscalated: "false",
    hasInlineAttachments: false,
    inlineAttachments: [],
  };
}
