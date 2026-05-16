import { Card, Stack, Typography } from "@wso2/oxygen-ui";

import type { ChatMessage } from "@features/chats/types";

import type { MESSAGE_AUTHOR_TYPES } from "@shared/constants";

export interface BubbleUserProps extends ChatMessage {
  author: typeof MESSAGE_AUTHOR_TYPES.USER;
}

export function BubbleUser({ content, timestamp }: BubbleUserProps) {
  return (
    <Stack direction="row" justifyContent="end" width="100%">
      <Card component={Stack} p={1.5} ml={10} width="fit-content" sx={{ bgcolor: "background.paper" }}>
        <Typography variant="body2" component="span" sx={{ "& > *": { margin: 0, lineHeight: 1.7 } }}>
          {content}
        </Typography>
        <Stack direction="row" justifyContent="end">
          <Typography variant="subtitle2" color="text.disabled" mt={1}>
            {timestamp}
          </Typography>
        </Stack>
      </Card>
    </Stack>
  );
}
