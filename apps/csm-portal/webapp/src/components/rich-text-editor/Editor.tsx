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
  htmlToPlainText,
  type ImageUploadHandler,
} from "@components/rich-text-editor/richTextEditor";
import { ALLOWED_IMAGE_MIME_TYPES } from "@components/rich-text-editor/richTextConstants";
import {
  imagePasteRefusalMessage,
  inlineImageRefusal,
  inlineImagesInEditor,
  rememberInlineImageSize,
  type ImagePasteRefusal,
  type InlineImageRoom,
} from "@components/rich-text-editor/inlineImageLimits";
import { looksLikeMarkdown } from "@components/rich-text-editor/looksLikeMarkdown";
import { isFlatClipboardHtml } from "@components/rich-text-editor/isFlatClipboardHtml";
import { markdownToPasteDom, mergeAdjacentParagraphs } from "@components/rich-text-editor/markdownPaste";
import PasteFormatDialog from "@components/rich-text-editor/PasteFormatDialog";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import { type ReactNode, useEffect, useState, useCallback, useMemo, useRef } from "react";
import Toolbar, {
  type ToolbarVariant,
} from "@components/rich-text-editor/ToolBar";
import type { JSX } from "react";
import { RichTextPlugin } from "@lexical/react/LexicalRichTextPlugin";
import { AutoFocusPlugin } from "@lexical/react/LexicalAutoFocusPlugin";
import { ImageNode } from "@components/rich-text-editor/ImageNode";
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
import { $createParagraphNode, $getRoot, type RangeSelection } from "lexical";
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
            stripLexicalTableStylingFromHtml(stripWhitespaceStyleFromHtml(html)),
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
 * Puts a caret that sits on the root itself inside a paragraph. Clicking into an
 * EMPTY editor leaves the selection as an element point on the root (`root`, 0),
 * not on a paragraph, so "is the selection in a plain paragraph" would be false for
 * the first thing typed or pasted. If the root has a block at that offset the caret
 * goes to its start (or the end of the last block); an empty root gets a paragraph.
 * Anything other than a collapsed root caret is left alone. Call it inside a
 * command handler or an update.
 */
function $moveRootCaretIntoBlock(): void {
  const selection = $getSelection();
  if (!$isRangeSelection(selection) || !selection.isCollapsed()) return;
  const root = $getRoot();
  if (selection.anchor.type !== "element" || !selection.anchor.getNode().is(root)) return;
  const offset = selection.anchor.offset;
  const target = root.getChildAtIndex(offset);
  if (target && $isElementNode(target)) {
    target.selectStart();
    return;
  }
  const previous = root.getChildAtIndex(offset - 1);
  if (previous && $isElementNode(previous)) {
    previous.selectEnd();
    return;
  }
  if (root.getChildrenSize() === 0) {
    const paragraph = $createParagraphNode();
    root.append(paragraph);
    paragraph.select();
  }
}

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
        $moveRootCaretIntoBlock();
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
        $moveRootCaretIntoBlock();
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
/** True when `html` holds at least one `<img>` and no text at all. */
function isImageOnlyHtml(html: string): boolean {
  if (!html.trim()) return false;
  const body = new DOMParser().parseFromString(html, "text/html").body;
  return body.querySelector("img") !== null && (body.textContent ?? "").trim() === "";
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
}: {
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
  } | null>(null);

  useEffect(() => {
    return editor.registerCommand(
      PASTE_COMMAND,
      (event) => {
        if (!(event instanceof ClipboardEvent)) return false;
        const clipboardData = event.clipboardData;
        if (!clipboardData) return false;

        const html = clipboardData.getData("text/html");
        const text = clipboardData.getData("text/plain");

        // A picture copied from a web page carries the image itself AND an
        // `<img src="https://...">` as its HTML. The image is stored by
        // ClipboardImagePlugin (or refused with its own message); pasting the
        // HTML as well would add a second, hot-linked copy of the same picture.
        // Only when the HTML is nothing but images: Word/Docs content with
        // text keeps going through the format prompt.
        const hasImageFile = Array.from(clipboardData.items ?? []).some((item) =>
          item.type.startsWith("image/"),
        );
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
          setPendingPaste({ html, text, variant: "markdown" });
          return true;
        }

        if (html.trim()) {
          event.preventDefault();
          setPendingPaste({ html, text, variant: "html" });
          return true;
        }

        if (text) {
          if (lineBreaks) $moveRootCaretIntoBlock();
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
      unwrapNestedPreCodeElements(dom);
      collapseEmptyParagraphElements(dom);
    }

    editor.update(
      () => {
        $moveRootCaretIntoBlock();
        const selection = $getOrCreateRangeSelection();
        // Keep a pasted blank line the same markup as a typed one.
        if (markdown && lineBreaks && $selectionInPlainParagraphs()) mergeAdjacentParagraphs(dom);
        const nodes = $generateNodesFromDOM(editor, dom);
        selection.insertNodes(nodes);
      },
      { tag: PASTE_TAG },
    );
    setPendingPaste(null);
  }, [editor, pendingPaste, lineBreaks]);

  /**
   * "Remove Formatting" -- discards all structure and treats the paste
   * exactly like today's plain-text paste path, tokenizing the clipboard's
   * own `text/plain` payload when present (what a plain-text paste of the
   * same content would have used) and otherwise falling back to a plain-text
   * rendering of the HTML.
   */
  const applyRemoveFormatting = useCallback(() => {
    if (!pendingPaste) return;
    const plainText = pendingPaste.text || htmlToPlainText(pendingPaste.html);

    editor.update(
      () => {
        $moveRootCaretIntoBlock();
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

  const cancelPendingPaste = useCallback(() => setPendingPaste(null), []);

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
      onError: (error: Error) => {
        logger.error("Error occurred in rich text editor", error);
      },
      editable: !disabled,
    }),
    [logger, disabled],
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
          <PasteNormalizationPlugin lineBreaks={lineBreakMode} markdownPaste={markdownPaste} />
          <LinkPlugin />
          <ClickableLinkPlugin />
          <InitialValuePlugin initialHtml={value} />
          <OnChangeHTMLPlugin onChange={onChange} />
          <ResetPlugin resetTrigger={resetTrigger} />
          {lineBreakMode && <EnterLineBreakPlugin />}
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
