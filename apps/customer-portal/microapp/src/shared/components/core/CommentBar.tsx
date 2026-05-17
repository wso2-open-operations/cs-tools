import type { ChangeEvent, KeyboardEvent, ReactNode } from "react";

import { CircularProgress, IconButton, InputBase, pxToRem, Stack } from "@wso2/oxygen-ui";
import { SendHorizonal } from "@wso2/oxygen-ui-icons-react";

import { useThemeMode } from "@context/theme";

interface CommentBarSlots {
  top?: ReactNode;
  bottom?: ReactNode;
}

interface PinnedCommentBarProps {
  value: string;
  placeholder?: string;
  loading?: boolean;
  disabled?: boolean;
  slots?: CommentBarSlots;
  onChange: (value: string) => void;
  onSend: () => void;
}

export function CommentBar({
  value,
  placeholder = "Add Comment",
  loading = false,
  disabled = false,
  slots,
  onChange,
  onSend,
}: PinnedCommentBarProps) {
  const mode = useThemeMode();

  const hasContent = value.trim().length > 0;
  const isDisabled = disabled || loading;
  const canSend = hasContent && !isDisabled;

  const handleSend = () => {
    if (!canSend) return;
    onSend();
  };

  const handleKeyDown = (event: KeyboardEvent) => {
    if (event.key === "Enter" && canSend) {
      event.preventDefault();
      handleSend();
    }
  };

  return (
    <Stack
      sx={{
        px: 2,
        py: 1.5,
        gap: 1,
        borderTop: 1,
        borderColor: "divider",
        bgcolor: mode === "dark" ? "black" : "white",
      }}
    >
      {slots?.top}

      <Stack direction="row" alignItems="center" sx={{ gap: 1 }}>
        <InputBase
          fullWidth
          value={value}
          placeholder={placeholder}
          disabled={isDisabled}
          onChange={(event: ChangeEvent<HTMLInputElement>) => onChange(event.target.value)}
          onKeyDown={handleKeyDown}
          sx={{ fontSize: pxToRem(14) }}
        />

        <IconButton size="small" color="primary" onClick={handleSend} disabled={!canSend}>
          {loading ? <CircularProgress size={18} color="primary" /> : <SendHorizonal size={pxToRem(18)} />}
        </IconButton>
      </Stack>

      {slots?.bottom}
    </Stack>
  );
}
