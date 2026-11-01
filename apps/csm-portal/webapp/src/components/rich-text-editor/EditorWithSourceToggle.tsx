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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Switch,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Code } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useState, type JSX } from "react";
import Editor from "@components/rich-text-editor/Editor";
import HtmlSourcePreview from "@components/rich-text-editor/HtmlSourcePreview";
import { htmlNeedsSourceEditing } from "@components/rich-text-editor/richHtmlFidelity";
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
 *
 * With `keepStyledHtmlAsSource` set, content the rich editor cannot represent
 * (a bordered box, coloured links, anything with inline styling it has no node
 * for -- see `htmlNeedsSourceEditing`) opens in source mode and stays there:
 * loading it into the rich editor would rewrite it to a plainer version and
 * report that as an edit. The editor is only mounted for such content if the
 * user explicitly asks for it, after a warning. It is opt-in so that every
 * other caller keeps exactly the behaviour it always had; the announcement
 * screens turn it on because an announcement's styled layout is what customers
 * see.
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
  keepStyledHtmlAsSource = false,
  ariaRequired = false,
}: {
  value: string;
  onChange: (html: string) => void;
  placeholder?: string;
  minHeight?: number | string;
  maxHeight?: number | string;
  toolbarVariant?: ToolbarVariant;
  disabled?: boolean;
  id?: string;
  /** Keep HTML the rich editor would flatten in source mode, and warn before switching away. */
  keepStyledHtmlAsSource?: boolean;
  /** Tells assistive tech the field is mandatory, in either mode (`aria-required`
   * on the rich editor's editable area or on the source box). The caller draws
   * the visible asterisk on its own caption. */
  ariaRequired?: boolean;
}): JSX.Element {
  // null until the user flips the switch themselves; after that their choice wins.
  const [userSourceChoice, setUserSourceChoice] = useState<boolean | null>(null);
  // Latched: true once the value has needed source mode, and never cleared by
  // the value changing again, so deleting the last styled element while typing
  // in the source box does not yank the user into the rich editor mid-edit.
  // Set during render (not in an effect) so the rich editor is never mounted
  // with the value that arrives after the first paint, which is exactly how the
  // request dialog supplies it.
  const losesStyling = useCallback(
    (html: string): boolean => keepStyledHtmlAsSource && htmlNeedsSourceEditing(html),
    [keepStyledHtmlAsSource],
  );
  const [needsSource, setNeedsSource] = useState(() => losesStyling(value));
  if (!needsSource && userSourceChoice === null && losesStyling(value)) {
    setNeedsSource(true);
  }
  const sourceMode = userSourceChoice ?? needsSource;
  const [editorMountKey, setEditorMountKey] = useState(0);
  const [confirmRichOpen, setConfirmRichOpen] = useState(false);

  const switchToRich = useCallback(() => {
    setUserSourceChoice(false);
    setEditorMountKey((k) => k + 1);
  }, []);

  const toggleSourceMode = useCallback(
    (next: boolean) => {
      if (next) {
        setUserSourceChoice(true);
        return;
      }
      // Going back to the rich view re-parses the HTML, which simplifies it.
      // Say so before doing it to content that has styling to lose.
      if (losesStyling(value)) {
        setConfirmRichOpen(true);
        return;
      }
      switchToRich();
    },
    [value, switchToRich, losesStyling],
  );

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
            "aria-required": ariaRequired || undefined,
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
          ariaRequired={ariaRequired}
        />
      )}
      {sourceMode && (
        <Typography variant="caption" color="text.secondary" sx={{ display: "block", mt: 0.5 }}>
          Output is sent as-is. Use this to fix paste-formatting or insert tables.
          {needsSource && userSourceChoice === null &&
            " This content has styling the rich editor can't keep (such as boxes or coloured links), so it is shown as HTML source to preserve it exactly. The preview shows how it will look."}
        </Typography>
      )}
      {sourceMode && keepStyledHtmlAsSource && (
        <HtmlSourcePreview html={value} onChange={disabled ? undefined : onChange} />
      )}

      <Dialog open={confirmRichOpen} onClose={() => setConfirmRichOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>Switch to the rich editor?</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary">
            This content uses styling the rich editor can't keep, such as boxes, coloured links
            and nested layout. Switching will simplify it and the original styling will be lost.
            Staying in HTML source keeps it exactly as written, and the preview below the source box shows how it will look.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmRichOpen(false)} autoFocus>
            Keep HTML source
          </Button>
          <Button
            color="warning"
            onClick={() => {
              setConfirmRichOpen(false);
              switchToRich();
            }}
          >
            Switch and simplify
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
