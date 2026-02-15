import { useLayoutEffect, useState } from "react";
import { Box, Grid, Stack, Typography, pxToRem } from "@wso2/oxygen-ui";
import { BookOpen, MessageSquare } from "@wso2/oxygen-ui-icons-react";
import { StatusChip } from "@components/features/support";
import { InfoField, OverlineSlot, StickyCommentBar } from "@components/features/detail";
import { ConversationFeedback, MessageBubble, type ChatMessage } from "@components/features/chat";
import { useLayout } from "@context/layout";
import { SectionCard } from "@components/shared";

export default function ChatDetailPage() {
  const layout = useLayout();
  const [comment, setComment] = useState("");
  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      author: "assistant",
      blocks: [
        {
          type: "text",
          value:
            "Hi! I'm Novera, your AI-powered support assistant. How can I help you today? Please describe the issue you're experiencing.",
        },
      ],
    },
    {
      author: "you",
      blocks: [
        {
          type: "text",
          value: "Can you help me configure asgardeo?",
        },
      ],
    },
    {
      author: "assistant",
      blocks: [
        {
          type: "text",
          value: "Thanks for those details. Based on what you've shared, here are a few things to check:",
        },
        {
          type: "checklist",
          items: [
            "Verify your backend service timeout configurations",
            "Check system resource utilization CPU, memory",
            "Review recent deployment or configuration changes",
          ],
        },
        {
          type: "kb",
          items: [
            { id: "KB-1234", title: "Troubleshooting API Gateway Timeouts" },
            { id: "KB-1234", title: "Troubleshooting API Gateway Timeouts" },
          ],
        },
      ],
    },
  ]);
  const handleSend = () => {
    if (!comment.trim()) return;

    const userMessage: ChatMessage = {
      author: "you",
      blocks: [{ type: "text", value: comment }],
    };

    const assistantMessage: ChatMessage = {
      author: "assistant",
      blocks: [
        {
          type: "text",
          value: "Thanks for those details. Based on what you've shared, here are a few things to check:",
        },
        {
          type: "checklist",
          items: [
            "Verify your backend service timeout configurations",
            "Check system resource utilization CPU, memory",
            "Review recent deployment or configuration changes",
          ],
        },
        {
          type: "kb",
          items: [
            { id: "KB-1234", title: "Troubleshooting API Gateway Timeouts" },
            { id: "KB-1234", title: "Troubleshooting API Gateway Timeouts" },
          ],
        },
      ],
    };

    setMessages((prev) => [...prev, userMessage]);
    setTimeout(() => setMessages((prev) => [...prev, assistantMessage]), 2000);
  };

  const AppBarSlot = () => (
    <Stack direction="row" justifyContent="space-between" gap={1.5} mt={1}>
      <StatusChip status="resolved" size="small" />
      <Stack direction="row" gap={3}>
        <Stack direction="row" alignItems="center" gap={1}>
          <Box color="text.secondary">
            <MessageSquare size={pxToRem(14)} />
          </Box>
          <Typography variant="body2" color="text.secondary">
            8 messages
          </Typography>
        </Stack>
        <Stack direction="row" alignItems="center" gap={1}>
          <Box color="text.secondary">
            <BookOpen size={pxToRem(14)} />
          </Box>
          <Typography variant="body2" color="text.secondary">
            3 KB articles
          </Typography>
        </Stack>
      </Stack>
    </Stack>
  );

  useLayoutEffect(() => {
    layout.setTitleOverride("How do I configure custom claims in JWT tokens?");
    layout.setOverlineSlotOverride(<OverlineSlot type="chat" id="CH-1234" />);
    layout.setAppBarSlotsOverride(<AppBarSlot />);

    return () => {
      layout.setTitleOverride(undefined);
      layout.setOverlineSlotOverride(undefined);
      layout.setAppBarSlotsOverride(undefined);
    };
  }, []);

  return (
    <>
      <Stack gap={2} mb={10}>
        <SectionCard title="Chat Information">
          <Grid spacing={1.5} container>
            <Grid size={6}>
              <InfoField label="Started" value="Nov 17, 2025 2:15 PM" />
            </Grid>
            <Grid size={6}>
              <InfoField label="Resolved" value="Nov 17, 2025 4:30 PM" />
            </Grid>
            <Grid size={6}>
              <InfoField label="Duration" value="2h 15m" />
            </Grid>
            <Grid size={6}>
              <InfoField label="Status" value={<StatusChip size="small" status="resolved" />} />
            </Grid>
          </Grid>
        </SectionCard>
        <SectionCard title="Conversation">
          <Stack gap={2} mt={1}>
            {messages.map((message, index) => (
              <MessageBubble key={index} {...message} sx={{ bgcolor: "background.default" }} />
            ))}
          </Stack>
        </SectionCard>
        <ConversationFeedback />
      </Stack>
      <StickyCommentBar
        placeholder="Continue with the chat"
        value={comment}
        onChange={setComment}
        onSend={handleSend}
      />
    </>
  );
}
