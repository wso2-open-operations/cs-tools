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
import { Card, pxToRem, Skeleton, Stack, type SxProps, type Theme } from "@wso2/oxygen-ui";
import type { ChatMessage, MessageAuthor } from "@features/chats/types";
import { AssistantBubble, YouBubble } from "@features/chats/components";

export function MessageBubble({ author, ...props }: ChatMessage & { sx?: SxProps<Theme> }) {
  return author === "you" ? <YouBubble {...props} /> : <AssistantBubble {...props} />;
}

export function MessageBubbleSkeleton({ author = "assistant", sx }: {
  author?: MessageAuthor;
  sx?: SxProps<Theme>;
}) {
  const isYou = author === "you";

  return (
    <Stack direction="row" justifyContent={isYou ? "end" : "start"} width="100%">
      <Card
        component={Stack}
        p={1.5}
        ml={isYou ? 10 : undefined}
        width={isYou ? "fit-content" : "100%"}
        sx={{
          ...sx,
          bgcolor: "background.paper",
          borderStyle: "dashed",
          borderWidth: 1,
          borderColor: "divider",
        }}
      >
        {!isYou && (
          <Stack direction="row" justifyContent="start" gap={1} mb={1.5}>
            <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
            <Skeleton variant="text" width={60} height={20} />
          </Stack>
        )}

        <Stack gap={1}>
          <Skeleton variant="text" width={isYou ? 150 : "90%"} height={20} />
          <Skeleton variant="text" width={isYou ? 100 : "75%"} height={20} />
        </Stack>

        {isYou && (
          <Stack direction="row" justifyContent="end" mt={1}>
            <Skeleton variant="text" width={50} height={20} />
          </Stack>
        )}
      </Card>
    </Stack>
  );
}
