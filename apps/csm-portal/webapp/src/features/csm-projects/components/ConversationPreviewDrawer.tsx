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

import { Drawer } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import ConversationPreviewContent from "@features/csm-projects/components/ConversationPreviewContent";
import type { BeConversationView } from "@api/backend/types";

interface ConversationPreviewDrawerProps {
  /** The conversation being previewed. `null` keeps the drawer
   * mounted-but-closed, so its close transition can play instead of
   * unmounting mid-animation — same convention as `CasePreviewDrawer`. */
  conversation: BeConversationView | null;
  onClose: () => void;
}

/**
 * Read-only "quick look" for a conversation row — summary info plus the last
 * few messages — without leaving the Conversations tab. Right-anchored,
 * mirroring `CasePreviewDrawer`'s width and anchor.
 */
export default function ConversationPreviewDrawer({
  conversation,
  onClose,
}: ConversationPreviewDrawerProps): JSX.Element {
  return (
    <Drawer
      anchor="right"
      open={!!conversation}
      onClose={onClose}
      slotProps={{ paper: { sx: { width: { xs: "100%", sm: 420 } } } }}
    >
      {conversation && (
        <ConversationPreviewContent conversation={conversation} onClose={onClose} />
      )}
    </Drawer>
  );
}
