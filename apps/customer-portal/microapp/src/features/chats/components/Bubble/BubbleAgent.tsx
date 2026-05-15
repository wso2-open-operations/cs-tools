import { Card, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
import type { ChatMessage } from "@features/chats/types";
import { Sparkle } from "@wso2/oxygen-ui-icons-react";
import { TypewriterText } from "@shared/components/common";
import type { MESSAGE_AUTHOR_TYPES } from "@shared/constants";

export interface BubbleAgentProps extends ChatMessage {
  author: typeof MESSAGE_AUTHOR_TYPES.AGENT,
  animated?: boolean;
  /** Pass a string to show as the thinking label, or false to hide. */
  thinking?: string | boolean;
  onAnimationComplete?: () => void;
}

export function BubbleAgent({ content, timestamp, animated = true, thinking = false, onAnimationComplete }: BubbleAgentProps) {
  return (
    <Stack direction="row" justifyContent="start" width="100%">
      <Card
        component={Stack}
        p={1.5}
        width="100%"
        sx={{ bgcolor: "background.paper" }}
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
              {timestamp ?? "Just Now"}
            </Typography>
          )}
        </Stack>
          <Typography variant="body2" component="span" sx={{ "& > *": { margin: 0, lineHeight: 1.7 } }}>
            <TypewriterText
              tokens={content.split("")}
              animated={animated}
              pending={!!thinking}
              onAnimationComplete={onAnimationComplete}
            />
          </Typography>
        </Card>
    </Stack>
  );
}