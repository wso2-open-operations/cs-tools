import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Box, Divider, Grid, Skeleton, Stack, Typography, pxToRem } from "@wso2/oxygen-ui";
import { BookOpen, MessageSquare } from "@wso2/oxygen-ui-icons-react";
import { StatusChip } from "@components/features/support";
import { InfoField, OverlineSlot, StickyCommentBar } from "@components/features/detail";
import {
  ConversationFeedback,
  MessageBubble,
  MessageBubbleSkeleton,
  type ChatMessage,
} from "@components/features/chat";
import { useLayout } from "@context/layout";
import { SectionCard } from "@components/shared";
import { useParams } from "react-router-dom";
import { useMutation, useQueries, useQuery } from "@tanstack/react-query";
import { chats } from "@src/services/chats";
import { useProject } from "../context/project";
import type { MessageDispatchDTO } from "../types/chat.dto";
import { projects } from "../services/projects";

dayjs.extend(relativeTime);

export default function ChatDetailPage() {
  const layout = useLayout();
  const [comment, setComment] = useState("");
  const [messages, setMessages] = useState<ChatMessage[]>([]);

  const { id } = useParams();
  const { projectId } = useProject();
  const { data } = useQuery(chats.get(id!));
  const { data: comments, isLoading: isCommentsLoading } = useQuery(chats.comments(id!));

  const { mutate: createMessage, isPending: isCreatingMessage } = useMutation({
    ...chats.send(projectId!, id!),
    onSuccess: (response) => {
      setMessages((prev) => [
        ...prev,
        {
          author: "assistant",
          blocks: [{ type: "text", value: response.content }],
          timestamp: dayjs(response.timestamp).fromNow(),
        },
      ]);
    },
  });

  const { data: deployments = [] } = useQuery(projects.deployments(projectId!));

  const productQueries = useQueries({
    queries: deployments.map((deployment) => ({
      ...projects.products(deployment.id),
      enabled: !!deployment.id,
    })),
  });

  const envProducts = deployments.reduce((acc, deployment, index) => {
    const products = productQueries[index]?.data ?? [];
    const productNames = products.map((p) => p.name);

    return {
      ...acc,
      [deployment.name]: productNames,
    };
  }, {});

  const handleSend = () => {
    if (!comment.trim()) return;

    setMessages((prev) => [
      ...prev,
      {
        author: "you",
        blocks: [{ type: "text", value: comment }],
        timestamp: dayjs().fromNow(),
      },
    ]);

    const payload: Omit<MessageDispatchDTO, "region" | "tier"> = { message: comment, envProducts: envProducts };
    createMessage(payload);
    setComment("");
  };

  useEffect(() => {
    setMessages(
      comments?.map((comment) => ({
        author: comment.createdBy === "Novera" ? "assistant" : "you",
        blocks: [{ type: "text", value: comment.content || "" }],
        timestamp: dayjs(comment.createdOn).fromNow(),
      })) || [],
    );
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
      <Stack gap={2} mb={10}>
        <Typography ref={ref} variant="h5" fontWeight="medium">
          {data?.description}
        </Typography>
        <SectionCard>
          <Grid spacing={1.5} container>
            <Grid size={6}>
              <InfoField label="Started" value={dayjs(data?.createdOn).format("MMM D, YYYY h:mm A")} />
            </Grid>
            <Grid size={6}>
              <InfoField
                label="Status"
                value={
                  data?.statusId ? (
                    <StatusChip id={data.statusId} size="small" />
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
              {messages.map((message, index) => (
                <MessageBubble key={index} {...message} sx={{ bgcolor: "background.default" }} />
              ))}
            </Stack>
          )}
        </SectionCard>
        <ConversationFeedback />
      </Stack>
      <StickyCommentBar
        loading={isCreatingMessage}
        placeholder="Continue with the chat"
        value={comment}
        onChange={setComment}
        onSend={handleSend}
      />
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
