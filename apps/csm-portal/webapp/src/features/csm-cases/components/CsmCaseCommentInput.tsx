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
  Button,
  FormControlLabel,
  IconButton,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  Code,
  Lock,
  Maximize2,
  Minimize2,
  Send,
} from "@wso2/oxygen-ui-icons-react";
import { useCallback, useRef, useState, type JSX } from "react";
import Editor from "@components/rich-text-editor/Editor";

interface CsmCaseCommentInputProps {
  onSubmit: (html: string, internal: boolean) => Promise<unknown> | void;
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
  const [sourceMode, setSourceMode] = useState(false);
  const [internal, setInternal] = useState(false);
  const [maximized, setMaximized] = useState(false);

  // Incrementing this trigger clears the editor (see Editor's ResetPlugin).
  const resetTriggerRef = useRef(0);
  const [resetTrigger, setResetTrigger] = useState(0);

  // Bump this on every source→rich switch so the Editor remounts with the
  // edited HTML as its new initial value (InitialValuePlugin only injects once
  // per mount).
  const [editorMountKey, setEditorMountKey] = useState(0);

  const submit = useCallback(async () => {
    if (submitting || disabled) return;
    if (isEmpty(html)) {
      setError("Comment is empty.");
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      await onSubmit(html, internal);
      setHtml("");
      resetTriggerRef.current += 1;
      setResetTrigger(resetTriggerRef.current);
      setEditorMountKey((k) => k + 1);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to post comment.");
    } finally {
      setSubmitting(false);
    }
  }, [disabled, html, internal, onSubmit, submitting]);

  const toggleSourceMode = useCallback(
    (nextSource: boolean) => {
      setSourceMode(nextSource);
      // Coming back to rich mode: remount Editor so it picks up the (possibly
      // hand-edited) HTML as its initial value.
      if (!nextSource) setEditorMountKey((k) => k + 1);
    },
    [],
  );

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 1,
        p: internal ? 1.25 : 0,
        borderRadius: internal ? 1 : 0,
        backgroundColor: internal ? "warning.50" : undefined,
        border: internal ? 1 : 0,
        borderColor: internal ? "warning.main" : undefined,
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1,
          flexWrap: "wrap",
        }}
      >
        <Typography
          variant="caption"
          color={internal ? "warning.main" : "text.secondary"}
        >
          {internal
            ? "Internal work note — not visible to the customer."
            : sourceMode
              ? "HTML source — edit raw markup directly."
              : "Reply to this case…"}
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
          <FormControlLabel
            control={
              <Switch
                size="small"
                color="warning"
                checked={internal}
                onChange={(e) => setInternal(e.target.checked)}
                disabled={disabled || submitting}
              />
            }
            label={
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 0.5,
                  fontSize: "0.8rem",
                }}
              >
                <Lock size={14} />
                Internal note
              </Box>
            }
          />
          <FormControlLabel
            control={
              <Switch
                size="small"
                checked={sourceMode}
                onChange={(e) => toggleSourceMode(e.target.checked)}
                disabled={disabled || submitting}
              />
            }
            label={
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 0.5,
                  fontSize: "0.8rem",
                }}
              >
                <Code size={14} />
                HTML source
              </Box>
            }
          />
          <Tooltip
            title={maximized ? "Collapse editor" : "Maximize editor"}
          >
            <IconButton
              size="small"
              onClick={() => setMaximized((m) => !m)}
              aria-label={
                maximized ? "Collapse comment editor" : "Maximize comment editor"
              }
            >
              {maximized ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
            </IconButton>
          </Tooltip>
        </Box>
      </Box>

      {sourceMode ? (
        <TextField
          value={html}
          onChange={(e) => setHtml(e.target.value)}
          placeholder="<p>Type HTML here…</p>"
          multiline
          minRows={maximized ? 18 : 6}
          maxRows={maximized ? 32 : 12}
          disabled={disabled || submitting}
          inputProps={{
            style: {
              fontFamily:
                "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
              fontSize: "0.825rem",
              lineHeight: 1.45,
            },
            spellCheck: false,
          }}
          fullWidth
          variant="outlined"
        />
      ) : (
        <Editor
          key={editorMountKey}
          value={html}
          onChange={setHtml}
          resetTrigger={resetTrigger}
          disabled={disabled || submitting}
          placeholder="Reply to this case…"
          minHeight={maximized ? 480 : 96}
          maxHeight={maximized ? 720 : 260}
          toolbarVariant="full"
          showKeyboardHint
          enterToSubmit={false}
          onSubmitKeyDown={() => {
            void submit();
          }}
        />
      )}

      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1,
        }}
      >
        <Typography
          variant="caption"
          color={error ? "error" : "text.secondary"}
        >
          {error ??
            (sourceMode
              ? "Output is sent as-is. Use this to fix paste-formatting or insert tables."
              : "Ctrl/Cmd + Enter to send.")}
        </Typography>
        <Button
          variant="contained"
          color={internal ? "warning" : "primary"}
          size="small"
          startIcon={internal ? <Lock size={16} /> : <Send size={16} />}
          disabled={disabled || submitting || isEmpty(html)}
          onClick={() => {
            void submit();
          }}
        >
          {submitting
            ? "Sending…"
            : internal
              ? "Save work note"
              : "Send to customer"}
        </Button>
      </Box>
    </Box>
  );
}
