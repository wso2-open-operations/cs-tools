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

import type { ActivityCommentInputProps } from "@features/support/types/supportComponents";
import {
  Box,
  CircularProgress,
  IconButton,
  Tooltip,
  colors,
} from "@wso2/oxygen-ui";
import { ArrowUp } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useState, useRef } from "react";
import { usePostComment } from "@features/support/api/usePostComment";
import { usePostAttachments } from "@features/support/api/usePostAttachments";
import { useAsgardeo } from "@asgardeo/react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { hasSubmittableEditorContent } from "@features/support/utils/support";
import Editor from "@components/rich-text-editor/Editor";
import UploadAttachmentModal from "@case-details-attachments/UploadAttachmentModal";
import type { JSX } from "react";
import { CommentType } from "@features/support/constants/supportConstants";

/**
 * Input row with rich-text editor and send button.
 * Uses the shared Editor component and posts comments via usePostComment; on success invalidates and refetches comments.
 *
 * @param {ActivityCommentInputProps} props - caseId for POST.
 * @returns {JSX.Element} The comment input component.
 */
export default function ActivityCommentInput({
  caseId,
  caseStatus,
}: ActivityCommentInputProps): JSX.Element | null {
  const [value, setValue] = useState("");
  const [resetTrigger, setResetTrigger] = useState(0);
  const postComment = usePostComment();
  const postAttachments = usePostAttachments();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { showError } = useErrorBanner();

  // Attachment state
  type AttachmentItem = { id: string; file: File };
  const [attachments, setAttachments] = useState<AttachmentItem[]>([]);

  // Refs so handleSend always reads the latest value/attachments even when
  // called from a stale closure (e.g. via the Ctrl+Enter command handler in
  // EnterSubmitPlugin, which may hold an outdated render's callback).
  const valueRef = useRef(value);
  const attachmentsRef = useRef(attachments);
  useEffect(() => {
    valueRef.current = value;
    attachmentsRef.current = attachments;
  }, [value, attachments]);
  const attachmentNamesRef = useRef<Map<string, string>>(new Map());
  const attachmentIdCounterRef = useRef(0);
  const [isAttachmentModalOpen, setIsAttachmentModalOpen] = useState(false);
  const [isUploadingAttachments, setIsUploadingAttachments] = useState(false);

  const isCaseClosed = caseStatus?.toLowerCase() === "closed";

  if (isCaseClosed) return null;

  const isDisabled =
    !isSignedIn ||
    isAuthLoading ||
    postComment.isPending ||
    isUploadingAttachments;

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
    const currentValue = valueRef.current;
    const currentAttachments = attachmentsRef.current;
    const hasText = hasSubmittableEditorContent(currentValue);
    const hasAttachments = currentAttachments.length > 0;
    if ((!hasText && !hasAttachments) || isDisabled) return;

    // Snapshot attachments before posting
    const attachmentsSnapshot = [...currentAttachments];
    const attachmentNamesSnapshot = new Map(attachmentNamesRef.current);

    const clearUI = () => {
      setValue("");
      setResetTrigger((prev) => prev + 1);
      setAttachments([]);
      attachmentNamesRef.current.clear();
    };

    // Attachment-only: skip comment endpoint, upload directly
    if (!hasText && hasAttachments) {
      const ok = await uploadAttachmentsFromSnapshot(
        attachmentsSnapshot,
        attachmentNamesSnapshot,
      );
      if (ok) {
        clearUI();
        window.location.reload();
      }
      return;
    }

    // Text (with or without attachments): post comment first, then upload attachments
    postComment.mutate(
      {
        caseId,
        body: { content: currentValue.trim(), type: CommentType.COMMENT },
      },
      {
        onSuccess: async () => {
          clearUI();
          if (attachmentsSnapshot.length > 0) {
            const ok = await uploadAttachmentsFromSnapshot(
              attachmentsSnapshot,
              attachmentNamesSnapshot,
            );
            if (ok) {
              window.location.reload();
            }
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
      <Box>
        <Box sx={{ flex: 1 }}>
          <Editor
            onChange={setValue}
            disabled={isDisabled}
            resetTrigger={resetTrigger}
            minHeight={120}
            showToolbar={true}
            placeholder="Write a comment..."
            onSubmitKeyDown={handleSend}
            enterToSubmit={false}
            onAttachmentClick={handleAttachmentClick}
            attachments={attachments.map((a) => a.file)}
            onAttachmentRemove={handleAttachmentRemove}
            showKeyboardHint={true}
            maxHeight="310px"
            onPasteError={() =>
              showError("Image exceeds the maximum allowed size of 10 MB.")
            }
            overlayElement={
              <Tooltip title="Send comment">
                <span>
                  <IconButton
                    disabled={
                      (!hasSubmittableEditorContent(value) &&
                        attachments.length === 0) ||
                      isDisabled
                    }
                    onClick={handleSend}
                    aria-label="Send comment"
                    sx={{
                      "&.Mui-disabled": {
                        bgcolor: colors.grey[200],
                        color: colors.grey[400],
                      },
                      boxShadow: 3,
                      width: 36,
                      height: 36,
                      borderRadius: "50%",
                      border: "1px solid",
                      borderColor: "backgroundPaper",
                    }}
                  >
                    {postComment.isPending || isUploadingAttachments ? (
                      <CircularProgress color="inherit" size={18} />
                    ) : (
                      <ArrowUp size={18} />
                    )}
                  </IconButton>
                </span>
              </Tooltip>
            }
          />
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
