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

import { Box, CircularProgress, IconButton } from "@wso2/oxygen-ui";
import { Send } from "@wso2/oxygen-ui-icons-react";
import { useState, useRef } from "react";
import { usePostComment } from "@api/usePostComment";
import { usePostAttachments } from "@api/usePostAttachments";
import { useAsgardeo } from "@asgardeo/react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { stripHtml } from "@utils/support";
import Editor from "@components/common/rich-text-editor/Editor";
import UploadAttachmentModal from "@case-details-attachments/UploadAttachmentModal";
import type { JSX } from "react";
import { CommentType } from "@/constants/supportConstants";

export interface ActivityCommentInputProps {
  caseId: string;
  focusMode?: boolean;
  caseStatus?: string | null;
}

/**
 * Input row with rich-text editor and send button.
 * Uses the shared Editor component and posts comments via usePostComment; on success invalidates and refetches comments.
 *
 * @param {ActivityCommentInputProps} props - caseId for POST.
 * @returns {JSX.Element} The comment input component.
 */
export default function ActivityCommentInput({
  caseId,
  focusMode = false,
  caseStatus,
}: ActivityCommentInputProps): JSX.Element {
  const [value, setValue] = useState("");
  const [resetTrigger, setResetTrigger] = useState(0);
  const [isFocused, setIsFocused] = useState(false);
  const postComment = usePostComment();
  const postAttachments = usePostAttachments();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { showError } = useErrorBanner();

  // Attachment state
  type AttachmentItem = { id: string; file: File };
  const [attachments, setAttachments] = useState<AttachmentItem[]>([]);
  const attachmentNamesRef = useRef<Map<string, string>>(new Map());
  const attachmentIdCounterRef = useRef(0);
  const [isAttachmentModalOpen, setIsAttachmentModalOpen] = useState(false);
  const [isUploadingAttachments, setIsUploadingAttachments] = useState(false);

  const isCaseClosed = caseStatus?.toLowerCase() === "closed";
  const isDisabled =
    !isSignedIn ||
    isAuthLoading ||
    postComment.isPending ||
    isUploadingAttachments ||
    isCaseClosed;

  const fileSignature = (f: File) => `${f.name}-${f.size}-${f.lastModified}`;

  const handleAttachmentClick = () => {
    setIsAttachmentModalOpen(true);
  };

  const handleSelectAttachment = (file: File, attachmentName?: string) => {
    setAttachments((prev) => {
      const isDuplicate = prev.some(
        (a) => fileSignature(a.file) === fileSignature(file),
      );
      if (isDuplicate) return prev;
      const uniqueId = `att-${++attachmentIdCounterRef.current}-${Date.now()}`;
      if (attachmentName?.trim()) {
        attachmentNamesRef.current.set(uniqueId, attachmentName.trim());
      }
      return [...prev, { id: uniqueId, file }];
    });
  };

  const handleAttachmentRemove = (index: number) => {
    setAttachments((prev) => {
      const item = prev[index];
      if (item) {
        attachmentNamesRef.current.delete(item.id);
      }
      return prev.filter((_, i) => i !== index);
    });
  };

  const uploadAttachmentsFromSnapshot = async (
    attachmentsSnapshot: Array<{ id: string; file: File }>,
    namesSnapshot: Map<string, string>,
  ): Promise<boolean> => {
    if (attachmentsSnapshot.length === 0) return true;

    setIsUploadingAttachments(true);
    try {
      for (const { id, file } of attachmentsSnapshot) {
        const attachmentName = namesSnapshot.get(id) || file.name;

        await new Promise<void>((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = () => {
            const base64 =
              typeof reader.result === "string" ? reader.result : "";
            const commaIndex = base64.indexOf(",");
            const content =
              commaIndex >= 0 ? base64.slice(commaIndex + 1) : base64;

            const body = {
              name: attachmentName,
              type: file.type || "application/octet-stream",
              content,
            };

            postAttachments.mutate(
              { caseId, body },
              {
                onSuccess: () => resolve(),
                onError: (error) => reject(error),
              },
            );
          };
          reader.onerror = () => reject(new Error("Failed to read file"));
          reader.readAsDataURL(file);
        });
      }
      return true;
    } catch (error) {
      const message =
        error instanceof Error && error.message
          ? error.message
          : "Failed to upload attachments. Please try again.";
      showError(message);
      return false;
    } finally {
      setIsUploadingAttachments(false);
    }
  };

  const handleSend = async () => {
    if (stripHtml(value).trim().length === 0 || isDisabled) return;

    // Snapshot attachments before posting
    const attachmentsSnapshot = [...attachments];
    const attachmentNamesSnapshot = new Map(attachmentNamesRef.current);

    postComment.mutate(
      { caseId, body: { content: value.trim(), type: CommentType.COMMENT } },
      {
        onSuccess: async () => {
          // Clear UI immediately to prevent duplicate posts
          setValue("");
          setResetTrigger((prev) => prev + 1);
          setAttachments([]);
          attachmentNamesRef.current.clear();

          // Upload attachments using snapshot
          if (attachmentsSnapshot.length > 0) {
            await uploadAttachmentsFromSnapshot(
              attachmentsSnapshot,
              attachmentNamesSnapshot,
            );
          }
        },
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
    <>
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          gap: 1,
          px: 2,
          py: 1.5,
          flexShrink: 0,
        }}
      >
        <Box sx={{ flex: 1, position: "relative" }}>
          <Editor
            value={value}
            onChange={setValue}
            disabled={isDisabled}
            resetTrigger={resetTrigger}
            minHeight={focusMode ? 60 : 40}
            showToolbar={true}
            placeholder={
              isCaseClosed
                ? "Commenting is disabled for closed cases"
                : "Write a comment..."
            }
            onSubmitKeyDown={handleSend}
            onAttachmentClick={handleAttachmentClick}
            attachments={attachments.map((a) => a.file)}
            onAttachmentRemove={handleAttachmentRemove}
            showKeyboardHint={true}
            maxHeight={isFocused ? "310px" : focusMode ? "60px" : "40px"}
            onFocus={() => setIsFocused(true)}
            onBlur={() => setIsFocused(false)}
          />
          <Box
            sx={{
              position: "absolute",
              bottom: 8,
              right: 8,
              zIndex: 1,
              display: "flex",
              gap: 1,
            }}
          >
            <IconButton
              disabled={stripHtml(value).trim().length === 0 || isDisabled}
              onClick={handleSend}
              color="warning"
              aria-label="Send comment"
              sx={{
                bgcolor: "background.paper",
                "&:hover": { bgcolor: "action.hover" },
                boxShadow: 1,
                width: 32,
                height: 32,
              }}
            >
              {postComment.isPending || isUploadingAttachments ? (
                <CircularProgress color="inherit" size={18} />
              ) : (
                <Send size={18} />
              )}
            </IconButton>
          </Box>
        </Box>
      </Box>

      <UploadAttachmentModal
        open={isAttachmentModalOpen}
        onClose={() => setIsAttachmentModalOpen(false)}
        onSelect={handleSelectAttachment}
      />
    </>
  );
}
