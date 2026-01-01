import { Article, CheckBox, ExpandMore, Forum } from "@mui/icons-material";
import { Accordion, AccordionDetails, AccordionSummary, Card, Stack, Typography } from "@mui/material";
import { MessageBubble, type ChatMessage } from "../chat";

export function ConversationSummary({ messages }: { messages: ChatMessage[] }) {
  return (
    <Card p={1.5} component={Stack} gap={1} elevation={0}>
      <Stack direction="row" alignItems="center" gap={1}>
        <Forum color="primary" />
        <Typography variant="h6" fontWeight="medium">
          Conversation Summary
        </Typography>
      </Stack>
      <Stack gap={0.8}>
        <Stack direction="row" gap={2} alignItems="center">
          <Typography variant="body2" color="text.secondary">
            Messages Exchanged
          </Typography>
          <Typography variant="body2" fontWeight="medium">
            6
          </Typography>
        </Stack>
        <Stack direction="row" gap={2} alignItems="center">
          <Typography variant="body2" color="text.secondary">
            Troubleshooting attempts
          </Typography>
          <Stack direction="row" alignItems="center" gap={1}>
            <CheckBox color="success" />
            <Typography variant="body2" fontWeight="medium">
              2 Steps Completed
            </Typography>
          </Stack>
        </Stack>
        <Stack direction="row" gap={2} alignItems="center">
          <Typography variant="body2" color="text.secondary">
            Articles Reviewed
          </Typography>
          <Stack direction="row" alignItems="center" gap={1}>
            <Article sx={{ color: "components.portal.accent.blue" }} />
            <Typography variant="body2" fontWeight="medium">
              3 Articles Suggested
            </Typography>
          </Stack>
        </Stack>
        {messages.length > 0 && (
          <Accordion
            elevation={0}
            sx={{
              "&:before": {
                display: "none",
              },
            }}
            disableGutters
          >
            <AccordionSummary expandIcon={<ExpandMore />} sx={{ p: 0 }}>
              <Typography variant="subtitle1" fontWeight="bold" color="text.secondary" component="span">
                View Full Conversation
              </Typography>
            </AccordionSummary>
            <AccordionDetails sx={{ p: 0 }}>
              <Stack gap={2}>
                {messages.map((message, index) => (
                  <MessageBubble key={index} {...message} sx={{ bgcolor: "background.card" }} />
                ))}
              </Stack>
            </AccordionDetails>
          </Accordion>
        )}
      </Stack>
    </Card>
  );
}
