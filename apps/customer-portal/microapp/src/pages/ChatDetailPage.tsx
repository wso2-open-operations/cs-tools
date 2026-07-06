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

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Grid, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { StatusChip } from "@components/features/support";
import { InfoField, OverlineSlot, StickyCommentBar } from "@components/features/detail";
import {
  ConversationFeedback,
  MessageBubble,
  MessageBubbleSkeleton,
  type ChatMessage,
} from "@components/features/chat";
import { useLayout } from "@context/layout";
import { useProject } from "@context/project";
import { useNotify } from "@context/snackbar";
import { SectionCard } from "@components/shared";
import { useParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { chats } from "@src/services/chats";
import { READ_ONLY_CONVERSATION_STATUS_IDS } from "@src/config/constants";
import { useDateTime } from "../utils/useDateTime";

export default function ChatDetailPage() {
  const layout = useLayout();
  const notify = useNotify();
  const queryClient = useQueryClient();
  const { projectId } = useProject();
  const [messages, setMessages] = useState<(ChatMessage & { id: string })[]>([]);
  const [comment, setComment] = useState("");

  const { format } = useDateTime();
  const { id } = useParams();
  const { data } = useQuery(chats.get(id!));
  const { data: comments, isLoading: isCommentsLoading } = useQuery(chats.comments(id!));

  const isReadOnly = data?.statusId !== undefined && READ_ONLY_CONVERSATION_STATUS_IDS.includes(Number(data.statusId));

  const mutation = useMutation({
    mutationFn: (body: { message: string; envProducts: Record<string, string[]> }) => {
      if (!projectId) {
        return Promise.reject(new Error("Project ID is not available"));
      }
      if (!id) {
        return Promise.reject(new Error("Conversation ID is not available"));
      }
      return chats.send(projectId, id).mutationFn!(body);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: chats.comments(id!).queryKey });
      setComment("");
    },
    onError: () => notify.error("Failed to send message. Please try again."),
  });

  const handleSend = () => {
    const trimmed = comment.trim();
    if (!trimmed || !projectId || !id || isReadOnly) return;
    mutation.mutate({ message: trimmed, envProducts: {} });
  };

  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setMessages(
      comments?.map((comment) => ({
        id: comment.id,
        author: comment.createdBy === "Novera" ? "assistant" : "you",
        blocks: [{ type: "text", value: comment.content || "" }],
        timestamp: format(comment.createdOn),
      })) || [],
    );

    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [comments]);

  const ref = useRef<HTMLSpanElement>(null);
  const [overlineSlotVariant, setOverlineSlotVariant] = useState<"normal" | "shrunk">("normal");

  useEffect(() => {
    const element = ref.current;
    if (!element) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        const next = entry.isIntersecting ? "normal" : "shrunk";
        setOverlineSlotVariant(next);
      },
      {
        root: null,
        rootMargin: "-80px 0px 0px 0px",
        threshold: 1.0,
      },
    );

    observer.observe(element);

    return () => observer.unobserve(element);
  }, []);

  useLayoutEffect(() => {
    layout.setTitleOverride(
      <OverlineSlot variant={overlineSlotVariant} type="chat" id={data?.number} title={data?.description} />,
    );

    return () => {
      layout.setTitleOverride(undefined);
    };
  }, [data, overlineSlotVariant]);

  return (
    <>
      <Stack gap={2} mb={isReadOnly ? 2 : 10}>
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
                  <MessageBubble key={id} {...message} thinking={false} animated={false} />
                ))}
            </Stack>
          )}
        </SectionCard>
        <ConversationFeedback />
      </Stack>
      <div ref={bottomRef} />

      {!isReadOnly && (
        <StickyCommentBar
          value={comment}
          placeholder="Type your message"
          submitOnEnter={false}
          multiline
          loading={mutation.isPending}
          disabled={!projectId}
          onChange={setComment}
          onSend={handleSend}
        />
      )}
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
