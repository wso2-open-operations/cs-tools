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
import { Stack, Typography } from "@wso2/oxygen-ui";

import { Bubble, BubbleSkeleton } from "@features/case-types/conversations/components";
import { useConversation } from "@features/detail/hooks";

import { MESSAGE_AUTHOR_TYPES } from "@shared/constants";

export function BubbleList() {
  const { data, isLoading } = useConversation();

  if (isLoading) return <BubbleListSkeleton />;

  if (!data?.length) {
    return (
      <Typography variant="body2" color="text.secondary">
        No messages yet.
      </Typography>
    );
  }

  return (
    <Stack gap={2} mt={1}>
      {data
        .slice()
        .reverse()
        .map(({ id, content, createdBy, createdOn }) => {
          const author = createdBy === "Novera" ? MESSAGE_AUTHOR_TYPES.AGENT : MESSAGE_AUTHOR_TYPES.USER;

          return (
            <Bubble
              key={id}
              content={content}
              author={author}
              timestamp={createdOn}
              thinking={false}
              animated={false}
            />
          );
        })}
    </Stack>
  );
}

function BubbleListSkeleton() {
  return (
    <Stack gap={2} p={2} width="100%">
      {Array.from({ length: 6 }).map((_, index) => {
        const author = index % 2 === 0 ? "agent" : "user";
        return <BubbleSkeleton key={index} author={author} />;
      })}
    </Stack>
  );
}
