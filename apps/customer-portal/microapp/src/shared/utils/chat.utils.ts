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
import type { BubbleProps } from "@features/case-types/conversations/components";

import { MESSAGE_AUTHOR_TYPES } from "@shared/constants";

export const toTranscript = (messages: BubbleProps[]): string => {
  return messages
    .map((msg) => {
      const role = msg.author === MESSAGE_AUTHOR_TYPES.USER ? "User" : "Assistant";
      return `${role}: ${msg.content}`;
    })
    .join("\n");
};
