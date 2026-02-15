import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Card,
  colors,
  pxToRem,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { BookOpen, ChevronDown, CircleCheck, MessagesSquare } from "@wso2/oxygen-ui-icons-react";
import { MessageBubble, type ChatMessage } from "../chat";

export function ConversationSummary({ messages }: { messages: ChatMessage[] }) {
  return (
    <Card sx={{ bgcolor: "background.paper" }} p={1.5} component={Stack} gap={1}>
      <Stack direction="row" alignItems="center" gap={1}>
        <Box color="primary.main">
          <MessagesSquare size={pxToRem(18)} />
        </Box>
        <Typography variant="body1" fontWeight="medium">
          Conversation Summary
        </Typography>
      </Stack>
      <Stack gap={1}>
        <Stack>
          <Typography variant="caption" color="text.secondary">
            Messages Exchanged
          </Typography>
          <Typography variant="h6" fontWeight="medium">
            6
          </Typography>
        </Stack>
        <Stack>
          <Typography variant="caption" color="text.secondary">
            Troubleshooting attempts
          </Typography>
          <Stack direction="row" alignItems="center" gap={1}>
            <Box color="success.main">
              <CircleCheck size={pxToRem(16)} />
            </Box>
            <Typography variant="body2" fontWeight="medium">
              2 Steps Completed
            </Typography>
          </Stack>
        </Stack>
        <Stack>
          <Typography variant="caption" color="text.secondary">
            Articles Reviewed
          </Typography>
          <Stack direction="row" alignItems="center" gap={1}>
            <Box color={colors.blue[500]}>
              <BookOpen size={pxToRem(16)} />
            </Box>
            <Typography variant="body2" fontWeight="medium">
              3 Articles Suggested
            </Typography>
          </Stack>
        </Stack>
        {messages.length > 0 && (
          <Accordion elevation={0} sx={{ bgcolor: "transparent", border: "none" }} disableGutters>
            <AccordionSummary expandIcon={<ChevronDown size={pxToRem(20)} />} sx={{ p: 0 }}>
              <Typography variant="subtitle1" color="text.secondary" component="span">
                View Full Conversation
              </Typography>
            </AccordionSummary>
            <AccordionDetails sx={{ p: 0 }}>
              <Stack gap={2}>
                {messages.map((message, index) => (
                  <MessageBubble key={index} {...message} sx={{ bgcolor: "background.default" }} />
                ))}
              </Stack>
            </AccordionDetails>
          </Accordion>
        )}
      </Stack>
    </Card>
  );
}
