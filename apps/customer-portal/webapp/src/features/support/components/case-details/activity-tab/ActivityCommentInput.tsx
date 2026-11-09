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
  COMMENT_MAX_ATTACHMENTS,
  CREATE_FORM_ATTACHMENT_LIMIT_MESSAGE,
} from "@config/attachmentLimitsConfig";
import { cancelCaseUpload, useCaseUploads } from "@api/caseUploadProgress";
import { isAbortError } from "@api/storedAttachments";
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
import { useStoredAttachmentUpload } from "@api/useStoredAttachments";
import { useCustomerPermissions } from "@hooks/useCustomerPermissions";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import {
  confirmStoredUpload,
  uploadStoredAttachment,
  UploadConfirmPendingError,
} from "@api/storedAttachments";
import {
  blobImageSources,
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
  imageCountExceededMessage,
  imageSizeExceededMessage,
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
  const { upload: storedUpload } = useStoredAttachmentUpload();
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
  // Files of a sent comment still queued behind the one uploading now.
  const [waitingUploads, setWaitingUploads] = useState(0);
  // The uploads of this box's own comments, drawn inside the editor.
  const commentUploads = useCaseUploads(caseId)
    .filter((u) => u.inPlace)
    .map((u) => ({ name: u.name, percent: u.percent, onCancel: () => cancelCaseUpload(u.key) }));

  // Images pasted or inserted into the comment are shown from a local blob: URL and
  // kept here; nothing is uploaded until the comment is sent, so an image that is removed or never posted leaves
  // nothing on the server. Each finished upload's id is kept against that URL; the
  // URLs are then replaced by `/<id>` references, so no image bytes travel with
  // the comment.
  const authFetch = useAuthApiClient();
  const imageIdsRef = useRef<Map<string, string>>(new Map());
  const queuedImagesRef = useRef<Map<string, File>>(new Map());
  const previewUrlsRef = useRef<Set<string>>(new Set());
  const [pendingImages, setPendingImages] = useState(0);

  const releasePreviews = () => {
    previewUrlsRef.current.forEach((url) => URL.revokeObjectURL(url));
    previewUrlsRef.current.clear();
    imageIdsRef.current.clear();
    queuedImagesRef.current.clear();
  };
  useEffect(() => {
    const previewUrls = previewUrlsRef.current;
    const imageIds = imageIdsRef.current;
    const queued = queuedImagesRef.current;
    return () => {
      previewUrls.forEach((url) => URL.revokeObjectURL(url));
      previewUrls.clear();
      imageIds.clear();
      queued.clear();
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
    isUploadingAttachments ||
    pendingImages > 0;

  const fileSignature = (f: File) => `${f.name}-${f.size}-${f.lastModified}`;

  // Keeps one image for the send. The editor shows it from its blob: URL; it is
  // uploaded by uploadQueuedImages when the comment is sent.
  const uploadInlineImage = async (file: File, previewUrl: string) => {
    previewUrlsRef.current.add(previewUrl);
    queuedImagesRef.current.set(previewUrl, file);
  };

  // Stores the images the comment still shows, one after another, through
  // entity-service's upload flow. Resolves false after showing the error of the
  // first one that fails; the ones already stored are kept, so sending again
  // uploads only the rest.
  const uploadQueuedImages = async (html: string): Promise<boolean> => {
    // The same preview can appear more than once (a copied image): upload it once.
    const toUpload = Array.from(new Set(blobImageSources(html))).filter(
      (url) => !imageIdsRef.current.has(url) && queuedImagesRef.current.has(url),
    );
    if (toUpload.length === 0) return true;
    setPendingImages(toUpload.length);
    try {
      for (const previewUrl of toUpload) {
        const file = queuedImagesRef.current.get(previewUrl) as File;
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
              chunks: error.chunks,
            })
          ).id;
        }
        imageIdsRef.current.set(previewUrl, id);
        queuedImagesRef.current.delete(previewUrl);
        setPendingImages((n) => n - 1);
      }
      return true;
    } catch (error) {
      showError(
        error instanceof Error && error.message
          ? `${error.message} The comment was not sent; try again.`
          : "Failed to upload the image. The comment was not sent; try again.",
      );
      return false;
    } finally {
      setPendingImages(0);
    }
  };

  const handleAttachmentClick = () => {
    // A comment takes one attachment: say so instead of opening a picker whose
    // file would be refused.
    if (attachments.length >= COMMENT_MAX_ATTACHMENTS) {
      showError(CREATE_FORM_ATTACHMENT_LIMIT_MESSAGE);
      return;
    }
    setIsAttachmentModalOpen(true);
  };

  const handleSelectAttachment = (file: File, attachmentName?: string) => {
    setAttachments((prev) => {
      const isDuplicate = prev.some(
        (a) => fileSignature(a.file) === fileSignature(file),
      );
      if (isDuplicate || prev.length >= COMMENT_MAX_ATTACHMENTS) return prev;
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

  // Uploads each queued file as its own work_item attachment of the case, one
  // after another, through the same chunked flow as the Attachments tab. One
  // that fails does not stop the others; the result says which went through.
  const uploadAttachmentsFromSnapshot = async (
    attachmentsSnapshot: Array<{ id: string; file: File }>,
    namesSnapshot: Map<string, string>,
  ): Promise<{ uploadedIds: string[]; cancelledIds: string[]; failures: string[] }> => {
    // A file the person stopped counts as dealt with: it leaves the queue and is not an error.
    const uploadedIds: string[] = [];
    const cancelledIds: string[] = [];
    const failures: string[] = [];
    if (attachmentsSnapshot.length === 0) return { uploadedIds, cancelledIds, failures };

    setIsUploadingAttachments(true);
    try {
      for (const [index, { id, file }] of attachmentsSnapshot.entries()) {
        const attachmentName = namesSnapshot.get(id) || file.name;
        setWaitingUploads(attachmentsSnapshot.length - index - 1);
        try {
          await storedUpload.mutateAsync({
            type: "work_item",
            parentId: caseId,
            caseId,
            file,
            body: { name: attachmentName },
            progressInPlace: true,
          });
          uploadedIds.push(id);
        } catch (error) {
          if (isAbortError(error)) {
            cancelledIds.push(id);
            continue;
          }
          failures.push(
            `"${attachmentName}": ${
              error instanceof Error && error.message
                ? error.message
                : "the upload failed"
            }`,
          );
        }
      }
    } finally {
      setIsUploadingAttachments(false);
      setWaitingUploads(0);
    }
    return { uploadedIds, cancelledIds, failures };
  };

  // Takes the files that were stored out of the queue; a failed one stays so
  // pressing Send again uploads only it.
  const dropUploaded = (uploadedIds: string[]) => {
    if (uploadedIds.length === 0) return;
    uploadedIds.forEach((id) => attachmentNamesRef.current.delete(id));
    setAttachments((prev) => prev.filter((a) => !uploadedIds.includes(a.id)));
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

    // Snapshot attachments before posting
    const attachmentsSnapshot = [...currentAttachments];
    const attachmentNamesSnapshot = new Map(attachmentNamesRef.current);

    const clearEditor = () => {
      setValue("");
      setResetTrigger((prev) => prev + 1);
      releasePreviews();
    };

    const performSend = async () => {
      // The comment's images are uploaded now; their `/<id>` references replace
      // the preview URLs, and one that is left is an image that never uploaded and
      // must not be sent.
      let finalValue = currentValue;
      if (hasText) {
        if (!(await uploadQueuedImages(currentValue))) return;
        finalValue = rewriteBlobImageSources(
          currentValue,
          imageIdsRef.current,
        );
        if (hasBlobImageSource(finalValue)) {
          showError(
            "An image in the comment could not be uploaded. Remove it and try again.",
          );
          return;
        }
      }

      // The comment and the attachments are two separate requests, started
      // together: a comment that fails does not hold the files back, and a file
      // that fails does not lose the comment.
      const commentResult: Promise<{ ok: true } | { ok: false; message: string }> =
        hasText
          ? new Promise((resolve) =>
              postComment.mutate(
                {
                  caseId,
                  body: { content: finalValue.trim(), type: CommentType.COMMENT },
                },
                {
                  onSuccess: () => resolve({ ok: true }),
                  onError: (error: unknown) =>
                    resolve({
                      ok: false,
                      message:
                        error instanceof Error && error.message
                          ? error.message
                          : "Failed to post comment. Please try again.",
                    }),
                },
              ),
            )
          : Promise.resolve({ ok: true });
      const uploadsJob = uploadAttachmentsFromSnapshot(
        attachmentsSnapshot,
        attachmentNamesSnapshot,
      );
      const comment = await commentResult;

      // Once the comment is posted (or there is none) nothing is left to protect
      // in the editor, and the upload progress shows under it: so the editor
      // empties now instead of holding the sent text and files until the last
      // upload ends. A comment that failed stays put to be sent again.
      const commentSettled = !hasText || comment.ok;
      if (hasText && comment.ok) clearEditor();
      if (commentSettled && attachmentsSnapshot.length > 0) {
        const sentIds = new Set(attachmentsSnapshot.map((a) => a.id));
        setAttachments((prev) => prev.filter((a) => !sentIds.has(a.id)));
      }

      const uploads = await uploadsJob;
      // A cancelled file leaves the queue like a sent one, but is not a success.
      dropUploaded([...uploads.uploadedIds, ...uploads.cancelledIds]);
      if (commentSettled) {
        // A file that did not upload comes back, to be sent again.
        const failedItems = attachmentsSnapshot.filter(
          (a) =>
            !uploads.uploadedIds.includes(a.id) &&
            !uploads.cancelledIds.includes(a.id),
        );
        if (failedItems.length > 0) {
          setAttachments((prev) => [
            ...prev,
            ...failedItems.filter((f) => !prev.some((p) => p.id === f.id)),
          ]);
        }
      }

      const problems: string[] = [];
      if (!comment.ok) {
        problems.push(
          `The comment was not posted (${comment.message}). It is still in the editor; press Send to try again.`,
        );
      }
      if (uploads.failures.length > 0) {
        problems.push(
          `${uploads.failures.length === 1 ? "An attachment was" : "Some attachments were"} not uploaded: ${uploads.failures.join("; ")}. ${
            uploads.failures.length === 1 ? "It is" : "They are"
          } still attached; press Send to try again.`,
        );
      }
      if (problems.length > 0) {
        showError(problems.join(" "));
        return;
      }
      if (uploads.uploadedIds.length > 0) window.location.reload();
    };

    // Warn about PII in the comment text before posting. Attachment-only
    // sends have no text to scan, so they proceed directly.
    if (hasText) {
      piiGuard.checkBeforeSubmit(currentValue, performSend);
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
            maxAttachments={COMMENT_MAX_ATTACHMENTS}
            uploads={commentUploads}
            uploadsWaiting={waitingUploads}
            onAttachmentRemove={handleAttachmentRemove}
            onImageUpload={uploadInlineImage}
            showKeyboardHint={true}
            maxHeight="310px"
            onPasteError={(reason) =>
              showError(
                reason === "type"
                  ? ERROR_UNSUPPORTED_TYPE
                  : reason === "count"
                    ? imageCountExceededMessage()
                    : imageSizeExceededMessage(),
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
