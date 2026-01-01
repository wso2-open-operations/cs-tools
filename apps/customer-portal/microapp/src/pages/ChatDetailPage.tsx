import { useState } from "react";
import { StatusChip } from "@components/features/support";
import { Card, Grid, Stack, Typography } from "@mui/material";
import { InfoField, StickyCommentBar } from "@components/features/detail";
import { ConversationFeedback, MessageBubble, type ChatMessage } from "@components/features/chat";

export default function ChatDetailPage() {
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

  return (
    <>
      <Stack gap={2} mb={10}>
        <Card component={Stack} p={1.5} gap={1.5} elevation={0}>
          <Typography variant="h5" fontWeight="medium">
            Case Information
          </Typography>
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
        </Card>
        <Card component={Stack} p={1.5} gap={1.5} elevation={0}>
          <Typography variant="h5" fontWeight="medium">
            Conversation
          </Typography>
          <Stack gap={2} mt={1}>
            {messages.map((message, index) => (
              <MessageBubble key={index} {...message} sx={{ bgcolor: "background.card" }} />
            ))}
          </Stack>
        </Card>
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
