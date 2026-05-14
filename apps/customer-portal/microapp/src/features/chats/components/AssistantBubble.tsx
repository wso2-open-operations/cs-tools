import { Card, pxToRem, Stack, Typography, type SxProps, type Theme } from "@wso2/oxygen-ui";
import type { ChatMessage } from "@features/chats/types";
import { Sparkle } from "@wso2/oxygen-ui-icons-react";
import { BlockList } from "./BlockList";

type AssistantBubbleProps = Omit<ChatMessage, "author"> & { sx?: SxProps<Theme> };

export function AssistantBubble({ blocks, timestamp = "Just Now", sx, animated = true, thinking = false, onAnimationComplete }: AssistantBubbleProps) {
  return (
    <Stack direction="row" justifyContent="start" width="100%">
      <Card
        component={Stack}
        p={1.5}
        width="100%"
        sx={{ ...sx, bgcolor: "background.paper" }}
      >
        <Stack direction="row" justifyContent="space-between" gap={1} mb={0.5}>
          <Sparkle size={pxToRem(18)} style={{ color: "var(--oxygen-palette-primary-main)" }} />

          {thinking ? (
            <Typography noWrap variant="subtitle2"
                sx={{
                  fontWeight: "medium",
                  background: "linear-gradient(90deg, #aaa 25%, #fff 50%, #aaa 75%)",
                  backgroundSize: "200% 100%",
                  WebkitBackgroundClip: "text",
                  WebkitTextFillColor: "transparent",
                  backgroundClip: "text",
                  animation: "shimmer 1.5s infinite linear",
                  "@keyframes shimmer": {
                    from: { backgroundPosition: "200% center" },
                    to: { backgroundPosition: "-200% center" },
                  },
                  opacity: "80%",
                }}
            >
              {thinking}
            </Typography>
          ) : (
            <Typography variant="subtitle2" color="text.disabled">
              {timestamp}
            </Typography>
          )}
        </Stack>
        <BlockList blocks={blocks} animated={animated} thinking={thinking} onAnimationComplete={onAnimationComplete} />
      </Card>
    </Stack>
  );
}