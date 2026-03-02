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

import { Box, CircularProgress, IconButton, TextField } from "@wso2/oxygen-ui";
import { Send } from "@wso2/oxygen-ui-icons-react";
import { useState } from "react";
import { usePostChangeRequestComment } from "@api/usePostChangeRequestComment";
import { useAsgardeo } from "@asgardeo/react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import type { JSX } from "react";
import { CommentType } from "@/constants/supportConstants";

export interface ChangeRequestCommentInputProps {
  changeRequestId: string;
}

/**
 * Simple text input with send button for change request comments.
 * No rich text editor, no attachments - simplified compared to case comments.
 *
 * @param {ChangeRequestCommentInputProps} props - changeRequestId for POST.
 * @returns {JSX.Element} The comment input component.
 */
export default function ChangeRequestCommentInput({
  changeRequestId,
}: ChangeRequestCommentInputProps): JSX.Element {
  const [value, setValue] = useState("");
  const postChangeRequestComment = usePostChangeRequestComment();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { showError } = useErrorBanner();

  const isDisabled = !isSignedIn || isAuthLoading || postChangeRequestComment.isPending;

  const handleSend = async () => {
    const trimmedValue = value.trim();
    if (trimmedValue.length === 0 || isDisabled) return;

    postChangeRequestComment.mutate(
      {
        changeRequestId,
        body: { content: trimmedValue, type: CommentType.COMMENT },
      },
      {
        onSuccess: () => {
          setValue("");
        },
        onError: (error: unknown) => {
          let message = "Failed to post comment. Please try again.";

          if (error instanceof Error && error.message) {
            try {
              const jsonMatch = error.message.match(/\{.*\}/);
              if (jsonMatch) {
                const errorData = JSON.parse(jsonMatch[0]);
                if (errorData.message) {
                  message = errorData.message;
                }
              } else {
                message = error.message;
              }
            } catch {
              message = error.message;
            }
          }

          showError(message);
        },
      },
    );
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key === "Enter" && !e.shiftKey && !(e.metaKey || e.ctrlKey)) {
      // Plain Enter key sends the comment
      e.preventDefault();
      handleSend();
    }
    // Cmd/Ctrl+Shift+Enter adds new line (default textarea behavior)
  };

  return (
    <Box
      sx={{
        display: "flex",
        gap: 1,
        px: 2,
        py: 1.5,
        flexShrink: 0,
        alignItems: "flex-end",
      }}
    >
      <TextField
        multiline
        fullWidth
        minRows={2}
        maxRows={6}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={handleKeyDown}
        disabled={isDisabled}
        placeholder="Write a Comment or Note"
        sx={{
          "& .MuiInputBase-root": {
            padding: "8px 12px",
          },
        }}
      />
      <IconButton
        disabled={value.trim().length === 0 || isDisabled}
        onClick={handleSend}
        color="warning"
        aria-label="Send comment"
        sx={{
          bgcolor: "background.paper",
          "&:hover": { bgcolor: "action.hover" },
          boxShadow: 1,
          width: 40,
          height: 40,
        }}
      >
        {postChangeRequestComment.isPending ? (
          <CircularProgress color="inherit" size={18} />
        ) : (
          <Send size={18} />
        )}
      </IconButton>
    </Box>
  );
}
