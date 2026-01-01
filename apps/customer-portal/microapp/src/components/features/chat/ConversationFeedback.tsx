import { ThumbDownAlt, ThumbDownOffAlt, ThumbUpAlt, ThumbUpOffAlt } from "@mui/icons-material";
import { Card, IconButton, Stack, Typography } from "@mui/material";
import { useState } from "react";

export function ConversationFeedback() {
  const [feedback, setFeedback] = useState<"up" | "down" | undefined>(undefined);
  const up = feedback === "up";
  const down = feedback === "down";

  return (
    <Card component={Stack} direction="row" justifyContent="space-between" p={1.5} elevation={0}>
      <Stack>
        <Typography variant="body1" fontWeight="medium">
          Was this conversation helpful?
        </Typography>
        <Typography variant="subtitle1" color="text.secondary">
          Your feedback helps us improve Novera
        </Typography>
      </Stack>
      <Stack direction="row">
        <IconButton
          onClick={() => setFeedback("up")}
          sx={{ color: up ? "primary.main" : "text.secondary" }}
          disableRipple
        >
          {up ? <ThumbUpAlt /> : <ThumbUpOffAlt />}
        </IconButton>
        <IconButton onClick={() => setFeedback("down")} sx={{ color: "text.secondary" }} disableRipple>
          {down ? <ThumbDownAlt /> : <ThumbDownOffAlt />}
        </IconButton>
      </Stack>
    </Card>
  );
}
