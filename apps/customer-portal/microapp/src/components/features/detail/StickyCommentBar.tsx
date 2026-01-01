import { Send } from "@mui/icons-material";
import { Box, IconButton, Stack, InputBase as TextField } from "@mui/material";
import type { ChangeEvent, ReactNode, KeyboardEvent } from "react";

interface StickyCommentBarProps {
  value: string;
  placeholder?: string;
  topSlot?: ReactNode;

  onChange: (value: string) => void;
  onSend: () => void;
}

export function StickyCommentBar({ value, placeholder, topSlot, onChange, onSend }: StickyCommentBarProps) {
  const hasContent = value.trim().length > 0;

  const send = () => {
    onChange("");
    onSend();
  };

  const handleKeyDown = (event: KeyboardEvent) => {
    if (event.key === "Enter" && hasContent) {
      event.preventDefault();
      send();
    }
  };

  return (
    <Stack
      position="fixed"
      width="100%"
      p={2}
      mx={-2}
      bottom={100}
      gap={4}
      justifyContent="space-between"
      bgcolor="background.paper"
    >
      {topSlot && <Box>{topSlot}</Box>}

      <Stack direction="row" gap={2}>
        <TextField
          value={value}
          placeholder={placeholder}
          sx={{ alignSelf: "center" }}
          onChange={(event: ChangeEvent<HTMLInputElement>) => onChange(event.target.value)}
          onKeyDown={handleKeyDown}
          fullWidth
        />
        <IconButton color="primary" onClick={send}>
          <Send
            sx={(theme) => ({
              color: hasContent ? "primary.main" : "text.tertiary",
              fontSize: theme.typography.pxToRem(21),
            })}
          />
        </IconButton>
      </Stack>
    </Stack>
  );
}
