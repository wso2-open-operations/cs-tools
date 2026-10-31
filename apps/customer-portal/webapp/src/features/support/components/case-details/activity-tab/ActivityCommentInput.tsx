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
import { useEffect, useMemo, useState, useRef } from "react";
import { usePostComment } from "@features/support/api/usePostComment";
import { usePostAttachments } from "@features/support/api/usePostAttachments";
import { useCustomerPermissions } from "@hooks/useCustomerPermissions";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { isEntityAttachmentsEnabled } from "@/config/entityAttachmentsConfig";
import {
  confirmStoredUpload,
  uploadStoredAttachment,
  UploadConfirmPendingError,
} from "@api/storedAttachments";
import {
  hasBlobImageSource,
  rewriteBlobImageSources,
} from "@features/support/utils/inlineCommentImages";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { hasSubmittableEditorContent } from "@features/support/utils/support";
import { usePiiGuard } from "@features/support/hooks/usePiiGuard";
import PiiWarningDialog from "@features/support/components/dialogs/PiiWarningDialog";
import Editor from "@components/rich-text-editor/Editor";
import UploadAttachmentModal from "@case-details-attachments/UploadAttachmentModal";
import type { JSX } from "react";
import {
  CommentType,
  MAX_COMMENT_BODY_BYTES,
  MAX_COMMENT_CONTENT_BYTES,
} from "@features/support/constants/supportConstants";
import {
  ERROR_UNSUPPORTED_TYPE,
  ERROR_SIZE_EXCEEDED,
} from "@constants/common";
import { formatBytes } from "@features/project-details/utils/projectDetails";

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
  const { canCreateCase, isLoading: isPermissionsLoading } =
    useCustomerPermissions();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { showError } = useErrorBanner();
  const piiGuard = usePiiGuard();

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

  // Images pasted or inserted into the comment are uploaded as soon as they are
  // added (when entity attachments are on) and shown from a local blob: URL. Each
  // finished upload's id is kept against that URL; on send the URLs are replaced
  // by `/<id>` references, so no image bytes travel with the comment.
  const authFetch = useAuthApiClient();
  const imageIdsRef = useRef<Map<string, string>>(new Map());
  const previewUrlsRef = useRef<Set<string>>(new Set());
  const pendingImagesRef = useRef(0);
  const [pendingImages, setPendingImages] = useState(0);
  const uploadImagesInline = isEntityAttachmentsEnabled();

  const releasePreviews = () => {
    previewUrlsRef.current.forEach((url) => URL.revokeObjectURL(url));
    previewUrlsRef.current.clear();
    imageIdsRef.current.clear();
  };
  useEffect(() => {
    const previewUrls = previewUrlsRef.current;
    const imageIds = imageIdsRef.current;
    return () => {
      previewUrls.forEach((url) => URL.revokeObjectURL(url));
      previewUrls.clear();
      imageIds.clear();
    };
  }, []);

  const isCaseClosed = caseStatus?.toLowerCase() === "closed";

  // UTF-8 byte size of the comment body. The BE caps the whole request body, so
  // mirror it here to fail fast with a clear message instead of a 413.
  const bodyBytes = useMemo(() => new TextEncoder().encode(value).length, [value]);
  const overSizeLimit = bodyBytes > MAX_COMMENT_CONTENT_BYTES;
  const sizeError = overSizeLimit
    ? `Comment is too large (${formatBytes(bodyBytes)}). Maximum is ${formatBytes(
        MAX_COMMENT_BODY_BYTES,
      )} — upload large images as attachments instead of embedding them inline.`
    : null;

  if (isCaseClosed) return null;
  // Posting a comment is POST /cases/{id}/comments, which the backend gates
  // on the same cases/create permission -- hide the composer rather than let
  // the customer type a comment the submit will 403. Fails closed while
  // permissions are still loading: rendering the editor optimistically would
  // let a read-only user start typing before canCreateCase resolves to
  // false, only for the composer to vanish out from under their draft.
  if (isPermissionsLoading || !canCreateCase) return null;

  const isDisabled =
    !isSignedIn ||
    isAuthLoading ||
    postComment.isPending ||
    isUploadingAttachments;

  const fileSignature = (f: File) => `${f.name}-${f.size}-${f.lastModified}`;

  // Stores one image through entity-service's upload flow. Rejects after showing
  // the error, so the editor takes the image out again.
  const uploadInlineImage = async (file: File, previewUrl: string) => {
    previewUrlsRef.current.add(previewUrl);
    pendingImagesRef.current += 1;
    setPendingImages((n) => n + 1);
    try {
      const input = {
        type: "comment" as const,
        parentId: caseId,
        file,
        name: file.name || "image",
      };
      let id: string;
      try {
        id = (await uploadStoredAttachment(authFetch, input)).id;
      } catch (error) {
        // The file is already in place: finish the upload instead of repeating it.
        if (!(error instanceof UploadConfirmPendingError)) throw error;
        id = (
          await confirmStoredUpload(authFetch, {
            type: "comment",
            attachmentId: error.attachmentId,
          })
        ).id;
      }
      imageIdsRef.current.set(previewUrl, id);
    } catch (error) {
      showError(
        error instanceof Error && error.message
          ? error.message
          : "Failed to upload the image. Please try again.",
      );
      throw error;
    } finally {
      pendingImagesRef.current -= 1;
      setPendingImages((n) => n - 1);
    }
  };

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
    // Attachment-only sends have no comment body to weigh, so the size guard
    // only applies when there's text destined for the comment endpoint.
    if (hasText && overSizeLimit) {
      showError(sizeError as string);
      return;
    }

    if (pendingImagesRef.current > 0) {
      showError("Please wait for the image upload to finish.");
      return;
    }
    // Uploaded images are sent as `/<id>` references; a preview URL that is left
    // is an image that never finished uploading and must not be sent.
    const finalValue = rewriteBlobImageSources(
      currentValue,
      imageIdsRef.current,
    );
    if (hasBlobImageSource(finalValue)) {
      showError(
        "An image in the comment could not be uploaded. Remove it and try again.",
      );
      return;
    }

    // Snapshot attachments before posting
    const attachmentsSnapshot = [...currentAttachments];
    const attachmentNamesSnapshot = new Map(attachmentNamesRef.current);

    const clearUI = () => {
      setValue("");
      setResetTrigger((prev) => prev + 1);
      setAttachments([]);
      attachmentNamesRef.current.clear();
      releasePreviews();
    };

    const performSend = async () => {
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
          body: { content: finalValue.trim(), type: CommentType.COMMENT },
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

    // Warn about PII in the comment text before posting. Attachment-only
    // sends have no text to scan, so they proceed directly.
    if (hasText) {
      piiGuard.checkBeforeSubmit(finalValue, performSend);
    } else {
      performSend();
    }
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
            onImageUpload={uploadImagesInline ? uploadInlineImage : undefined}
            showKeyboardHint={true}
            maxHeight="310px"
            onPasteError={(reason) =>
              showError(
                reason === "type"
                  ? ERROR_UNSUPPORTED_TYPE
                  : ERROR_SIZE_EXCEEDED,
              )
            }
            overlayElement={
              <Tooltip
                title={
                  overSizeLimit && hasSubmittableEditorContent(value)
                    ? sizeError
                    : pendingImages > 0
                      ? "Uploading image..."
                      : "Send comment"
                }
              >
                <span>
                  <IconButton
                    disabled={
                      (!hasSubmittableEditorContent(value) &&
                        attachments.length === 0) ||
                      (overSizeLimit && hasSubmittableEditorContent(value)) ||
                      pendingImages > 0 ||
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
                    {postComment.isPending ||
                    isUploadingAttachments ||
                    pendingImages > 0 ? (
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

      <PiiWarningDialog {...piiGuard.dialogProps} />
    </>
  );
}
