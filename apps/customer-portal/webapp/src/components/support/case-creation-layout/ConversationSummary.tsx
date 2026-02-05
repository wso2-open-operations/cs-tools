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

import {
  Box,
  colors,
  Divider,
  Paper,
  Skeleton,
  Typography,
} from "@wso2/oxygen-ui";
import { Bot, MessageSquare } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { CaseCreationMetadata } from "@/models/mockData";

interface ConversationSummaryProps {
  metadata?: CaseCreationMetadata;
  isLoading: boolean;
}

/**
 * Sidebar component showing conversation summary and tips.
 */
export function ConversationSummary({
  metadata,
  isLoading,
}: ConversationSummaryProps): JSX.Element {
  return (
    <Paper sx={{ p: 3, position: "sticky", top: 3 }}>
      {/* sidebar header container */}
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 3 }}>
        <MessageSquare size={18} color={colors.orange[700]} />
        <Typography variant="h6">Conversation Summary</Typography>
      </Box>

      <Divider sx={{ mb: 3 }} />

      {/* stats container */}
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3, mb: 3 }}>
        {/* messages stat wrapper */}
        <Box>
          <Typography variant="h6" sx={{ display: "block", mb: 0.5 }}>
            Messages exchanged
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width="20%" height={20} />
          ) : (
            <Typography variant="body2">
              {metadata?.conversationSummary?.messagesExchanged ?? "N/A"}
            </Typography>
          )}
        </Box>
        {/* troubleshooting stat wrapper */}
        <Box>
          <Typography variant="h6" sx={{ display: "block", mb: 0.5 }}>
            Troubleshooting attempts
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width="60%" height={20} />
          ) : (
            <Typography variant="body2">
              {metadata?.conversationSummary?.troubleshootingAttempts ?? "N/A"}
            </Typography>
          )}
        </Box>
        {/* KB articles stat wrapper */}
        <Box>
          <Typography variant="h6" sx={{ display: "block", mb: 0.5 }}>
            KB articles reviewed
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width="50%" height={20} />
          ) : (
            <Typography variant="body2">
              {metadata?.conversationSummary?.kbArticlesReviewed ?? "N/A"}
            </Typography>
          )}
        </Box>
      </Box>

      <Divider sx={{ mb: 3 }} />

      <Typography
        variant="body2"
        sx={{
          color: "primary.main",
          cursor: isLoading ? "default" : "pointer",
          fontWeight: 600,
          "&:hover": { textDecoration: isLoading ? "none" : "underline" },
          mb: 3,
          display: "block",
          opacity: isLoading ? 0.5 : 1,
          pointerEvents: isLoading ? "none" : "auto",
        }}
      >
        View full conversation
      </Typography>

      {/* conversation attachment tip container */}
      <Paper sx={{ p: 2 }}>
        <Typography
          variant="caption"
          sx={{ color: "info.main", display: "flex", gap: 1 }}
        >
          <Bot size={14} style={{ flexShrink: 0, marginTop: 2 }} />
          All conversation details will be attached to your case for the support
          team&apos;s reference.
        </Typography>
      </Paper>
    </Paper>
  );
}
