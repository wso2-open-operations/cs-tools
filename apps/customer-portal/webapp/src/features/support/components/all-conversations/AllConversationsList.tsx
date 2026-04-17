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
  Button,
  Chip,
  Form,
  Stack,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import {
  Calendar,
  ExternalLink,
  MessageSquare,
  Play,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import {
  formatDateTime,
  getStatusColor,
  getStatusIcon,
  resolveColorFromTheme,
} from "@features/support/utils/support";
import { resolveConversationListRowAction } from "@features/support/utils/conversationsList";
import {
  ALL_CONVERSATIONS_LIST_ACTION_RESUME_LABEL,
  ALL_CONVERSATIONS_LIST_ACTION_VIEW_LABEL,
  ALL_CONVERSATIONS_LIST_CREATED_BY_PREFIX,
  ALL_CONVERSATIONS_LIST_EMPTY_CONTAINER_PY,
  ALL_CONVERSATIONS_LIST_EMPTY_DEFAULT_MESSAGE,
  ALL_CONVERSATIONS_LIST_EMPTY_REFINED_MESSAGE,
  ALL_CONVERSATIONS_LIST_ERROR_ENTITY_NAME,
  ALL_CONVERSATIONS_LIST_ERROR_MESSAGE,
  ALL_CONVERSATIONS_LIST_ILLUSTRATION_MARGIN_BOTTOM_PX,
  ALL_CONVERSATIONS_LIST_ILLUSTRATION_WIDTH_PX,
  ALL_CONVERSATIONS_LIST_MESSAGE_PLURAL,
  ALL_CONVERSATIONS_LIST_MESSAGE_SINGULAR,
} from "@features/support/constants/conversationConstants";
import {
  ConversationListRowAction,
  type AllConversationsListProps,
} from "@features/support/types/conversations";
import AllConversationsListSkeleton from "./AllConversationsListSkeleton";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import EmptyIcon from "@components/empty-state/EmptyIcon";
import SearchNoResultsIcon from "@components/empty-state/SearchNoResultsIcon";

/**
 * Component to display conversations as cards.
 *
 * @param {AllConversationsListProps} props - Conversations array and handlers.
 * @returns {JSX.Element} The rendered conversation cards list.
 */
export default function AllConversationsList({
  conversations,
  isLoading,
  isError = false,
  hasListRefinement = false,
  onConversationClick,
}: AllConversationsListProps): JSX.Element {
  const theme = useTheme();

  if (isLoading) {
    return <AllConversationsListSkeleton />;
  }

  if (isError) {
    return (
      <Box sx={{ textAlign: "center", py: ALL_CONVERSATIONS_LIST_EMPTY_CONTAINER_PY }}>
        <ErrorIndicator
          entityName={ALL_CONVERSATIONS_LIST_ERROR_ENTITY_NAME}
          size="medium"
        />
        <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
          {ALL_CONVERSATIONS_LIST_ERROR_MESSAGE}
        </Typography>
      </Box>
    );
  }

  if (conversations.length === 0) {
    if (hasListRefinement) {
      return (
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            py: ALL_CONVERSATIONS_LIST_EMPTY_CONTAINER_PY,
          }}
        >
          <SearchNoResultsIcon
            style={{
              width: ALL_CONVERSATIONS_LIST_ILLUSTRATION_WIDTH_PX,
              maxWidth: "100%",
              height: "auto",
              marginBottom: ALL_CONVERSATIONS_LIST_ILLUSTRATION_MARGIN_BOTTOM_PX,
            }}
          />
          <Typography variant="body1" color="text.secondary">
            {ALL_CONVERSATIONS_LIST_EMPTY_REFINED_MESSAGE}
          </Typography>
        </Box>
      );
    }
    return (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          py: ALL_CONVERSATIONS_LIST_EMPTY_CONTAINER_PY,
        }}
      >
        <EmptyIcon
          style={{
            width: ALL_CONVERSATIONS_LIST_ILLUSTRATION_WIDTH_PX,
            maxWidth: "100%",
            height: "auto",
            marginBottom: ALL_CONVERSATIONS_LIST_ILLUSTRATION_MARGIN_BOTTOM_PX,
          }}
        />
        <Typography variant="body1" color="text.secondary">
          {ALL_CONVERSATIONS_LIST_EMPTY_DEFAULT_MESSAGE}
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {conversations.map((conv) => {
        const StatusIcon = getStatusIcon(conv.state?.label);
        const colorPath = getStatusColor(conv.state?.label);
        const resolvedColor = resolveColorFromTheme(colorPath, theme);
        const action = resolveConversationListRowAction(conv.state?.label);

        return (
          <Form.CardButton
            key={conv.id}
            onClick={() => onConversationClick?.(conv)}
            sx={{
              p: 3,
              display: "flex",
              flexDirection: "column",
              alignItems: "stretch",
              gap: 1,
            }}
          >
            <Form.CardHeader
              sx={{ p: 0 }}
              title={
                <Stack
                  direction="row"
                  spacing={1.5}
                  alignItems="center"
                  sx={{ mb: 1, flexWrap: "wrap" }}
                >
                  <Typography
                    variant="body2"
                    fontWeight={500}
                    color="text.primary"
                  >
                    {conv.number || "--"}
                  </Typography>
                  <Chip
                    size="small"
                    variant="outlined"
                    label={conv.state?.label || "--"}
                    icon={<StatusIcon size={12} />}
                    sx={{
                      bgcolor: alpha(resolvedColor, 0.1),
                      color: resolvedColor,
                      height: 20,
                      fontSize: "0.75rem",
                      px: 0,
                      "& .MuiChip-icon": {
                        color: "inherit",
                        ml: "6px",
                        mr: "6px",
                      },
                      "& .MuiChip-label": {
                        pl: 0,
                        pr: "6px",
                      },
                    }}
                  />
                </Stack>
              }
            />

            <Form.CardContent sx={{ p: 0 }}>
              <Typography
                variant="h6"
                color="text.primary"
                sx={{ mb: 1, fontWeight: 500 }}
              >
                {conv.initialMessage || "--"}
              </Typography>

              <Stack
                direction="row"
                spacing={2}
                alignItems="center"
                sx={{ flexWrap: "wrap", gap: 1 }}
              >
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 0.5,
                    flexShrink: 0,
                  }}
                >
                  <Calendar size={14} />
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{ lineHeight: 1 }}
                  >
                    {formatDateTime(conv.createdOn) || "--"}
                  </Typography>
                </Box>
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 0.5,
                    flexShrink: 0,
                  }}
                >
                  <MessageSquare size={14} />
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{ lineHeight: 1 }}
                  >
                    {conv.messageCount}{" "}
                    {conv.messageCount === 1
                      ? ALL_CONVERSATIONS_LIST_MESSAGE_SINGULAR
                      : ALL_CONVERSATIONS_LIST_MESSAGE_PLURAL}
                  </Typography>
                </Box>
                {conv.createdBy && (
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{ lineHeight: 1 }}
                  >
                    {ALL_CONVERSATIONS_LIST_CREATED_BY_PREFIX}
                    {conv.createdBy}
                  </Typography>
                )}
              </Stack>
            </Form.CardContent>

            <Form.CardActions sx={{ p: 0, justifyContent: "flex-end" }}>
              <Button
                size="small"
                variant="text"
                color={
                  action === ConversationListRowAction.View
                    ? "secondary"
                    : "primary"
                }
                disableRipple
                onClick={(e) => {
                  e.stopPropagation();
                  onConversationClick?.(conv);
                }}
                startIcon={
                  action === ConversationListRowAction.View ? (
                    <ExternalLink size={14} />
                  ) : (
                    <Play size={14} />
                  )
                }
                sx={{ textTransform: "none", fontWeight: 500 }}
              >
                {action === ConversationListRowAction.View
                  ? ALL_CONVERSATIONS_LIST_ACTION_VIEW_LABEL
                  : ALL_CONVERSATIONS_LIST_ACTION_RESUME_LABEL}
              </Button>
            </Form.CardActions>
          </Form.CardButton>
        );
      })}
    </Box>
  );
}
