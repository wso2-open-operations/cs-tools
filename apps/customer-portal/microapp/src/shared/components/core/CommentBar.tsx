// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
import type { ChangeEvent, KeyboardEvent, ReactNode } from "react";

import { CircularProgress, IconButton, pxToRem, Stack, TextField } from "@wso2/oxygen-ui";
import { SendHorizonal } from "@wso2/oxygen-ui-icons-react";

import { useThemeMode } from "@context/theme";

interface CommentBarSlots {
  top?: ReactNode;
  bottom?: ReactNode;
}

export interface PinnedCommentBarProps {
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
  placeholder = "Add a comment...",
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
        borderTop: 1,
        borderBottom: 1,
        borderColor: "divider",
        bgcolor: mode === "dark" ? "black" : "white",
        position: "fixed",
        bottom: 95,
        left: 0,
        right: 0,
      }}
    >
      {slots?.top}

      <Stack direction="row" alignItems="center" sx={{ gap: 1.5, px: 1.5, py: 2 }}>
        <TextField
          fullWidth
          value={value}
          placeholder={placeholder}
          disabled={isDisabled}
          onChange={(event: ChangeEvent<HTMLInputElement>) => onChange(event.target.value)}
          onKeyDown={handleKeyDown}
          sx={{ fontSize: 14 }}
        />

        <IconButton size="small" color="primary" onClick={handleSend} disabled={!canSend}>
          {loading ? <CircularProgress size={18} color="primary" /> : <SendHorizonal size={pxToRem(18)} />}
        </IconButton>
      </Stack>

      {slots?.bottom}
    </Stack>
  );
}
