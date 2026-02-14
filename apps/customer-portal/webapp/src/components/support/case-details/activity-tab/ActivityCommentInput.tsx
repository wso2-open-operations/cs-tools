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

import {
  Box,
  CircularProgress,
  IconButton,
  TextField,
} from "@wso2/oxygen-ui";
import { Send } from "@wso2/oxygen-ui-icons-react";
import { useState } from "react";
import { usePostComment } from "@api/usePostComment";
import { useAsgardeo } from "@asgardeo/react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import type { JSX } from "react";

export interface ActivityCommentInputProps {
  caseId: string;
}

/**
 * Fixed (non-scrollable) input row with text field and send button.
 * Posts comment via usePostComment; on success invalidates and refetches comments.
 *
 * @param {ActivityCommentInputProps} props - caseId for POST.
 * @returns {JSX.Element} The comment input component.
 */
export default function ActivityCommentInput({
  caseId,
}: ActivityCommentInputProps): JSX.Element {
  const [value, setValue] = useState("");
  const postComment = usePostComment();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { showError } = useErrorBanner();

  const isDisabled = !isSignedIn || isAuthLoading || postComment.isPending;

  const handleSend = () => {
    if (!value.trim() || isDisabled) return;
    const content = value.trim();
    postComment.mutate(
      { caseId, body: { content } },
      {
        onSuccess: () => setValue(""),
        onError: (error: unknown) => {
          const message =
            error instanceof Error && error.message
              ? error.message
              : "Failed to post comment. Please try again.";
          showError(message);
        },
      },
    );
  };

  return (
    <Box
      sx={{
        display: "flex",
        gap: 1,
        px: 2,
        py: 1.5,
        flexShrink: 0,
      }}
    >
      <TextField
        fullWidth
        placeholder="Add a comment..."
        size="small"
        value={value}
        disabled={isDisabled}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            handleSend();
          }
        }}
      />
      <IconButton
        disabled={!value.trim() || isDisabled}
        onClick={handleSend}
        color="warning"
        aria-label="Send comment"
      >
        {postComment.isPending ? (
          <CircularProgress color="inherit" size={18} />
        ) : (
          <Send size={18} />
        )}
      </IconButton>
    </Box>
  );
}
