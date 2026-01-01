import { Stars } from "@mui/icons-material";
import { Card, Divider, Stack, Typography, type SxProps, type Theme } from "@mui/material";
import { KBCard } from "./KBCard";
import { ChecklistItem } from "./ChecklistItem";

export type MessageBlock =
  | { type: "text"; value: string }
  | { type: "checklist"; items: string[] }
  | { type: "kb"; items: { id: string; title: string }[] };

export interface ChatMessage {
  author: "you" | "assistant";
  blocks: MessageBlock[];
  timestamp?: string;
}

export function MessageBubble({ author, blocks, timestamp = "Just Now", sx }: ChatMessage & { sx?: SxProps<Theme> }) {
  const you = author === "you";

  return (
    <Stack direction="row" justifyContent={you ? "end" : "start"}>
      <Card
        component={Stack}
        p={1.5}
        elevation={0}
        ml={you ? 10 : undefined}
        width={you ? "fit-content" : undefined}
        sx={sx}
      >
        {!you && (
          <Stack direction="row" justifyContent="start" gap={1} mb={1}>
            <Stars color="primary" sx={(theme) => ({ fontSize: theme.typography.pxToRem(22) })} />
            <Typography variant="subtitle2" fontWeight="regular" color="text.disabled">
              {timestamp}
            </Typography>
          </Stack>
        )}

        <Stack gap={2}>
          {blocks.map((block, index) => {
            switch (block.type) {
              case "text":
                return (
                  <Typography key={index} variant="body2">
                    {block.value}
                  </Typography>
                );

              case "checklist":
                return (
                  <Stack key={index} gap={1}>
                    {block.items.map((item, i) => (
                      <ChecklistItem key={i}>{item}</ChecklistItem>
                    ))}
                  </Stack>
                );

              case "kb":
                return (
                  <Stack key={index} gap={1.5}>
                    <Divider sx={{ my: 1 }} />
                    {block.items.map((item, i) => (
                      <KBCard key={i} id={item.id} title={item.title} />
                    ))}
                  </Stack>
                );

              default:
                return null;
            }
          })}
        </Stack>

        {you && (
          <Stack direction="row" justifyContent="end">
            <Typography variant="subtitle2" fontWeight="regular" color="text.disabled" mt={1}>
              {timestamp}
            </Typography>
          </Stack>
        )}
      </Card>
    </Stack>
  );
}
