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

import type { ChangeEvent, ReactNode, KeyboardEvent } from "react";
import { Box, CircularProgress, IconButton, Stack, TextField, pxToRem } from "@wso2/oxygen-ui";
import { SendHorizonal } from "@wso2/oxygen-ui-icons-react";
import { useThemeMode } from "@root/src/context/theme";

interface StickyCommentBarProps {
  value: string;
  placeholder?: string;
  topSlot?: ReactNode;
  bottomSlot?: ReactNode;
  loading?: boolean;
  disabled?: boolean;

  onChange: (value: string) => void;
  onSend: () => void;
}

export function StickyCommentBar({
  value,
  placeholder,
  topSlot,
  bottomSlot,
  loading = false,
  disabled = false,
  onChange,
  onSend,
}: StickyCommentBarProps) {
  const mode = useThemeMode();
  const hasContent = value.trim().length > 0;

  const send = () => {
    if (loading || !hasContent) return;
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
      bottom={90}
      gap={4}
      justifyContent="space-between"
      bgcolor={mode === "dark" ? "black" : "white"}
    >
      {topSlot && <Box sx={{ m: -2 }}>{topSlot}</Box>}

      <Stack direction="row" gap={2}>
        <TextField
          size="small"
          value={value}
          placeholder={placeholder}
          sx={{ alignSelf: "center" }}
          onChange={(event: ChangeEvent<HTMLInputElement>) => onChange(event.target.value)}
          onKeyDown={handleKeyDown}
          disabled={loading || disabled}
          fullWidth
        />
        <IconButton color="primary" onClick={send} disabled={!hasContent || loading || disabled}>
          {loading ? (
            <CircularProgress size={20} color="primary" />
          ) : (
            <Box color={hasContent ? "primary.main" : "text.disabled"}>
              <SendHorizonal size={pxToRem(20)} />
            </Box>
          )}
        </IconButton>
      </Stack>

      {bottomSlot && <Box sx={{ m: -2 }}>{bottomSlot}</Box>}
    </Stack>
  );
}
