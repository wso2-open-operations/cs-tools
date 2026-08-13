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

import { Box, Button, Card, Divider, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type ReactNode } from "react";
import { Link as RouterLink, useLocation } from "react-router";
import ConversationStateChip from "@components/ConversationStateChip";
import RelativeTime from "@components/RelativeTime";
import UserRefLink from "@components/UserRefLink";
import { useNavTransition } from "@hooks/useNavTransition";
import { useNormalizedIdParam } from "@hooks/useNormalizedIdParam";
import ConversationMessagesList from "@features/csm-projects/components/ConversationMessagesList";
import { useGetCsmConversationMessages } from "@features/csm-cases/api/useCsmConversationMessages";
import type { BeConversationView } from "@api/backend/types";

function MetaField({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
      >
        {label}
      </Typography>
      <Box sx={{ minWidth: 0 }}>{children}</Box>
    </Box>
  );
}

/**
 * Full read-only transcript for a single chat session, at `/conversations/:id`.
 *
 * There is no `GET /conversations/{id}` endpoint (only `POST
 * /conversations/search` and `GET /conversations/{id}/messages` exist) — so
 * the conversation's own summary metadata (number, initiator, state,
 * project, linked case) is only available when navigated here from a row
 * that already had it (the Conversations tab / preview drawer forward it via
 * router `state`, the same way `CasesList` forwards its `from` back-target).
 * A cold/direct/bookmarked visit to this URL still works — the transcript
 * itself always loads from `useGetCsmConversationMessages`, which only needs
 * the id — but the summary card is skipped since there is nothing to show.
 * Flagged as a known gap rather than worked around: a `GET
 * /conversations/{id}` endpoint would let this page (and a bookmarked/shared
 * link to it) show full metadata on a cold load too.
 */
export default function ConversationDetailPage(): JSX.Element {
  const id = useNormalizedIdParam("id");
  const navigate = useNavTransition();
  const location = useLocation();
  const state = location.state as
    | { conversation?: BeConversationView; from?: string }
    | undefined;
  const conversation = state?.conversation;
  const backTarget = state?.from;

  const { data: messages, isLoading, isError } = useGetCsmConversationMessages(id);

  const back = (): void => {
    if (backTarget) {
      navigate(backTarget);
    } else {
      navigate(-1);
    }
  };

  const BackButton = (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={back}
      sx={{ alignSelf: "flex-start" }}
    >
      Back
    </Button>
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      {BackButton}

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1, minWidth: 0 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Typography variant="h5">
            {conversation?.number || `Chat session ${id ?? ""}`}
          </Typography>
          {conversation?.state && <ConversationStateChip state={conversation.state} />}
        </Box>
      </Box>

      {conversation && (
        <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography variant="subtitle2">Overview</Typography>
          <Box
            sx={{
              display: "grid",
              gap: 2,
              gridTemplateColumns: {
                xs: "1fr",
                sm: "repeat(2, minmax(0, 1fr))",
                md: "repeat(4, minmax(0, 1fr))",
              },
            }}
          >
            <MetaField label="Initiator">
              <Typography variant="body2" noWrap>
                {conversation.createdBy ? (
                  <UserRefLink
                    name={conversation.createdBy.name || conversation.createdBy.email || "—"}
                    email={conversation.createdBy.email}
                    userId={conversation.createdBy.id}
                  />
                ) : (
                  "—"
                )}
              </Typography>
            </MetaField>
            <MetaField label="Project">
              <Typography variant="body2" noWrap>
                {conversation.project?.name || "—"}
              </Typography>
            </MetaField>
            <MetaField label="Messages">
              <Typography variant="body2">{conversation.messageCount}</Typography>
            </MetaField>
            <MetaField label="Created">
              <Typography variant="body2">
                <RelativeTime iso={conversation.createdOn} />
              </Typography>
            </MetaField>
          </Box>
          {conversation.case?.id && (
            <Box>
              <Button
                component={RouterLink}
                to={`/cases/${conversation.case.id}`}
                variant="outlined"
                size="small"
              >
                View case {conversation.case.name}
              </Button>
            </Box>
          )}
        </Card>
      )}

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Transcript</Typography>
        <Divider />
        <ConversationMessagesList messages={messages} isLoading={isLoading} isError={isError} />
      </Card>
    </Box>
  );
}
