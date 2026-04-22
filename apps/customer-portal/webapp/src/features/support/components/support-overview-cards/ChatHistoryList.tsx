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

import type { ChatHistoryListProps } from "@features/support/types/supportComponents";
import {
  Box,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import ChatHistorySkeleton from "@features/support/components/support-overview-cards/ChatHistorySkeleton";
import ChatHistoryCard from "@features/support/components/support-overview-cards/ChatHistoryCard";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import EmptyIcon from "@components/empty-state/EmptyIcon";

/**
 * Renders a list of chat history rows for the support overview card.
 *
 * @param {ChatHistoryListProps} props - Items and optional action handler.
 * @returns {JSX.Element} The list of chat rows.
 */
export default function ChatHistoryList({
  items,
  isLoading,
  isError,
  onItemAction,
}: ChatHistoryListProps): JSX.Element {
  const listShellSx = {
    display: "flex",
    flexDirection: "column" as const,
    gap: 1.5,
    width: "100%",
    flex: 1,
    minHeight: 0,
  };

  if (isError) {
    return <ErrorIndicator entityName="chat history" size="medium" />;
  }

  if (isLoading) {
    return (
      <Box sx={listShellSx}>
        <ChatHistorySkeleton />
      </Box>
    );
  }

  if (!items || items.length === 0) {
    return (
      <Box
        sx={{
          ...listShellSx,
          alignItems: "center",
          justifyContent: "center",
          py: 2,
        }}
      >
        <EmptyIcon
          style={{
            width: 120,
            maxWidth: "100%",
            height: "auto",
            marginBottom: 12,
          }}
        />
        <Typography variant="body2" color="text.secondary">
          No chat history.
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={listShellSx}>
      {items.map((item) => (
        <ChatHistoryCard
          key={item.chatId}
          item={item}
          onItemAction={onItemAction}
        />
      ))}
    </Box>
  );
}
