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
import { useEffect } from "react";

import { scrollToBottom } from "@root/src/shared/utils";
import { Stack, Typography } from "@wso2/oxygen-ui";
import DOMPurify from "dompurify";

import { CommentItem, CommentItemSkeleton } from "@features/detail/components";
import { useComments } from "@features/detail/hooks";

import { RichText } from "@shared/components/common";

import { useDateTime } from "@shared/hooks";

export function CommentsList() {
  const { format } = useDateTime();
  const { comments, isLoading } = useComments();

  useEffect(scrollToBottom, [comments]);

  if (isLoading) return <CommentsListSkeleton />;

  if (!comments?.length) {
    return (
      <Typography variant="body2" color="text.secondary">
        No comments yet.
      </Typography>
    );
  }

  return (
    <Stack gap={1.5}>
      {comments.map(({ id, content, createdOn, createdBy }) => (
        <CommentItem key={id} author={createdBy} timestamp={format(createdOn)}>
          <RichText dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(content) }} />
        </CommentItem>
      ))}
    </Stack>
  );
}

function CommentsListSkeleton() {
  return (
    <Stack gap={1.5}>
      {Array.from({ length: 5 }).map((_, index) => (
        <CommentItemSkeleton key={index} />
      ))}
    </Stack>
  );
}
