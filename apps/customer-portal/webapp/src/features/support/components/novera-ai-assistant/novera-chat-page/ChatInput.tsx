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
  Tooltip,
  alpha,
  colors,
} from "@wso2/oxygen-ui";
import { Send, PanelTopClose, FileText } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useState } from "react";
import Editor from "@components/rich-text-editor/Editor";
import { htmlToPlainText } from "@features/support/utils/richTextEditor";

const CHAT_PLACEHOLDER = "Type your message...";

/**
 * Renders the input area for the Novera Chat page with rich text editor.
 * Rich text: Ctrl+Enter or ⌘+Enter sends; Enter adds lines or list items.
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
  resetTrigger = 0,
}: ChatInputProps): JSX.Element {
  const plainText = htmlToPlainText(inputValue).trim();
  const isSendDisabled = !plainText || isSending;
  const [showToolbar, setShowToolbar] = useState(false);

  // Calculate max height for 5 lines (line-height is ~24px in body2)
  const singleLineHeight = 40;
  const maxLinesHeight = 120;
  const BUTTON_TOP_WITHOUT_TOOLBAR = 8;
  const BUTTON_TOP_WITH_TOOLBAR = 56;

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
          <Box sx={{ flex: 1, minWidth: 0, position: "relative" }}>
            {/* Toolbar toggle button as prefix */}
            <Box
              sx={{
                position: "absolute",
                left: 8,
                top: showToolbar
                  ? BUTTON_TOP_WITH_TOOLBAR
                  : BUTTON_TOP_WITHOUT_TOOLBAR,
                zIndex: 10,
                transition: "top 0.2s ease",
              }}
            >
              <Tooltip
                title={showToolbar ? "Hide formatting" : "Show formatting"}
              >
                <IconButton
                  onClick={() => setShowToolbar(!showToolbar)}
                  color="default"
                  size="small"
                  sx={{
                    flexShrink: 0,
                    width: 32,
                    height: 32,
                    "&:hover": {
                      backgroundColor: "action.hover",
                    },
                  }}
                  aria-label={
                    showToolbar ? "Hide formatting" : "Show formatting"
                  }
                >
                  <PanelTopClose size={16} />
                </IconButton>
              </Tooltip>
            </Box>

            {/* Editor with adjusted padding for prefix button */}
            <Box
              sx={{
                "& .MuiPaper-root": {
                  pl: 6, // Add padding-left to make room for the button
                },
              }}
            >
              <Editor
                id="novera-chat-input-editor"
                value={inputValue}
                onChange={setInputValue}
                placeholder={CHAT_PLACEHOLDER}
                minHeight={singleLineHeight}
                maxHeight={maxLinesHeight}
                showToolbar={showToolbar}
                toolbarVariant="describeIssue"
                onSubmitKeyDown={() => !isSendDisabled && onSend()}
                disabled={isSending}
                resetTrigger={resetTrigger}
              />
            </Box>
          </Box>

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
