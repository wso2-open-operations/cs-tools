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

import { LexicalComposer } from "@lexical/react/LexicalComposer";
import { ContentEditable } from "@lexical/react/LexicalContentEditable";
import { HistoryPlugin } from "@lexical/react/LexicalHistoryPlugin";
import { ListPlugin } from "@lexical/react/LexicalListPlugin";
import { LexicalErrorBoundary } from "@lexical/react/LexicalErrorBoundary";
import { ListItemNode, ListNode } from "@lexical/list";
import {
  Box,
  Paper,
  Typography,
  useTheme,
  Divider,
  IconButton,
  Stack,
  Tooltip,
} from "@wso2/oxygen-ui";
import { Trash, ChevronLeft, ChevronRight } from "@wso2/oxygen-ui-icons-react";
import EditorUploadProgress, { type EditorUpload } from "@components/rich-text-editor/EditorUploadProgress";
import {
  getFileIcon,
  scrollElement,
  INSERT_IMAGE_COMMAND,
  tokenizePlainTextPaste,
  unwrapNestedPreCodeElements,
  collapseEmptyParagraphElements,
  stripWhitespaceStyleFromHtml,
  stripLexicalTableStylingFromHtml,
  markNestedListItemsInHtml,
  htmlToPastePlainText,
  hasMeaningfulHtmlContent,
  htmlHasTextOrTable,
  convertWordListsToHtmlLists,
  preserveEditorWhitespace,
  preserveTrailingLineBreaks,
  type ImageUploadHandler,
} from "@components/rich-text-editor/richTextEditor";
import { ALLOWED_IMAGE_MIME_TYPES } from "@components/rich-text-editor/richTextConstants";
import {
  imagePasteRefusalMessage,
  inlineImageRefusal,
  inlineImagesInEditor,
  rememberInlineImageSize,
  inlineImageSize,
  type ImagePasteRefusal,
  type InlineImageRoom,
} from "@components/rich-text-editor/inlineImageLimits";
import { looksLikeMarkdown } from "@components/rich-text-editor/looksLikeMarkdown";
import { isFlatClipboardHtml } from "@components/rich-text-editor/isFlatClipboardHtml";
import {
  flattenTablesToParagraphs,
  imagesToLinks,
  isAppImageSrc,
  markdownToPasteDom,
  mergeAdjacentParagraphs,
} from "@components/rich-text-editor/markdownPaste";
import { $insertPastedNodes } from "@components/rich-text-editor/insertPastedNodes";
import { $caretOnRootIntoBlock } from "@components/rich-text-editor/splitLineBreakLines";
import PasteFormatDialog from "@components/rich-text-editor/PasteFormatDialog";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import { type ReactNode, useEffect, useState, useCallback, useMemo, useRef } from "react";
import Toolbar, {
  type ToolbarVariant,
} from "@components/rich-text-editor/ToolBar";
import type { JSX } from "react";
import { RichTextPlugin } from "@lexical/react/LexicalRichTextPlugin";
import { AutoFocusPlugin } from "@lexical/react/LexicalAutoFocusPlugin";
import { ImageNode, INLINE_IMAGES_THEME_KEY, markInlineImageEditor } from "@components/rich-text-editor/ImageNode";
import ImagesPlugin from "@components/rich-text-editor/ImagesPlugin";
import { LinkPlugin } from "@lexical/react/LexicalLinkPlugin";
import { ClickableLinkPlugin } from "@lexical/react/LexicalClickableLinkPlugin";
import { LinkNode } from "@lexical/link";
import { HeadingNode, QuoteNode } from "@lexical/rich-text";
import { CodeNode, $isCodeNode } from "@lexical/code";
import { TableNode, TableRowNode, TableCellNode } from "@lexical/table";
import { TablePlugin } from "@lexical/react/LexicalTablePlugin";
import { attachmentLimitMessage, inlineImageMaxBytes } from "@config/attachmentLimitsConfig";
import { useLogger } from "@hooks/useLogger";
import { $generateHtmlFromNodes, $generateNodesFromDOM } from "@lexical/html";
import { $createParagraphNode, $getNodeByKey, $getRoot, $setSelection, $isTextNode, type RangeSelection } from "lexical";
import { OnChangePlugin } from "@lexical/react/LexicalOnChangePlugin";
import {
  KEY_ENTER_COMMAND,
  COMMAND_PRIORITY_HIGH,
  $getSelection,
  $isRangeSelection,
  INSERT_PARAGRAPH_COMMAND,
  INSERT_LINE_BREAK_COMMAND,
  $isParagraphNode,
  $isElementNode,
  PASTE_COMMAND,
  PASTE_TAG,
  $createTabNode,
  KEY_TAB_COMMAND,
  KEY_DOWN_COMMAND,
  INDENT_CONTENT_COMMAND,
  OUTDENT_CONTENT_COMMAND,
  COMMAND_PRIORITY_LOW,
} from "lexical";
import { $isListItemNode } from "@lexical/list";
import { $findMatchingParent, CAN_USE_BEFORE_INPUT, IS_APPLE_WEBKIT, IS_IOS, IS_SAFARI } from "@lexical/utils";
import { $isTableCellNode } from "@lexical/table";

/**
 * Internal component to handle editable state changes.
 */
const EditableStatePlugin = ({ disabled }: { disabled: boolean }) => {
  const [editor] = useLexicalComposerContext();

  useEffect(() => {
    editor.setEditable(!disabled);
  }, [editor, disabled]);

  return null;
};

/**
 * Internal component to handle initial HTML value.
 * Injects when initialHtml is non-empty and editor root is empty (e.g. first paint or value set after mount).
 */
const InitialValuePlugin = ({ initialHtml }: { initialHtml?: string }) => {
  const [editor] = useLexicalComposerContext();
  const appliedRef = useRef(false);

  useEffect(() => {
    if (!initialHtml?.trim() || appliedRef.current) return;
    editor.update(() => {
      const root = $getRoot();
      const parser = new DOMParser();
      const dom = parser.parseFromString(initialHtml, "text/html");
      // Our own export keeps tabs and runs of spaces raw; loading must not collapse them.
      preserveEditorWhitespace(dom);
      // ...and a break that ends a line (`<p>Hi<br></p>`) must not be dropped as the importer's "last child".
      preserveTrailingLineBreaks(dom);
      const nodes = $generateNodesFromDOM(editor, dom);
      root.clear();
      root.append(...nodes);
      appliedRef.current = true;
    });
  }, [editor, initialHtml]);

  return null;
};

/**
 * Internal component to handle changes and export HTML.
 */
const OnChangeHTMLPlugin = ({
  onChange,
}: {
  onChange?: (html: string) => void;
}) => {
  const [editor] = useLexicalComposerContext();

  return (
    <OnChangePlugin
      onChange={(editorState) => {
        editorState.read(() => {
          const html = $generateHtmlFromNodes(editor);
          onChange?.(
            markNestedListItemsInHtml(stripLexicalTableStylingFromHtml(stripWhitespaceStyleFromHtml(html))),
          );
        });
      }}
    />
  );
};

/**
 * Plugin to handle Enter key for submit.
 *
 * When enterToSubmit=false (default): Ctrl+Enter / Cmd+Enter submits; plain Enter stays in editor
 *   (Lexical's default: a new paragraph, or `EnterLineBreakPlugin`'s `<br>` when
 *   `enterInsertsLineBreak` is on -- that plugin never handles a Ctrl/Cmd+Enter, so the
 *   two do not conflict).
 * When enterToSubmit=true: plain Enter submits; Shift+Enter in a list inserts a new list item;
 *   Shift+Enter outside a list passes through for a soft line break; Ctrl/Cmd+Enter is ignored.
 */
const EnterSubmitPlugin = ({
  onSubmit,
  disabled,
  enterToSubmit = false,
}: {
  onSubmit?: () => void;
  disabled?: boolean;
  enterToSubmit?: boolean;
}) => {
  const [editor] = useLexicalComposerContext();

  useEffect(() => {
    if (!onSubmit || disabled) return;

    const unregEnter = editor.registerCommand(
      KEY_ENTER_COMMAND,
      (event: KeyboardEvent | null) => {
        if (event === null) return false;

        if (enterToSubmit) {
          if (event.isComposing) return false;
          if (event.ctrlKey || event.metaKey) return false;

          if (event.shiftKey) {
            const selection = $getSelection();
            if ($isRangeSelection(selection)) {
              const anchorNode = selection.anchor.getNode();
              const listItem = $findMatchingParent(anchorNode, $isListItemNode);
              if (listItem) {
                event.preventDefault();
                editor.dispatchCommand(INSERT_PARAGRAPH_COMMAND, undefined);
                return true;
              }
            }
            return false;
          }

          event.preventDefault();
          onSubmit();
          return true;
        }

        if (!event.ctrlKey && !event.metaKey) return false;
        event.preventDefault?.();
        onSubmit();
        return true;
      },
      COMMAND_PRIORITY_HIGH,
    );

    return unregEnter;
  }, [editor, onSubmit, disabled, enterToSubmit]);

  return null;
};

/**
 * The current range selection; with none (the editor lost its selection before a
 * dialog action ran) the end of the root, adding an empty paragraph to an empty
 * root, so a held paste is never dropped silently.
 */
function $getOrCreateRangeSelection(): RangeSelection {
  const selection = $getSelection();
  if ($isRangeSelection(selection)) return selection;
  const root = $getRoot();
  const last = root.getLastChild();
  if (last && $isElementNode(last)) return last.selectEnd();
  const paragraph = $createParagraphNode();
  root.append(paragraph);
  return paragraph.select();
}

/**
 * True when the whole selection sits in plain paragraphs -- the only place the
 * `enterInsertsLineBreak` mode applies (Enter and plain-text paste). Lists,
 * headings, quotes, code and table cells keep Lexical's own behaviour. Call it
 * inside a command handler or an update.
 */
function $selectionInPlainParagraphs(): boolean {
  const selection = $getSelection();
  if (!$isRangeSelection(selection)) return false;
  return [selection.anchor, selection.focus].every((point) => {
    const block = $findMatchingParent(
      point.getNode(),
      (n) => $isElementNode(n) && !n.isInline(),
    );
    return block !== null && $isParagraphNode(block) && $findMatchingParent(block, $isTableCellNode) === null;
  });
}

/**
 * `enterInsertsLineBreak` mode (GitHub-style new lines): plain Enter inserts a
 * `<br>` inside the current paragraph instead of starting a new one, so two
 * Enters make one blank line. The exported HTML then looks the same in every
 * renderer (a paragraph gap is 6px here, ~0.25em in the customer portal, ~1em
 * in other HTML renderers; a `<br>` is a line everywhere).
 *
 * Only a selection that sits entirely in plain paragraphs is handled (see
 * {@link $selectionInPlainParagraphs}). List items (Enter = new item, Enter on
 * an empty item exits the list), headings, quotes, code blocks and table cells
 * keep Lexical's own Enter behaviour (cells keep it so Lexical's table-edge
 * paragraph handling still works). Shift+Enter already inserts a `<br>`;
 * Ctrl/Cmd+Enter is left to `EnterSubmitPlugin`. Registered at
 * COMMAND_PRIORITY_HIGH, above `@lexical/rich-text`'s default handler, and
 * independent of whether a submit handler is set.
 *
 * The keydown path skips an IME composition (`isComposing`). On Safari/iOS
 * Lexical itself leaves Enter to `beforeinput`, which dispatches
 * INSERT_PARAGRAPH_COMMAND -- also what a soft keyboard's Enter reaches -- so
 * that is handled too. By then Lexical has cleared the composition, so the
 * `isComposing` check there is only a safeguard.
 */
const EnterLineBreakPlugin = () => {
  const [editor] = useLexicalComposerContext();

  useEffect(() => {
    const unregKey = editor.registerCommand(
      KEY_ENTER_COMMAND,
      (event: KeyboardEvent | null) => {
        if (event === null) return false;
        if (event.isComposing || event.shiftKey || event.ctrlKey || event.metaKey || event.altKey) {
          return false;
        }
        // Mirrors @lexical/rich-text: these browsers send Enter as a
        // `beforeinput` instead, which the INSERT_PARAGRAPH_COMMAND handler covers.
        if ((IS_IOS || IS_SAFARI || IS_APPLE_WEBKIT) && CAN_USE_BEFORE_INPUT) return false;
        $caretOnRootIntoBlock();
        if (!$selectionInPlainParagraphs()) return false;
        event.preventDefault();
        editor.dispatchCommand(INSERT_LINE_BREAK_COMMAND, false);
        return true;
      },
      COMMAND_PRIORITY_HIGH,
    );

    const unregParagraph = editor.registerCommand(
      INSERT_PARAGRAPH_COMMAND,
      () => {
        if (editor.isComposing()) return false;
        $caretOnRootIntoBlock();
        if (!$selectionInPlainParagraphs()) return false;
        editor.dispatchCommand(INSERT_LINE_BREAK_COMMAND, false);
        return true;
      },
      COMMAND_PRIORITY_HIGH,
    );

    return () => {
      unregKey();
      unregParagraph();
    };
  }, [editor]);

  return null;
};

/** Records that this editor uses inline images (see `markInlineImageEditor`), before any content renders. */
const InlineImagesPlugin = () => {
  const [editor] = useLexicalComposerContext();
  useState(() => markInlineImageEditor(editor));
  return null;
};

/**
 * The normal editor's Enter with the caret on the root itself (what a click into an
 * empty editor leaves): the new paragraph is added but the caret stayed before it, so
 * the next typed text landed on the FIRST line. Put the caret in a paragraph first,
 * so it behaves like Enter in an empty paragraph. Everything else
 * is left to Lexical's own Enter (the reply box has `EnterLineBreakPlugin`).
 */
const EnterEmptyRootPlugin = ({
  onSubmit,
  enterToSubmit,
}: {
  onSubmit?: () => void;
  enterToSubmit: boolean;
}) => {
  const [editor] = useLexicalComposerContext();

  useEffect(() => {
    // Enter that `EnterSubmitPlugin` is about to turn into a submit must not touch the
    // document first (it would add a paragraph and fire onChange before the submit).
    const willSubmit = (event: KeyboardEvent | null) =>
      !!onSubmit &&
      event !== null &&
      (enterToSubmit ? !event.ctrlKey && !event.metaKey && !event.shiftKey : event.ctrlKey || event.metaKey);
    const unregKey = editor.registerCommand(
      KEY_ENTER_COMMAND,
      (event: KeyboardEvent | null) => {
        if (!willSubmit(event)) $caretOnRootIntoBlock();
        return false;
      },
      COMMAND_PRIORITY_HIGH,
    );
    const unregParagraph = editor.registerCommand(
      INSERT_PARAGRAPH_COMMAND,
      () => {
        $caretOnRootIntoBlock();
        return false;
      },
      COMMAND_PRIORITY_HIGH,
    );
    return () => {
      unregKey();
      unregParagraph();
    };
  }, [editor, onSubmit, enterToSubmit]);

  return null;
};

/**
 * Tab / Shift+Tab inside a list item nest / un-nest it, like the Indent and Outdent
 * buttons, without ever trapping keyboard focus (WCAG 2.1.2):
 * - Tab passes through (focus moves on) when the item cannot be nested: it is the first
 *   item of its list. So repeated Tab nests once, then moves focus.
 * - Shift+Tab passes through on a top-level item, which has nothing to un-nest.
 * - Escape is an escape hatch: the next Tab / Shift+Tab after it is never handled. Any
 *   other key or a click clears it.
 * Outside a list Tab keeps the browser default; a code block keeps its own handling.
 */
const ListTabPlugin = () => {
  const [editor] = useLexicalComposerContext();

  useEffect(() => {
    let escaped = false;
    const clear = () => {
      escaped = false;
    };
    const unregKeyDown = editor.registerCommand(
      KEY_DOWN_COMMAND,
      (event: KeyboardEvent) => {
        if (event.key === "Escape") escaped = true;
        else if (!["Tab", "Shift", "Control", "Alt", "Meta"].includes(event.key)) escaped = false;
        return false;
      },
      COMMAND_PRIORITY_LOW,
    );
    const unregRoot = editor.registerRootListener((root, previous) => {
      previous?.removeEventListener("click", clear);
      root?.addEventListener("click", clear);
    });
    const unregTab = editor.registerCommand(
      KEY_TAB_COMMAND,
      (event: KeyboardEvent) => {
        if (escaped) {
          escaped = false;
          return false;
        }
        if (event.isComposing || event.ctrlKey || event.metaKey || event.altKey) return false;
        const selection = $getSelection();
        if (!$isRangeSelection(selection)) return false;
        if ($findMatchingParent(selection.anchor.getNode(), $isCodeNode) !== null) return false;
        const item = $findMatchingParent(selection.anchor.getNode(), $isListItemNode);
        if (!item) return false;
        if (event.shiftKey ? item.getIndent() === 0 : item.getPreviousSibling() === null) return false;
        event.preventDefault();
        editor.dispatchCommand(event.shiftKey ? OUTDENT_CONTENT_COMMAND : INDENT_CONTENT_COMMAND, undefined);
        return true;
      },
      COMMAND_PRIORITY_LOW,
    );
    return () => {
      unregKeyDown();
      unregRoot();
      unregTab();
    };
  }, [editor]);

  return null;
};

/**
 * Internal component to handle resetting the editor.
 */
const ResetPlugin = ({ resetTrigger }: { resetTrigger?: number }) => {
  const [editor] = useLexicalComposerContext();

  useEffect(() => {
    if (resetTrigger !== undefined && resetTrigger > 0) {
      editor.update(() => {
        const root = $getRoot();
        root.clear();
      });
    }
  }, [editor, resetTrigger]);

  return null;
};

/**
 * Handles paste events containing image data (e.g. Ctrl+C from screen, then Ctrl+V).
 * Inserts it via INSERT_IMAGE_COMMAND: with `onImageUpload` the image is handed to
 * that (it is stored, and a local preview is shown); without one the image is
 * refused, never embedded as base64.
 */
const ClipboardImagePlugin = ({
  onPasteError,
  onImageUpload,
}: {
  onPasteError?: (reason: ImagePasteRefusal) => void;
  onImageUpload?: ImageUploadHandler;
}): null => {
  const [editor] = useLexicalComposerContext();

  // Keep the latest callback in a ref so the paste listener (registered once
  // per editor) always calls the current onPasteError without re-registering.
  const onPasteErrorRef = useRef(onPasteError);
  const onImageUploadRef = useRef(onImageUpload);
  useEffect(() => {
    onPasteErrorRef.current = onPasteError;
    onImageUploadRef.current = onImageUpload;
  }, [onPasteError, onImageUpload]);

  useEffect(() => {
    const handlePaste = (event: ClipboardEvent) => {
      const items = event.clipboardData?.items;
      if (!items) return;

      // Upload mode only: how many more images this draft may take.
      let room: InlineImageRoom | null = null;

      // Excel, LibreOffice Calc and the like put a picture of the copied cells beside the table
      // HTML: the picture only renders the same content, so the HTML is pasted and the picture
      // is not (PasteNormalizationPlugin). A picture alone, or beside HTML that has an <img> of
      // its own (a copied web picture, a chat message with a screenshot), is still inserted.
      if (clipboardImageRendersHtml(event.clipboardData)) return;

      for (const item of Array.from(items)) {
        if (item.type.startsWith("image/")) {
          const file = item.getAsFile();
          if (!file) continue;

          event.preventDefault();

          const upload = onImageUploadRef.current;
          if (!upload) {
            // No place to store the image: never embed it as base64.
            onPasteErrorRef.current?.("disabled");
            break;
          }

          if (
            !ALLOWED_IMAGE_MIME_TYPES.includes(
              file.type as (typeof ALLOWED_IMAGE_MIME_TYPES)[number],
            )
          ) {
            onPasteErrorRef.current?.("type");
            break;
          }

          if (file.size > inlineImageMaxBytes()) {
            onPasteErrorRef.current?.("size");
            break;
          }

          room ??= inlineImagesInEditor(editor);
          const refusal = inlineImageRefusal(room, file.size);
          if (refusal) {
            onPasteErrorRef.current?.(refusal);
            break;
          }
          const src = upload(file);
          rememberInlineImageSize(src, file.size);
          room = { count: room.count + 1, bytes: room.bytes + file.size };
          editor.dispatchCommand(INSERT_IMAGE_COMMAND, {
            src,
            altText: "Pasted Image",
          });
          // Several images may be pasted at once: take as many as fit.
          continue;
        }
      }
    };

    return editor.registerRootListener(
      (
        rootElement: HTMLElement | null,
        prevRootElement: HTMLElement | null,
      ) => {
        prevRootElement?.removeEventListener("paste", handlePaste);
        rootElement?.addEventListener("paste", handlePaste);
      },
    );
  }, [editor]);

  return null;
};

/**
 * Normalizes two paste-import gaps in Lexical's default paste handling
 * before it runs (registered at COMMAND_PRIORITY_HIGH, above the default
 * rich-text paste handler's COMMAND_PRIORITY_EDITOR, so it can intercept
 * first). Falls through (returns false) whenever neither fix applies, so
 * every other paste shape is handled exactly as before.
 *
 * - Plain-text paste (clipboard has text/plain but no text/html -- exactly
 *   ChatGPT's own clipboard format): replays the same tokenization Lexical's
 *   default handler uses, but collapsing blank lines into ordinary paragraph
 *   breaks instead of standalone empty paragraphs. See
 *   `tokenizePlainTextPaste` in richTextEditor.tsx for why the default
 *   causes the reported "extra space between paragraphs".
 * - HTML paste containing `<pre><code>...</code></pre>` (ChatGPT/GitHub/
 *   Notion's standard code block markup): flattens the nested `<code>`
 *   before conversion so it doesn't produce a duplicated, nested CodeNode.
 *   See `unwrapNestedPreCodeElements` in richTextEditor.tsx.
 * - HTML paste containing an empty `<p>`/`<div>` between two real paragraphs
 *   (how Google Docs, Gmail, Gemini, Claude, Notion, Word, and a manual
 *   text-selection copy from ChatGPT's rendered page all represent a blank
 *   line in their clipboard HTML): removes the empty block before
 *   conversion, the same "extra space between paragraphs" fix the plain-text
 *   path gets from `tokenizePlainTextPaste`, generalized to any source that
 *   puts `text/html` on the clipboard. See `collapseEmptyParagraphElements`
 *   in richTextEditor.tsx.
 * - Any HTML paste (clipboard carries a non-empty `text/html`): instead of
 *   inserting immediately, holds the clipboard payload and shows
 *   `PasteFormatDialog`, which lets the user choose between "Keep Formatting"
 *   (the normalized-HTML path above) and "Remove Formatting" (the plain-text
 *   path below, applied to the pasted content). A plain-text-only paste
 *   (clipboard has `text/plain` but no `text/html`) is unaffected and always
 *   goes straight to the plain-text path with no prompt.
 */
/**
 * True when the clipboard's image file is just a rendering of its HTML: the HTML has a table or
 * real text and no `<img>` of its own (Excel/Calc put a png of the range beside the table).
 */
function clipboardImageRendersHtml(clipboardData: DataTransfer | null): boolean {
  if (!clipboardData) return false;
  const hasImageFile = Array.from(clipboardData.items ?? []).some((item) => item.type.startsWith("image/"));
  if (!hasImageFile) return false;
  const html = clipboardData.getData("text/html");
  if (!html.trim() || !htmlHasTextOrTable(html)) return false;
  return new DOMParser().parseFromString(html, "text/html").body.querySelector("img") === null;
}

/** True when `html` holds at least one `<img>` and no text at all. */
function isImageOnlyHtml(html: string): boolean {
  if (!html.trim()) return false;
  const body = new DOMParser().parseFromString(html, "text/html").body;
  return body.querySelector("img") !== null && (body.textContent ?? "").trim() === "";
}

/**
 * Puts a selection captured when a paste was held back into the editor. The dialog
 * takes focus, and the editor may re-resolve its selection from the DOM when focus
 * returns (in Chrome that put the caret at the end of the line), so the held paste is
 * inserted where the caret WAS. Does nothing when the nodes are gone (the caller then
 * falls back to the current selection). Call it inside an update.
 */
function $restoreSelection(saved: RangeSelection | null): void {
  if (!saved) return;
  const selection = saved.clone();
  for (const point of [selection.anchor, selection.focus]) {
    const node = $getNodeByKey(point.key);
    if (!node) return;
    const size = $isTextNode(node) ? node.getTextContentSize() : $isElementNode(node) ? node.getChildrenSize() : 0;
    point.set(point.key, Math.min(point.offset, size), point.type);
  }
  $setSelection(selection);
}

/** True when the selection starts or ends inside a code block (never prompted on paste). */
function $selectionInCodeBlock(): boolean {
  const selection = $getSelection();
  if (!$isRangeSelection(selection)) return false;
  return [selection.anchor, selection.focus].some(
    (point) => $findMatchingParent(point.getNode(), $isCodeNode) !== null,
  );
}

const PasteNormalizationPlugin = ({
  lineBreaks = false,
  markdownPaste = false,
  limitImages = false,
  onPasteError,
}: {
  /** Upload mode: images kept from an in-app copy count against the inline image limits. */
  limitImages?: boolean;
  onPasteError?: (reason: ImagePasteRefusal) => void;
  /**
   * `markdownPaste` mode: pasted text that looks like Markdown gets a "Paste as
   * Markdown" / "Remove Formatting" prompt instead of its normal path.
   */
  markdownPaste?: boolean;
  /** `enterInsertsLineBreak` mode: plain-text newlines become `<br>`s, not paragraphs. */
  lineBreaks?: boolean;
}): JSX.Element | null => {
  const [editor] = useLexicalComposerContext();
  const [pendingPaste, setPendingPaste] = useState<{
    html: string;
    text: string;
    variant: "html" | "markdown";
    /** The clipboard also carried the picture file (stored by ClipboardImagePlugin). */
    hasImageFile: boolean;
    /** The clipboard came from an editor of this app (`application/x-lexical-editor`). */
    fromApp: boolean;
  } | null>(null);
  /** The selection when the paste was held, restored before the dialog's choice is applied. */
  const heldSelection = useRef<RangeSelection | null>(null);

  useEffect(() => {
    const holdSelection = () => {
      const selection = $getSelection();
      heldSelection.current = $isRangeSelection(selection) ? selection.clone() : null;
    };
    return editor.registerCommand(
      PASTE_COMMAND,
      (event) => {
        if (!(event instanceof ClipboardEvent)) return false;
        const clipboardData = event.clipboardData;
        if (!clipboardData) return false;

        const html = clipboardData.getData("text/html");
        const text = clipboardData.getData("text/plain");
        const fromApp = Boolean(clipboardData.getData("application/x-lexical-editor"));

        // A picture copied from a web page carries the image itself AND an
        // `<img src="https://...">` as its HTML. The image is stored by
        // ClipboardImagePlugin (or refused with its own message); pasting the
        // HTML as well would add a second, hot-linked copy of the same picture.
        // Only when the HTML is nothing but images: Word/Docs content with
        // text keeps going through the format prompt.
        // Excel's picture of the range is skipped by ClipboardImagePlugin, so it is not "stored".
        const hasImageFile =
          !clipboardImageRendersHtml(clipboardData) &&
          Array.from(clipboardData.items ?? []).some((item) => item.type.startsWith("image/"));
        if (hasImageFile && isImageOnlyHtml(html)) {
          event.preventDefault();
          return true;
        }

        // Markdown source (VS Code, a chat's copy button) arrives as text/plain,
        // with no HTML or only a flat code-view one. Rendered HTML (a Word list, a
        // Gmail quote) has rendered text/plain that can look like Markdown, so it keeps
        // the Keep/Remove prompt. Never offered inside a code block.
        if (
          markdownPaste &&
          text &&
          looksLikeMarkdown(text) &&
          isFlatClipboardHtml(html) &&
          !$selectionInCodeBlock()
        ) {
          event.preventDefault();
          holdSelection();
          setPendingPaste({ html, text, variant: "markdown", hasImageFile, fromApp });
          return true;
        }

        // Into a code block any paste goes in as code lines (blank lines and indentation kept),
        // never as paragraphs inside the <pre>, and with no prompt (also for Markdown-looking text).
        if ($selectionInCodeBlock()) {
          const codeText = text || (hasMeaningfulHtmlContent(html) ? htmlToPastePlainText(html) : "");
          const selection = $getSelection();
          if (codeText && $isRangeSelection(selection)) {
            event.preventDefault();
            editor.update(
              () => {
                const current = $getSelection();
                if ($isRangeSelection(current)) current.insertRawText(codeText.replace(/\r\n?/g, "\n"));
              },
              { tag: PASTE_TAG },
            );
            return true;
          }
        }

        // HTML with nothing in it (a lone <meta>, empty blocks) has nothing to keep: its
        // text/plain is pasted through the plain path below, with no prompt.
        const htmlHasContent = hasMeaningfulHtmlContent(html);

        if (htmlHasContent) {
          event.preventDefault();
          holdSelection();
          setPendingPaste({ html, text, variant: "html", hasImageFile, fromApp });
          return true;
        }

        if (text) {
          // An empty editor's caret sits on the root: put it in a paragraph first (both editors).
          $caretOnRootIntoBlock();
          const tokens = tokenizePlainTextPaste(text, {
            lineBreaks: lineBreaks && $selectionInPlainParagraphs(),
          });
          if (tokens.length === 0) return false;

          event.preventDefault();
          editor.update(
            () => {
              for (const token of tokens) {
                const currentSelection = $getSelection();
                if (!$isRangeSelection(currentSelection)) continue;

                if (token.type === "paragraph") {
                  currentSelection.insertParagraph();
                } else if (token.type === "linebreak") {
                  currentSelection.insertLineBreak();
                } else if (token.type === "tab") {
                  currentSelection.insertNodes([$createTabNode()]);
                } else {
                  currentSelection.insertText(token.value);
                }
              }
            },
            { tag: PASTE_TAG },
          );
          return true;
        }

        return false;
      },
      COMMAND_PRIORITY_HIGH,
    );
  }, [editor, lineBreaks, markdownPaste]);

  /** "Keep Formatting" -- exactly the normalized-HTML insertion path above. */
  const applyKeepFormatting = useCallback(() => {
    if (!pendingPaste) return;
    // "Paste as Markdown": the pasted text converted, not the clipboard's HTML.
    const markdown = pendingPaste.variant === "markdown";
    const dom = markdown
      ? markdownToPasteDom(pendingPaste.text)
      : new DOMParser().parseFromString(pendingPaste.html, "text/html");
    if (!markdown) {
      convertWordListsToHtmlLists(dom);
      unwrapNestedPreCodeElements(dom);
      collapseEmptyParagraphElements(dom);
      // Never hot-link or embed a foreign image: a link, or nothing when the picture file is stored
      // on its own. An image of this app (copied out of an editor: a `blob:` preview, a stored
      // `/<uuid>`, an older draft's `data:`) stays an image, within the inline image limits.
      let room = limitImages ? inlineImagesInEditor(editor) : null;
      const result: { refused: ImagePasteRefusal | null } = { refused: null };
      imagesToLinks(dom, {
        dropAll: pendingPaste.hasImageFile,
        decide: (img) => {
          const src = img.getAttribute("src") ?? "";
          if (!isAppImageSrc(src, pendingPaste.fromApp)) return undefined;
          if (!room) return "keep";
          const bytes = inlineImageSize(src);
          const refusal = inlineImageRefusal(room, bytes);
          if (refusal) {
            result.refused ??= refusal;
            return "text";
          }
          room = { count: room.count + 1, bytes: room.bytes + bytes };
          return "keep";
        },
      });
      if (result.refused) onPasteError?.(result.refused);
    }
    const saved = heldSelection.current;
    heldSelection.current = null;

    editor.update(
      () => {
        $restoreSelection(saved);
        $caretOnRootIntoBlock();
        const selection = $getOrCreateRangeSelection();
        // Keep a pasted blank line the same markup as a typed one.
        if (lineBreaks && $selectionInPlainParagraphs()) mergeAdjacentParagraphs(dom);
        // A table cannot hold a table: rows of text instead.
        if ($findMatchingParent(selection.anchor.getNode(), $isTableCellNode)) flattenTablesToParagraphs(dom);
        $insertPastedNodes(selection, $generateNodesFromDOM(editor, dom));
      },
      { tag: PASTE_TAG },
    );
    setPendingPaste(null);
  }, [editor, pendingPaste, lineBreaks, limitImages, onPasteError]);

  /**
   * "Remove Formatting" -- discards all structure and treats the paste
   * exactly like today's plain-text paste path, tokenizing the clipboard's
   * own `text/plain` payload when present (what a plain-text paste of the
   * same content would have used) and otherwise falling back to a plain-text
   * rendering of the HTML.
   */
  const applyRemoveFormatting = useCallback(() => {
    if (!pendingPaste) return;
    const plainText = pendingPaste.text || htmlToPastePlainText(pendingPaste.html);
    const saved = heldSelection.current;
    heldSelection.current = null;

    editor.update(
      () => {
        $restoreSelection(saved);
        $caretOnRootIntoBlock();
        $getOrCreateRangeSelection();
        const tokens = tokenizePlainTextPaste(plainText, {
          lineBreaks: lineBreaks && $selectionInPlainParagraphs(),
        });
        for (const token of tokens) {
          const currentSelection = $getSelection();
          if (!$isRangeSelection(currentSelection)) continue;

          if (token.type === "paragraph") {
            currentSelection.insertParagraph();
          } else if (token.type === "linebreak") {
            currentSelection.insertLineBreak();
          } else if (token.type === "tab") {
            currentSelection.insertNodes([$createTabNode()]);
          } else {
            currentSelection.insertText(token.value);
          }
        }
      },
      { tag: PASTE_TAG },
    );
    setPendingPaste(null);
  }, [editor, pendingPaste, lineBreaks]);

  const cancelPendingPaste = useCallback(() => {
    heldSelection.current = null;
    setPendingPaste(null);
  }, []);

  if (!pendingPaste) return null;

  return (
    <PasteFormatDialog
      onKeepFormatting={applyKeepFormatting}
      onRemoveFormatting={applyRemoveFormatting}
      onClose={cancelPendingPaste}
      variant={pendingPaste.variant}
    />
  );
};

/** Static editor config (namespace, nodes, theme). */
const DEFAULT_EDITOR_CONFIG = {
  namespace: "MyEditor",
  nodes: [
    ListNode,
    ListItemNode,
    ImageNode,
    CodeNode,
    LinkNode,
    HeadingNode,
    QuoteNode,
    TableNode,
    TableRowNode,
    TableCellNode,
  ],
  theme: {
    text: {
      bold: "editor-text-bold",
      italic: "editor-text-italic",
      underline: "editor-text-underline",
      strikethrough: "editor-text-strikethrough",
      code: "editor-text-code",
    },
    list: {
      ul: "editor-list-ul",
      ol: "editor-list-ol",
      // Indenting a list item wraps it in a new, textless parent <li> that exists only
      // to hold the nested <ul>/<ol> ($handleIndent in @lexical/list). Lexical tags that
      // wrapper with this class (see $setListItemThemeClassNames) whenever a list item's
      // children include a nested list, so its own bullet/number can be hidden below —
      // otherwise it renders an empty marker next to nothing, above the actually-indented
      // item's marker.
      nested: {
        listitem: "editor-list-item-nested",
      },
    },
    link: "editor-link",
    code: "editor-code",
  },
};

const Editor = ({
  onAttachmentClick,
  attachments = [],
  maxAttachments,
  uploads = [],
  uploadsWaiting = 0,
  onAttachmentRemove,
  disabled = false,
  value,
  onChange,
  resetTrigger,
  minHeight = 150,
  showToolbar = true,
  toolbarVariant = "full",
  onSubmitKeyDown,
  enterToSubmit = false,
  enterInsertsLineBreak = false,
  markdownPaste = false,
  placeholder = "Enter description...",
  id,
  showKeyboardHint = false,
  maxHeight = "300px",
  autoFocus = false,
  onFocus,
  onBlur,
  overlayElement,
  onPasteError,
  onImageUpload,
  ariaRequired = false,
}: {
  onAttachmentClick?: () => void;
  attachments?: File[];
  /** The most attachments the editor takes; the attach button is off once it holds that many. */
  maxAttachments?: number;
  /** Files a sent comment is still uploading; shown inside the editor with their progress. */
  uploads?: EditorUpload[];
  /** Files queued behind the running uploads. */
  uploadsWaiting?: number;
  onAttachmentRemove?: (index: number) => void;
  disabled?: boolean;
  value?: string;
  onChange?: (html: string) => void;
  resetTrigger?: number;
  minHeight?: number | string;
  showToolbar?: boolean;
  toolbarVariant?: ToolbarVariant;
  onSubmitKeyDown?: () => void;
  enterToSubmit?: boolean;
  /**
   * GitHub-style new lines: plain Enter inserts a `<br>` in the current paragraph
   * (two Enters = one blank line) instead of starting a paragraph; toolbar block
   * conversions and plain-text paste follow suit. Off by default; ignored with
   * `enterToSubmit`. See `EnterLineBreakPlugin`.
   */
  enterInsertsLineBreak?: boolean;
  /**
   * Pasted text that looks like Markdown prompts "Paste as Markdown" / "Remove
   * Formatting" (see `looksLikeMarkdown`). Off by default.
   */
  markdownPaste?: boolean;
  placeholder?: string;
  id?: string;
  showKeyboardHint?: boolean;
  maxHeight?: string | number;
  /** Focus the editor on mount (e.g. when a reply composer opens). */
  autoFocus?: boolean;
  onFocus?: () => void;
  onBlur?: () => void;
  /** Optional element rendered as an absolute overlay at the bottom-right inside the editor. */
  overlayElement?: ReactNode;
  /**
   * Called when a pasted image is rejected: `"type"` for an unsupported format
   * (e.g. SVG), `"size"` for exceeding the max paste size, `"count"` when the
   * draft already holds the most images, `"total"` when the draft's images would pass the
   * total size limit (both only with `onImageUpload`).
   */
  onPasteError?: (reason: ImagePasteRefusal) => void;
  /**
   * Stores a pasted or picked image; see {@link ImageUploadHandler}. Leave
   * unset where there is nowhere to store images: image insertion is then off
   * (no toolbar button, a pasted image is refused). Nothing is embedded as base64.
   */
  onImageUpload?: ImageUploadHandler;
  /** Tells assistive tech the field is mandatory (`aria-required` on the editable area). */
  ariaRequired?: boolean;
}): JSX.Element => {
  const oxygenTheme = useTheme();
  const lineBreakMode = enterInsertsLineBreak && !enterToSubmit;
  const logger = useLogger();
  // Without an `onPasteError` handler a refused pasted image is said right
  // below the editor, for a few seconds.
  const [imageNotice, setImageNotice] = useState<string | null>(null);
  useEffect(() => {
    if (!imageNotice) return undefined;
    const timer = setTimeout(() => setImageNotice(null), 8000);
    return () => clearTimeout(timer);
  }, [imageNotice]);
  const onImageRefused = useMemo(
    () =>
      onPasteError ?? ((reason: ImagePasteRefusal) => setImageNotice(imagePasteRefusalMessage(reason))),
    [onPasteError],
  );

  const memoizedEditorConfig = useMemo(
    () => ({
      ...DEFAULT_EDITOR_CONFIG,
      theme: { ...DEFAULT_EDITOR_CONFIG.theme, [INLINE_IMAGES_THEME_KEY]: lineBreakMode },
      onError: (error: Error) => {
        logger.error("Error occurred in rich text editor", error);
      },
      editable: !disabled,
    }),
    [logger, disabled, lineBreakMode],
  );

  const [canScrollLeft, setCanScrollLeft] = useState(false);
  const [canScrollRight, setCanScrollRight] = useState(false);
  const scrollNodeRef = useRef<HTMLDivElement | null>(null);

  const scrollRef = useCallback((node: HTMLDivElement | null) => {
    scrollNodeRef.current = node;
  }, []);

  useEffect(() => {
    const node = scrollNodeRef.current;
    if (!node) return;
    const checkScroll = () => {
      setCanScrollLeft(node.scrollLeft > 0);
      setCanScrollRight(
        node.scrollLeft < node.scrollWidth - node.clientWidth - 1,
      );
    };
    checkScroll();
    node.addEventListener("scroll", checkScroll);
    window.addEventListener("resize", checkScroll);
    return () => {
      node.removeEventListener("scroll", checkScroll);
      window.removeEventListener("resize", checkScroll);
    };
  }, [attachments.length]);

  const scrollAttachments = useCallback((direction: "left" | "right") => {
    scrollElement(scrollNodeRef, direction);
  }, []);

  const editorInputDynamicSx = useMemo(() => {
    const unbounded =
      maxHeight === "none" ||
      maxHeight === "unset" ||
      (typeof maxHeight === "string" &&
        maxHeight.trim().toLowerCase() === "none");
    return {
      maxHeight:
        typeof maxHeight === "number" ? `${maxHeight}px` : maxHeight,
      overflowY: (unbounded ? "visible" : "auto") as "visible" | "auto",
    };
  }, [maxHeight]);

  return (
    <LexicalComposer initialConfig={memoizedEditorConfig}>
      <Paper
        variant="outlined"
        sx={{
          p: 2,
          borderColor: disabled ? "action.disabled" : "divider",
          transition: "border-color 0.2s",
          "&:hover:not(:focus-within)": {
            borderColor: disabled ? "action.disabled" : "text.primary",
          },

          "&:focus-within": {
            borderWidth: "2px",
            borderColor: "primary.main",
          },
        }}
      >
        {lineBreakMode && <InlineImagesPlugin />}
        <EditableStatePlugin disabled={disabled} />
        {showToolbar && (
          <>
            <Toolbar
              onAttachmentClick={onAttachmentClick}
              attachDisabledReason={
                maxAttachments !== undefined && attachments.length >= maxAttachments
                  ? attachmentLimitMessage(maxAttachments)
                  : undefined
              }
              disabled={disabled}
              variant={toolbarVariant}
              showKeyboardHint={showKeyboardHint}
              enterToSubmit={enterToSubmit}
              enterInsertsLineBreak={lineBreakMode}
              imageUploader={onImageUpload}
            />
            <Divider sx={{ my: 1 }} />
          </>
        )}
        <Box
          onFocus={onFocus}
          onBlur={onBlur}
          sx={{
            position: "relative",
            minHeight,
            "& .editor-input": {
              outline: "none",
              minHeight:
                typeof minHeight === "number" ? `${minHeight}px` : minHeight,
              padding: 0,
              ...editorInputDynamicSx,
              ...oxygenTheme.typography.body2,
              color: disabled ? "text.disabled" : "text.primary",
              "& p": {
                margin: 0,
                padding: 0,
              },
              // Mirrors CsmCaseCommentBubble's read-view rule (`"& p + p": { mt: 0.75 }`)
              // so a real paragraph-to-paragraph adjacency shows the same visible gap
              // while composing as it will once posted -- otherwise distinct paragraphs
              // look identical to a soft line break during composition.
              "& p + p": {
                marginTop: 0.75,
              },
              "& > p:only-child > br:only-child": {
                display: "none",
              },
            },
            "& .editor-text-bold": {
              fontWeight: oxygenTheme.typography.fontWeightBold || "bold",
            },
            "& .editor-text-italic": { fontStyle: "italic" },
            "& .editor-text-underline": { textDecoration: "underline" },
            "& .editor-text-strikethrough": {
              textDecoration: "line-through",
            },
            // Mirrors the read-view "& code" rule (CsmCaseCommentBubble.tsx)
            // so inline code looks the same while composing as it will once
            // posted.
            "& .editor-text-code": {
              bgcolor: "background.default",
              px: 0.5,
              borderRadius: 0.5,
              fontFamily: "monospace",
              fontSize: "0.85em",
            },
            "& .editor-list-ul": { ml: 3, listStyleType: "disc" },
            "& .editor-list-ol": { ml: 3, listStyleType: "decimal" },
            "& .editor-list-item-nested": { listStyleType: "none" },
            "& .editor-link": {
              color: "primary.main",
              textDecoration: "underline",
              "&:hover": {
                textDecoration: "none",
              },
            },
            "& .editor-code": {
              backgroundColor: "background.default",
              color: "text.primary",
              fontFamily: "monospace",
              display: "block",
              padding: "8px 16px",
              lineHeight: 1.5,
              margin: "8px 0",
              overflowX: "auto",
              borderRadius: 1,
              border: "1px solid",
              borderColor: "divider",
            },
            // Mirrors CsmCaseCommentBubble's read-view table styling so a
            // table looks the same while composing as it will once posted.
            "& table": {
              display: "block",
              overflowX: "auto",
              maxWidth: "100%",
              width: "max-content",
              minWidth: "100%",
              borderCollapse: "collapse",
              margin: "8px 0",
            },
            // Lexical's table importer stamps every cell with an inline
            // `style="width: 75px"` (its own COLUMN_WIDTH default) whenever
            // the source HTML carries no explicit column widths — without
            // this override every imported table renders as a cramped,
            // heavily-wrapped 75px-per-column grid regardless of actual
            // content, no matter what width this stylesheet rule sets,
            // since an inline style otherwise always wins. `!important` is
            // the only way to reclaim natural (`table-layout: auto`)
            // content-based sizing from it.
            "& th, & td": {
              border: 1,
              borderColor: "divider",
              px: 1,
              py: 0.5,
              textAlign: "left",
              width: "auto !important",
              minWidth: 75,
            },
            "& th": {
              bgcolor: "action.hover",
              fontWeight: 600,
            },
          }}
        >
          <RichTextPlugin
            contentEditable={
              <ContentEditable
                id={id}
                className="editor-input"
                data-testid="case-description-editor"
                ariaRequired={ariaRequired || undefined}
              />
            }
            placeholder={
              <Typography
                variant="body2"
                sx={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  color: "text.secondary",
                  opacity: 0.7,
                  pointerEvents: "none",
                  userSelect: "none",
                  margin: 0,
                  padding: 0,
                  ...oxygenTheme.typography.body2,
                }}
              >
                {placeholder}
              </Typography>
            }
            ErrorBoundary={LexicalErrorBoundary}
          />
          {autoFocus && <AutoFocusPlugin />}
          <HistoryPlugin />
          <ListPlugin />
          <TablePlugin />
          <ImagesPlugin />
          <ClipboardImagePlugin onPasteError={onImageRefused} onImageUpload={onImageUpload} />
          <PasteNormalizationPlugin
            lineBreaks={lineBreakMode}
            markdownPaste={markdownPaste}
            limitImages={Boolean(onImageUpload)}
            onPasteError={onImageRefused}
          />
          <LinkPlugin />
          <ClickableLinkPlugin />
          <InitialValuePlugin initialHtml={value} />
          <OnChangeHTMLPlugin onChange={onChange} />
          <ResetPlugin resetTrigger={resetTrigger} />
          {lineBreakMode ? <EnterLineBreakPlugin /> : <EnterEmptyRootPlugin onSubmit={disabled ? undefined : onSubmitKeyDown} enterToSubmit={enterToSubmit} />}
          <ListTabPlugin />
          <EnterSubmitPlugin onSubmit={onSubmitKeyDown} disabled={disabled} enterToSubmit={enterToSubmit} />
        </Box>
        {imageNotice && (
          <Typography variant="caption" color="error" role="alert" sx={{ display: "block", mt: 0.5 }}>
            {imageNotice}
          </Typography>
        )}
        {overlayElement && (
          <Box
            sx={{
              display: "flex",
              justifyContent: "flex-end",
              mt: 1,
            }}
          >
            {overlayElement}
          </Box>
        )}
        <EditorUploadProgress uploads={uploads} waiting={uploadsWaiting} />
        {attachments.length > 0 && (
          <>
            <Divider sx={{ my: 1.5 }} />
            <Box
              sx={{
                display: "flex",
                flexDirection: "column",
                gap: 1,
              }}
            >
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ fontWeight: 500 }}
              >
                Attachments ({attachments.length})
              </Typography>
              <Box
                sx={{
                  position: "relative",
                  width: "100%",
                  display: "flex",
                  alignItems: "center",
                }}
              >
                {canScrollLeft && (
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      mr: 0.5,
                      flexShrink: 0,
                    }}
                  >
                    <IconButton
                      size="small"
                      onClick={() => scrollAttachments("left")}
                      aria-label="Scroll attachments left"
                      sx={{
                        color: "text.secondary",
                        "&:hover": {
                          color: "primary.main",
                          bgcolor: "action.hover",
                        },
                      }}
                    >
                      <ChevronLeft size={16} />
                    </IconButton>
                  </Box>
                )}

                <Box
                  ref={scrollRef}
                  sx={{
                    overflowX: "auto",
                    overflowY: "hidden",
                    width: "100%",
                    msOverflowStyle: "none",
                    scrollbarWidth: "none",
                    "&::-webkit-scrollbar": {
                      display: "none",
                    },
                  }}
                >
                  <Stack
                    direction="row"
                    spacing={1}
                    sx={{
                      flexWrap: "nowrap",
                      gap: 1,
                      width: "max-content",
                    }}
                  >
                    {attachments.map((file, index) => (
                      <Paper
                        key={`${file.name}-${index}`}
                        sx={{
                          display: "flex",
                          alignItems: "center",
                          gap: 1,
                          p: 0.75,
                          pl: 1,
                          pr: 0.5,
                          transition: "all 0.2s",
                          flexShrink: 0,
                        }}
                      >
                        {getFileIcon(file, oxygenTheme)}
                        <Tooltip title={file.name} placement="top">
                          <Typography
                            variant="caption"
                            sx={{
                              maxWidth: 150,
                              overflow: "hidden",
                              textOverflow: "ellipsis",
                              whiteSpace: "nowrap",
                              fontWeight: 500,
                            }}
                          >
                            {file.name}
                          </Typography>
                        </Tooltip>
                        <IconButton
                          size="small"
                          onClick={() => onAttachmentRemove?.(index)}
                          aria-label={`Remove attachment ${index + 1}`}
                          sx={{
                            p: 0.25,
                            color: "text.secondary",
                            "&:hover": { color: "error.main" },
                          }}
                        >
                          <Trash size={14} />
                        </IconButton>
                      </Paper>
                    ))}
                  </Stack>
                </Box>

                {canScrollRight && (
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      ml: 0.5,
                      flexShrink: 0,
                    }}
                  >
                    <IconButton
                      size="small"
                      onClick={() => scrollAttachments("right")}
                      aria-label="Scroll attachments right"
                      sx={{
                        color: "text.secondary",
                        "&:hover": {
                          color: "primary.main",
                          bgcolor: "action.hover",
                        },
                      }}
                    >
                      <ChevronRight size={16} />
                    </IconButton>
                  </Box>
                )}
              </Box>
            </Box>
          </>
        )}
      </Paper>
    </LexicalComposer>
  );
};

export default Editor;
