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

import { Box, Button, Divider, IconButton, Typography } from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX, type ReactNode } from "react";
import { Link as RouterLink } from "react-router";
import ConversationStateChip from "@components/ConversationStateChip";
import RelativeTime from "@components/RelativeTime";
import UserRefLink from "@components/UserRefLink";
import ConversationMessagesList from "@features/csm-projects/components/ConversationMessagesList";
import { useGetCsmConversationMessages } from "@features/csm-cases/api/useCsmConversationMessages";
import type { BeConversationView } from "@api/backend/types";

// "The last few at least" — mirrors CasePreviewContent's RECENT_COMMENTS_LIMIT:
// a quick look, not a second copy of the full transcript page.
const RECENT_MESSAGES_LIMIT = 5;

interface ConversationPreviewContentProps {
  conversation: BeConversationView;
  onClose: () => void;
}

function MetaField({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body2" noWrap>
        {children}
      </Typography>
    </Box>
  );
}

/**
 * The actual "quick look" content for a conversation row — number/state, a
 * meta grid, and the last handful of messages, plus a "View full details"
 * escape hatch to `ConversationDetailPage`. Mirrors `CasePreviewContent`'s
 * scope and layout so the two preview surfaces feel consistent.
 */
export default function ConversationPreviewContent({
  conversation,
  onClose,
}: ConversationPreviewContentProps): JSX.Element {
  const { data: messages, isLoading, isError } = useGetCsmConversationMessages(conversation.id);

  const recentMessages = useMemo(() => {
    if (!messages) return [];
    // Keep transcript order (oldest -> newest) — just the tail end of it —
    // rather than re-sorting, so the preview still reads like a conversation.
    return messages.slice(-RECENT_MESSAGES_LIMIT);
  }, [messages]);

  const initiator = conversation.createdBy;
  const detailPath = conversation.id ? `/conversations/${conversation.id}` : undefined;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <Box sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
        <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 1 }}>
          <Box sx={{ minWidth: 0 }}>
            <Typography variant="subtitle2" color="text.secondary" noWrap>
              Chat session
            </Typography>
            <Typography variant="h6" sx={{ wordBreak: "break-word" }}>
              {conversation.number || conversation.id || "—"}
            </Typography>
          </Box>
          <IconButton size="small" onClick={onClose} aria-label="Close preview">
            <X size={18} />
          </IconButton>
        </Box>
        <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
          {conversation.state ? (
            <ConversationStateChip state={conversation.state} variant="outlined" />
          ) : (
            <Typography variant="body2" color="text.secondary">
              —
            </Typography>
          )}
        </Box>
      </Box>

      <Divider />

      <Box sx={{ flex: 1, overflowY: "auto", p: 2.5, display: "flex", flexDirection: "column", gap: 2.5 }}>
        <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
          <MetaField label="Initiator">
            {initiator ? (
              <UserRefLink
                name={initiator.name || initiator.email || "—"}
                email={initiator.email}
                userId={initiator.id}
              />
            ) : (
              "—"
            )}
          </MetaField>
          <MetaField label="Messages">{conversation.messageCount}</MetaField>
          <MetaField label="Created">
            <RelativeTime iso={conversation.createdOn} />
          </MetaField>
          <MetaField label="Project">{conversation.project?.name || "—"}</MetaField>
        </Box>

        {conversation.case?.id && (
          <Box>
            <Button
              component={RouterLink}
              to={`/cases/${conversation.case.id}`}
              variant="outlined"
              size="small"
              onClick={onClose}
            >
              View case {conversation.case.name}
            </Button>
          </Box>
        )}

        <Box>
          <Typography variant="subtitle2" sx={{ mb: 1.5 }}>
            Recent messages
          </Typography>
          <ConversationMessagesList
            messages={recentMessages}
            isLoading={isLoading}
            isError={isError}
            compact
          />
        </Box>
      </Box>

      <Divider />

      <Box sx={{ p: 2 }}>
        {detailPath ? (
          <Button
            component={RouterLink}
            to={detailPath}
            variant="contained"
            fullWidth
            onClick={onClose}
            state={{ conversation }}
          >
            View full details
          </Button>
        ) : (
          <Button variant="contained" fullWidth disabled>
            View full details
          </Button>
        )}
      </Box>
    </Box>
  );
}
