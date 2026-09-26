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

import { Box, FormControlLabel, Switch, TextField, Typography } from "@wso2/oxygen-ui";
import { Code } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useState, type JSX } from "react";
import Editor from "@components/rich-text-editor/Editor";
import type { ToolbarVariant } from "@components/rich-text-editor/ToolBar";

/**
 * Adds the same "HTML source" switch `CsmCaseCommentInput.tsx` offers on a
 * case reply (toggling between the rich `Editor` and a plain monospace
 * textarea holding the exact same underlying HTML string) to any other field
 * that needs it — first requested for the two announcement description
 * fields, so an engineer can paste/hand-edit raw markup the rich toolbar
 * can't produce (e.g. a `<table>`), matching the editing experience already
 * available elsewhere in the portal (and, before this app, in the ServiceNow
 * flow this replaces).
 *
 * Toggling back to rich mode remounts `Editor` (keyed on a counter) so it
 * re-parses whatever HTML was just hand-edited as its new initial value —
 * `Editor` only applies `value` as an *initial* value on mount (see its own
 * `InitialValuePlugin`), not on every change, so without the remount the
 * rich view would keep showing whatever it last rendered instead of the
 * edited source.
 */
export default function EditorWithSourceToggle({
  value,
  onChange,
  placeholder,
  minHeight = 150,
  maxHeight = 300,
  toolbarVariant = "full",
  disabled = false,
  id,
}: {
  value: string;
  onChange: (html: string) => void;
  placeholder?: string;
  minHeight?: number | string;
  maxHeight?: number | string;
  toolbarVariant?: ToolbarVariant;
  disabled?: boolean;
  id?: string;
}): JSX.Element {
  const [sourceMode, setSourceMode] = useState(false);
  const [editorMountKey, setEditorMountKey] = useState(0);

  const toggleSourceMode = useCallback((next: boolean) => {
    setSourceMode(next);
    if (!next) setEditorMountKey((k) => k + 1);
  }, []);

  return (
    <Box>
      <Box sx={{ display: "flex", justifyContent: "flex-end", mb: 0.5 }}>
        <FormControlLabel
          control={
            <Switch
              size="small"
              checked={sourceMode}
              onChange={(e) => toggleSourceMode(e.target.checked)}
              disabled={disabled}
            />
          }
          label={
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, fontSize: "0.8rem" }}>
              <Code size={14} />
              HTML source
            </Box>
          }
        />
      </Box>

      {sourceMode ? (
        <TextField
          id={id}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder="<p>Type HTML here…</p>"
          multiline
          minRows={6}
          maxRows={12}
          disabled={disabled}
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
          id={id}
          value={value}
          onChange={onChange}
          placeholder={placeholder}
          minHeight={minHeight}
          maxHeight={maxHeight}
          toolbarVariant={toolbarVariant}
          disabled={disabled}
        />
      )}
      {sourceMode && (
        <Typography variant="caption" color="text.secondary" sx={{ display: "block", mt: 0.5 }}>
          Output is sent as-is. Use this to fix paste-formatting or insert tables.
        </Typography>
      )}
    </Box>
  );
}
