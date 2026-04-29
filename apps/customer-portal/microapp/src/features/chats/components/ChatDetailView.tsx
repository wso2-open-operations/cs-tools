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

import { useEffect, useLayoutEffect, useRef } from "react";
import { Grid, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { StatusChip } from "@components/support";
import { InfoField, OverlineSlot } from "@components/detail";
import { ConversationFeedback, MessageBubble, MessageBubbleSkeleton } from "@features/chats/components";
import { useLayout } from "@context/layout";
import { SectionCard } from "@components/common";
import { useDateTime } from "@shared/hooks/useDateTime";
import { useOverlineVariant } from "@shared/hooks/useOverlineVariant";
import type { Chat } from "@features/chats/types/chat.model";
import type { ChatMessage } from "@features/chats/components";

type ChatDetailViewProps = {
  data: Chat | undefined;
  messages: (ChatMessage & { id: string })[];
  isCommentsLoading: boolean;
};

export function ChatDetailView({ data, messages, isCommentsLoading }: ChatDetailViewProps) {
  const layout = useLayout();
  const { format } = useDateTime();
  const { ref, variant: overlineSlotVariant } = useOverlineVariant();
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  useLayoutEffect(() => {
    layout.setLayoutOverrides({
      title: <OverlineSlot variant={overlineSlotVariant} type="chat" id={data?.number} title={data?.description} />,
    });
    return () => {
      layout.setLayoutOverrides({ title: undefined });
    };
  }, [data, overlineSlotVariant]);

  return (
    <>
      <Stack gap={2} mb={10}>
        <Typography ref={ref} variant="h5" fontWeight="medium">
          {data?.description}
        </Typography>
        <SectionCard>
          <Grid spacing={1.5} container>
            <Grid size={6}>
              <InfoField label="Started" value={data?.createdOn ? format(data.createdOn) : undefined} />
            </Grid>
            <Grid size={6}>
              <InfoField
                label="Status"
                value={
                  data?.statusId ? (
                    <StatusChip type="chat" id={data.statusId} size="small" />
                  ) : (
                    <Skeleton variant="text" width={50} height={30} />
                  )
                }
              />
            </Grid>
            <Grid size={6}>
              <InfoField label="Messages Exchanged" value={`${data?.count} messages`} />
            </Grid>
            <Grid size={6}>
              <InfoField label="Available KBs" value="0 KB articles" />
            </Grid>
          </Grid>
        </SectionCard>

        <SectionCard title="Conversation">
          {isCommentsLoading ? (
            <MessagesListContentSkeleton />
          ) : (
            <Stack gap={2} mt={1}>
              {messages
                .slice()
                .reverse()
                .map(({ id, ...message }) => (
                  <MessageBubble
                    key={id}
                    {...message}
                    sx={{ bgcolor: "background.default" }}
                    thinking={false}
                    animated={false}
                  />
                ))}
            </Stack>
          )}
        </SectionCard>
        <ConversationFeedback />
      </Stack>
      <div ref={bottomRef} />
    </>
  );
}

function MessagesListContentSkeleton({ count = 6 }: { count?: number }) {
  return (
    <Stack gap={2} p={2} width="100%">
      {Array.from({ length: count }).map((_, index) => {
        const author = index % 2 === 0 ? "assistant" : "you";
        return <MessageBubbleSkeleton key={index} author={author} />;
      })}
    </Stack>
  );
}
