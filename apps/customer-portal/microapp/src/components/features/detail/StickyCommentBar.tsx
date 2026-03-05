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

interface StickyCommentBarProps {
  value: string;
  placeholder?: string;
  topSlot?: ReactNode;
  loading?: boolean;

  onChange: (value: string) => void;
  onSend: () => void;
}

export function StickyCommentBar({
  value,
  placeholder,
  topSlot,
  loading = false,
  onChange,
  onSend,
}: StickyCommentBarProps) {
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
      bgcolor="background.paper"
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
          disabled={loading}
          fullWidth
        />
        <IconButton color="primary" onClick={send} disabled={!hasContent || loading}>
          {loading ? (
            <CircularProgress size={20} color="primary" />
          ) : (
            <Box color={hasContent ? "primary.main" : "text.disabled"}>
              <SendHorizonal size={pxToRem(20)} />
            </Box>
          )}
        </IconButton>
      </Stack>
    </Stack>
  );
}
