import { useState } from "react";
import { Card, IconButton, pxToRem, Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import { ThumbsDown, ThumbsUp } from "@wso2/oxygen-ui-icons-react";

export function ConversationFeedback() {
  const theme = useTheme();
  const [feedback, setFeedback] = useState<"up" | "down" | undefined>(undefined);
  const up = feedback === "up";
  const down = feedback === "down";

  return (
    <Card component={Stack} direction="row" justifyContent="space-between" p={1.5}>
      <Stack>
        <Typography variant="body1" fontWeight="medium">
          Was this conversation helpful?
        </Typography>
        <Typography variant="subtitle2" color="text.secondary">
          Your feedback helps us improve Novera
        </Typography>
      </Stack>
      <Stack direction="row">
        <IconButton
          onClick={() => setFeedback("up")}
          sx={{ color: up ? "primary.main" : "text.secondary" }}
          disableRipple
        >
          <ThumbsUp size={pxToRem(18)} fill={up ? theme.palette.primary.main : "none"} />
        </IconButton>
        <IconButton
          onClick={() => setFeedback("down")}
          sx={{ color: down ? "primary.main" : "text.secondary" }}
          disableRipple
        >
          <ThumbsDown size={pxToRem(18)} fill={down ? theme.palette.primary.main : "none"} />
        </IconButton>
      </Stack>
    </Card>
  );
}
