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

import { Alert, Box, Button, TextField, Typography } from "@wso2/oxygen-ui";
import { useState, type FormEvent, type JSX } from "react";
import { BackendApiError } from "@api/backend/client";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { getErrorReferenceId } from "@utils/correlationId";
import { usePostAllocationUpdate } from "../api/useAllocations";
import { MAX_CONTENT_LENGTH, MAX_SUBJECT_LENGTH, NOT_VISIBLE_NOTE } from "../utils/allocations";

/** The server's own reason for a refusal, or a generic line when it gave none. */
function errorMessage(error: Error): string {
  if (error instanceof BackendApiError && error.message.trim()) return error.message;
  return error.message.trim() || "The update could not be posted.";
}

/**
 * Post a status update on an allocation. Shown to everyone: whether the caller
 * may post is decided by the server (only the allocated engineer can), and a
 * refusal is shown inline in the server's words.
 */
export default function PostUpdateForm({ allocationId }: { allocationId: string }): JSX.Element {
  const [subject, setSubject] = useState("");
  const [content, setContent] = useState("");
  const post = usePostAllocationUpdate(allocationId);
  const { showSuccess } = useSuccessBanner();

  const trimmedSubject = subject.trim();
  const trimmedContent = content.trim();
  const canSubmit = trimmedSubject.length > 0 && trimmedContent.length > 0 && !post.isPending;

  const handleSubmit = (e: FormEvent): void => {
    e.preventDefault();
    if (!canSubmit) return;
    post.mutate(
      { subject: trimmedSubject, content: trimmedContent },
      {
        onSuccess: () => {
          setSubject("");
          setContent("");
          showSuccess("Update posted.");
        },
      },
    );
  };

  const referenceId = post.error ? getErrorReferenceId(post.error) : undefined;

  return (
    <Box
      component="form"
      onSubmit={handleSubmit}
      aria-labelledby="post-update-heading"
      sx={{ display: "flex", flexDirection: "column", gap: 2 }}
    >
      <Typography id="post-update-heading" variant="subtitle1" component="h3">
        Post update
      </Typography>
      <Alert severity="info" role="note">
        {NOT_VISIBLE_NOTE}
      </Alert>
      <TextField
        label="Subject"
        size="small"
        required
        fullWidth
        value={subject}
        onChange={(e) => setSubject(e.target.value)}
        disabled={post.isPending}
        slotProps={{ htmlInput: { maxLength: MAX_SUBJECT_LENGTH } }}
      />
      <TextField
        label="Content"
        size="small"
        required
        fullWidth
        multiline
        minRows={4}
        value={content}
        onChange={(e) => setContent(e.target.value)}
        disabled={post.isPending}
        helperText={`${content.length} / ${MAX_CONTENT_LENGTH}`}
        slotProps={{ htmlInput: { maxLength: MAX_CONTENT_LENGTH } }}
      />
      {post.isError && (
        <Alert severity="error" role="alert">
          {errorMessage(post.error)}
          {referenceId ? ` (Reference ID: ${referenceId})` : ""}
        </Alert>
      )}
      <Box>
        <Button type="submit" variant="contained" size="small" disabled={!canSubmit}>
          {post.isPending ? "Posting…" : "Post update"}
        </Button>
      </Box>
    </Box>
  );
}
