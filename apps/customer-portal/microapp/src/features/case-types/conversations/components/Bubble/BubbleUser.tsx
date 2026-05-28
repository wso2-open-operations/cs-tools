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
import { Card, Stack, Typography } from "@wso2/oxygen-ui";

import type { ChatMessage } from "@features/case-types/conversations/types";

import type { MESSAGE_AUTHOR_TYPES } from "@shared/constants";
import { useDateTime } from "@shared/hooks";

export interface BubbleUserProps extends ChatMessage {
  author: typeof MESSAGE_AUTHOR_TYPES.USER;
}

export function BubbleUser({ content, timestamp }: BubbleUserProps) {
  const { fromNow } = useDateTime();

  return (
    <Stack direction="row" justifyContent="end" width="100%">
      <Card component={Stack} p={1.5} ml={10} width="fit-content" sx={{ bgcolor: "background.paper" }}>
        <Typography variant="body2" component="span" sx={{ "& > *": { margin: 0, lineHeight: 1.7 } }}>
          {content}
        </Typography>
        <Stack direction="row" justifyContent="end">
          {timestamp && (
            <Typography variant="subtitle2" color="text.disabled" mt={1}>
              {fromNow(timestamp)}
            </Typography>
          )}
        </Stack>
      </Card>
    </Stack>
  );
}
