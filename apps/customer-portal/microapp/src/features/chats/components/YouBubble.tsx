import type { ChatMessage } from "@features/chats/types";
import { Card, Stack, Typography, type SxProps, type Theme } from "@wso2/oxygen-ui";
import { BlockList } from "@features/chats/components";

type YouBubbleProps = Omit<ChatMessage, "author"> & { sx?: SxProps<Theme> };

export function YouBubble({ blocks, timestamp = "Just Now", sx, animated = true, thinking = false, onAnimationComplete }: YouBubbleProps) {
  return (
    <Stack direction="row" justifyContent="end" width="100%">
      <Card
        component={Stack}
        p={1.5}
        ml={10}
        width="fit-content"
        sx={{ ...sx, bgcolor: "background.paper" }}
      >
        <BlockList blocks={blocks} animated={animated} thinking={thinking} onAnimationComplete={onAnimationComplete} />
        <Stack direction="row" justifyContent="end">
          <Typography variant="subtitle2" color="text.disabled" mt={1}>
            {timestamp}
          </Typography>
        </Stack>
      </Card>
    </Stack>
  );
}
