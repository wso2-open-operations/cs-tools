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

import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { Send } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useRef, useState, type JSX } from "react";
import Editor from "@components/rich-text-editor/Editor";

interface CsmCaseCommentInputProps {
  onSubmit: (html: string) => Promise<unknown> | void;
  disabled?: boolean;
}

/** Strip tags + collapse whitespace to decide if the editor is effectively empty. */
function isEmpty(html: string): boolean {
  const text = html.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim();
  return text.length === 0;
}

export default function CsmCaseCommentInput({
  onSubmit,
  disabled = false,
}: CsmCaseCommentInputProps): JSX.Element {
  const [html, setHtml] = useState<string>("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Incrementing this trigger clears the editor (see Editor's ResetPlugin).
  const resetTriggerRef = useRef(0);
  const [resetTrigger, setResetTrigger] = useState(0);

  const submit = useCallback(async () => {
    if (submitting || disabled) return;
    if (isEmpty(html)) {
      setError("Comment is empty.");
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      await onSubmit(html);
      setHtml("");
      resetTriggerRef.current += 1;
      setResetTrigger(resetTriggerRef.current);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to post comment.");
    } finally {
      setSubmitting(false);
    }
  }, [disabled, html, onSubmit, submitting]);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Editor
        value=""
        onChange={setHtml}
        resetTrigger={resetTrigger}
        disabled={disabled || submitting}
        placeholder="Reply to this case…"
        minHeight={96}
        maxHeight={260}
        toolbarVariant="full"
        showKeyboardHint
        enterToSubmit={false}
        onSubmitKeyDown={() => {
          void submit();
        }}
      />
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography
          variant="caption"
          color={error ? "error" : "text.secondary"}
        >
          {error ?? "Ctrl/Cmd + Enter to send."}
        </Typography>
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={<Send size={16} />}
          disabled={disabled || submitting || isEmpty(html)}
          onClick={() => {
            void submit();
          }}
        >
          {submitting ? "Sending…" : "Send"}
        </Button>
      </Box>
    </Box>
  );
}
