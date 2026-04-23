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

import type { ConversationSummaryProps } from "@features/support/types/supportComponents";
import {
  Box,
  Button,
  colors,
  Divider,
  Paper,
  Skeleton,
  Typography,
} from "@wso2/oxygen-ui";
import { Bot, ExternalLink, MessageSquare } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import { useNavigate, useParams } from "react-router";
import useGetConversationSummary from "@features/support/api/useGetConversationSummary";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";

/**
 * Sidebar component showing conversation summary and tips.
 *
 * @param {ConversationSummaryProps} props - conversationId to fetch summary.
 * @returns {JSX.Element} The conversation summary sidebar.
 */
export function ConversationSummary({
  conversationId,
}: ConversationSummaryProps): JSX.Element {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();
  
  const {
    data: summary,
    isLoading,
    isError,
  } = useGetConversationSummary(
    projectId || "",
    conversationId || "",
  );

  const handleViewConversation = () => {
    if (projectId && conversationId) {
      navigate(`/projects/${projectId}/support/conversations/${conversationId}`);
    }
  };

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
        <Box>
          <Typography variant="h6" sx={{ display: "block", mb: 0.5 }}>
            Messages exchanged
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width="20%" height={20} />
          ) : isError || summary?.messagesExchanged == null ? (
            <ErrorIndicator entityName="Messages exchanged" size="small" />
          ) : (
            <Typography variant="body2">
              {summary.messagesExchanged}
            </Typography>
          )}
        </Box>
        <Box>
          <Typography variant="h6" sx={{ display: "block", mb: 0.5 }}>
            Troubleshooting attempts
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width="60%" height={20} />
          ) : isError || summary?.troubleshootingAttempts == null ? (
            <ErrorIndicator
              entityName="Troubleshooting attempts"
              size="small"
            />
          ) : (
            <Typography variant="body2">
              {summary.troubleshootingAttempts}
            </Typography>
          )}
        </Box>
        <Box>
          <Typography variant="h6" sx={{ display: "block", mb: 0.5 }}>
            KB articles reviewed
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width="50%" height={20} />
          ) : isError || summary?.kbArticlesReviewed == null ? (
            <ErrorIndicator entityName="KB articles reviewed" size="small" />
          ) : (
            <Typography variant="body2">
              {summary.kbArticlesReviewed}
            </Typography>
          )}
        </Box>
      </Box>

      <Divider sx={{ mb: 3 }} />

      <Button
        variant="text"
        endIcon={<ExternalLink size={16} />}
        onClick={handleViewConversation}
        disabled={isLoading || !projectId || !conversationId}
        sx={{
          mb: 3,
          textTransform: "none",
          px: 0,
          justifyContent: "flex-start",
        }}
      >
        View full conversation
      </Button>

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
