import { Card, Stack, Typography } from "@wso2/oxygen-ui";

import type { ChatMessage } from "@features/case-types/conversations/types";

import type { MESSAGE_AUTHOR_TYPES } from "@shared/constants";
import { useDateTime } from "@shared/hooks";

export interface BubbleUserProps extends ChatMessage {
  author: typeof MESSAGE_AUTHOR_TYPES.USER;
}

export function BubbleUser({ content, timestamp }: BubbleUserProps) {
  const { fromNow } = useDateTime();

  return (
    <Stack direction="row" justifyContent="end" width="100%">
      <Card component={Stack} p={1.5} ml={10} width="fit-content" sx={{ bgcolor: "background.paper" }}>
        <Typography variant="body2" component="span" sx={{ "& > *": { margin: 0, lineHeight: 1.7 } }}>
          {content}
        </Typography>
        <Stack direction="row" justifyContent="end">
          {timestamp && (
            <Typography variant="subtitle2" color="text.disabled" mt={1}>
              {fromNow(timestamp)}
            </Typography>
          )}
        </Stack>
      </Card>
    </Stack>
  );
}
