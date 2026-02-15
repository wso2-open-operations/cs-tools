import type { ChangeEvent, ReactNode, KeyboardEvent } from "react";
import { Box, IconButton, Stack, TextField, pxToRem } from "@wso2/oxygen-ui";
import { SendHorizonal } from "@wso2/oxygen-ui-icons-react";

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
    onSend();
    onChange("");
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
          size="small"
          value={value}
          placeholder={placeholder}
          sx={{ alignSelf: "center" }}
          onChange={(event: ChangeEvent<HTMLInputElement>) => onChange(event.target.value)}
          onKeyDown={handleKeyDown}
          fullWidth
        />
        <IconButton color="primary" onClick={send}>
          <Box color={hasContent ? "primary.main" : "text.disabled"}>
            <SendHorizonal size={pxToRem(20)} />
          </Box>
        </IconButton>
      </Stack>
    </Stack>
  );
}
