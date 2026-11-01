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
  DialogContent,
  DialogTitle,
  IconButton,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { GripHorizontal, Maximize2, Minimize2 } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useRef,
  useState,
  type JSX,
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from "react";
import Editor from "@components/rich-text-editor/Editor";
import RequiredMark from "@components/RequiredMark";

/** The panel's height before anyone resizes it: it grows with its text from the minimum up to this. */
export const RICH_TEXT_DEFAULT_MAX_HEIGHT = 300;
/** The tallest a panel can be dragged to. */
export const RICH_TEXT_MAX_HEIGHT = 1200;
/** How far one arrow key press resizes the panel. */
export const RICH_TEXT_RESIZE_STEP = 40;

interface ExpandableRichTextFieldProps {
  /** Ids the label, the helper text and the full-screen dialog are derived from. */
  id: string;
  label: string;
  /** The field's current HTML. */
  value: string;
  /**
   * What the inline editor starts from. The editor takes its value once, when
   * it mounts; a form that freezes its seed (the edit form, for dirty
   * tracking) passes it here. Defaults to `value`.
   */
  seedHtml?: string;
  onChange: (html: string) => void;
  disabled?: boolean;
  placeholder?: string;
  /** Shown under the panel and referenced by it (aria-describedby). */
  helperText?: ReactNode;
  /** Marks the field mandatory: the asterisk after the label (as MUI draws it) and
   * `aria-required` on the editor. Display only: the form still decides what to send. */
  required?: boolean;
  minHeight?: number;
  /** Laid out by the caller (e.g. a grid column). */
  sx?: Record<string, unknown>;
}

const clamp = (n: number, min: number, max: number): number => Math.min(max, Math.max(min, n));

/**
 * A rich-text panel of the change request forms that can be made bigger: drag
 * the handle under it (or focus the handle and use the arrow keys; Home goes
 * back to the default size), or open it full screen with Expand. Full screen
 * edits the same field live; when it closes the inline panel reloads from
 * what was written there (the editor reads its value only when it mounts).
 * A disabled field (e.g. an approved change request) still opens full screen,
 * read-only, so a long plan can be reviewed.
 */
export default function ExpandableRichTextField({
  id,
  label,
  value,
  seedHtml,
  onChange,
  disabled = false,
  placeholder,
  helperText,
  required = false,
  minHeight = 100,
  sx,
}: ExpandableRichTextFieldProps): JSX.Element {
  // `null` = the default: grow with the text up to RICH_TEXT_DEFAULT_MAX_HEIGHT.
  const [height, setHeight] = useState<number | null>(null);
  const [inline, setInline] = useState(() => ({ key: 0, seed: seedHtml ?? value }));
  // The text full screen opened with, while it is open.
  const [fullScreen, setFullScreen] = useState<{ openedWith: string } | null>(null);
  // Whether full screen has changed the text since it opened (read synchronously, so a ref).
  const changedInFullScreen = useRef(false);
  const panelRef = useRef<HTMLDivElement>(null);
  const drag = useRef<{ startY: number; startHeight: number } | null>(null);

  const labelId = `${id}-label`;
  const helpId = `${id}-help`;
  const dialogTitleId = `${id}-fullscreen-title`;

  /** The editing area's current height, as the screen shows it. */
  const currentHeight = (): number => {
    const input = panelRef.current?.querySelector<HTMLElement>(".editor-input");
    return input?.getBoundingClientRect().height || height || RICH_TEXT_DEFAULT_MAX_HEIGHT;
  };

  const onPointerDown = (e: ReactPointerEvent<HTMLDivElement>): void => {
    e.preventDefault();
    drag.current = { startY: e.clientY, startHeight: currentHeight() };
    e.currentTarget.setPointerCapture?.(e.pointerId);
  };
  const onPointerMove = (e: ReactPointerEvent<HTMLDivElement>): void => {
    if (!drag.current) return;
    setHeight(clamp(Math.round(drag.current.startHeight + e.clientY - drag.current.startY), minHeight, RICH_TEXT_MAX_HEIGHT));
  };
  const onPointerUp = (e: ReactPointerEvent<HTMLDivElement>): void => {
    drag.current = null;
    e.currentTarget.releasePointerCapture?.(e.pointerId);
  };
  const onHandleKeyDown = (e: ReactKeyboardEvent<HTMLDivElement>): void => {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      const delta = e.key === "ArrowDown" ? RICH_TEXT_RESIZE_STEP : -RICH_TEXT_RESIZE_STEP;
      setHeight(clamp(Math.round((height ?? currentHeight()) + delta), minHeight, RICH_TEXT_MAX_HEIGHT));
    } else if (e.key === "Home") {
      e.preventDefault();
      setHeight(null);
    }
  };

  const openedWith = fullScreen?.openedWith;
  const onFullScreenChange = useCallback(
    (next: string) => {
      // Read-only, or the editor's own first emission (it loading the text): not an edit.
      if (disabled || (!changedInFullScreen.current && next === openedWith)) return;
      changedInFullScreen.current = true;
      onChange(next);
    },
    [disabled, onChange, openedWith],
  );

  const openFullScreen = (): void => {
    changedInFullScreen.current = false;
    setFullScreen({ openedWith: value });
  };
  const closeFullScreen = (): void => {
    // The inline editor reloads only when full screen changed the text.
    if (changedInFullScreen.current) setInline((k) => ({ key: k.key + 1, seed: value }));
    changedInFullScreen.current = false;
    setFullScreen(null);
  };

  const sizeLabel = height === null ? "default size" : `${height} pixels tall`;

  return (
    <Box sx={sx}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 0.5 }}>
        <Typography id={labelId} variant="caption" color="text.secondary" sx={{ display: "block" }}>
          {label}
          {required && <RequiredMark />}
        </Typography>
        <Tooltip title="Expand to full screen">
          <IconButton
            size="small"
            aria-label={`Expand ${label} to full screen`}
            onClick={openFullScreen}
            sx={{ p: 0.25 }}
          >
            <Maximize2 size={14} />
          </IconButton>
        </Tooltip>
      </Box>
      <Box
        ref={panelRef}
        role="group"
        aria-labelledby={labelId}
        aria-describedby={helperText ? helpId : undefined}
      >
        {fullScreen ? (
          <Box
            sx={{
              border: 1,
              borderColor: "divider",
              borderRadius: 1,
              p: 2,
              minHeight,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <Typography variant="body2" color="text.secondary">
              Open in full screen. This panel shows the text again when full screen closes.
            </Typography>
          </Box>
        ) : (
          <Editor
            key={inline.key}
            value={inline.seed}
            onChange={onChange}
            placeholder={placeholder}
            minHeight={height ?? minHeight}
            maxHeight={height ?? RICH_TEXT_DEFAULT_MAX_HEIGHT}
            toolbarVariant="full"
            disabled={disabled}
            ariaRequired={required}
          />
        )}
      </Box>
      {!fullScreen && (
        <Box
          role="separator"
          aria-orientation="horizontal"
          aria-label={`Resize ${label}`}
          aria-valuemin={minHeight}
          aria-valuemax={RICH_TEXT_MAX_HEIGHT}
          aria-valuenow={height ?? undefined}
          aria-valuetext={sizeLabel}
          tabIndex={0}
          title="Drag to resize; arrow keys resize, Home restores the default size, double-click restores it too"
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onPointerCancel={onPointerUp}
          onDoubleClick={() => setHeight(null)}
          onKeyDown={onHandleKeyDown}
          sx={{
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
            height: 12,
            cursor: "ns-resize",
            touchAction: "none",
            color: "text.secondary",
            borderRadius: 1,
            "&:hover, &:focus-visible": { bgcolor: "action.hover", color: "text.primary" },
            "&:focus-visible": { outline: 2, outlineColor: "primary.main", outlineStyle: "solid" },
          }}
        >
          <GripHorizontal size={14} aria-hidden />
        </Box>
      )}
      {helperText && (
        <Typography id={helpId} variant="caption" color="text.secondary" sx={{ display: "block", mt: 0.5 }}>
          {helperText}
        </Typography>
      )}
      <Dialog fullScreen open={!!fullScreen} onClose={closeFullScreen} aria-labelledby={dialogTitleId}>
        {/* An id of its own: MUI otherwise gives the title the dialog's aria-labelledby id, which belongs to the span. */}
        <DialogTitle
          id={`${id}-fullscreen-heading`}
          sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2 }}
        >
          {/* The dialog is named by this text alone, not by the Done button beside it. */}
          <span id={dialogTitleId}>
            {label}
            {disabled && (
              <>
                {" "}
                <Typography component="span" variant="body2" color="text.secondary">
                  (read-only)
                </Typography>
              </>
            )}
          </span>
          <Button variant="contained" size="small" startIcon={<Minimize2 size={14} />} onClick={closeFullScreen}>
            Done
          </Button>
        </DialogTitle>
        <DialogContent dividers>
          {fullScreen && (
            <Box role="group" aria-label={label}>
              <Editor
                value={fullScreen.openedWith}
                onChange={onFullScreenChange}
                placeholder={placeholder}
                minHeight="calc(100vh - 220px)"
                maxHeight="none"
                toolbarVariant="full"
                disabled={disabled}
                autoFocus={!disabled}
              />
            </Box>
          )}
        </DialogContent>
      </Dialog>
    </Box>
  );
}
