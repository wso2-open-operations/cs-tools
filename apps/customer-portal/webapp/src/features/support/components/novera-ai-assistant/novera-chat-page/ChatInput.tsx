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

import type { ChatInputProps } from "@features/support/types/supportComponents";
import {
  Box,
  Button,
  IconButton,
  TextField,
  alpha,
  colors,
} from "@wso2/oxygen-ui";
import { Send, FileText } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

const CHAT_PLACEHOLDER = "Type your message...";

/**
 * Renders the input area for the Novera Chat page.
 *
 * @returns The ChatInput JSX element.
 */
export default function ChatInput({
  inputValue,
  setInputValue,
  onSend,
  onCreateCase,
  isSending = false,
  isCreateCaseLoading = false,
  disabled = false,
}: ChatInputProps): JSX.Element | null {
  const isSendDisabled = !inputValue.trim() || isSending;

  if (disabled) {
    return null;
  }

  return (
    <Box sx={{ flexShrink: 0 }}>
      {/* Create Case Banner */}
      <Box
        sx={{
          px: 6,
          py: 1.5,
          bgcolor: alpha(colors.yellow[800], 0.05),
        }}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "flex-end",
          }}
        >
          <Button
            onClick={onCreateCase}
            loading={isCreateCaseLoading}
            loadingPosition="start"
            variant="outlined"
            size="small"
            color="warning"
            startIcon={<FileText size={14} />}
            sx={{
              textTransform: "none",
              fontWeight: 500,
              backgroundColor: "background.paper",
              borderColor: "orange.300",
              textColor: "orange.600",
              "&:hover": {
                backgroundColor: "orange.50",
                borderColor: "orange.400",
              },
            }}
          >
            Create Case
          </Button>
        </Box>
      </Box>

      {/* Input Area */}
      <Box sx={{ p: 2, bgcolor: "background.paper", flexShrink: 0 }}>
        <Box sx={{ display: "flex", gap: 1, alignItems: "flex-end" }}>
          <TextField
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            placeholder={CHAT_PLACEHOLDER}
            multiline
            maxRows={5}
            fullWidth
            variant="outlined"
            size="small"
            disabled={isSending}
            sx={{
              "& textarea": {
                overflowWrap: "anywhere",
                wordBreak: "break-word",
                whiteSpace: "pre-wrap",
              },
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                if (e.nativeEvent?.isComposing) return;
                e.preventDefault();
                if (!isSendDisabled) onSend();
              }
            }}
          />

          {/* Send button */}
          <IconButton
            disabled={isSendDisabled}
            onClick={onSend}
            color="warning"
            sx={{ flexShrink: 0 }}
            aria-label="Send message"
          >
            <Send size={18} />
          </IconButton>
        </Box>
      </Box>
    </Box>
  );
}
