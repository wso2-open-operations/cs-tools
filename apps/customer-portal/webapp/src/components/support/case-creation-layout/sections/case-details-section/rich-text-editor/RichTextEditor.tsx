// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { alpha, Box, Divider, Paper } from "@wso2/oxygen-ui";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type JSX,
  type KeyboardEvent,
} from "react";
import {
  RICH_TEXT_BLOCK_TAGS,
  RICH_TEXT_HISTORY_LIMIT,
  RICH_TEXT_UNDO_DEBOUNCE_MS,
} from "@constants/supportConstants";
import {
  createCodeBlockHtml,
  htmlToMarkdown,
  insertHtmlAtCursor,
  markdownToHtml,
  queryActiveFormats,
  setBlockFormat,
  setIndentation,
  setTextAlignment,
  toggleInlineFormat,
  toggleList,
} from "@utils/richTextEditor";
import { useLogger } from "@hooks/useLogger";
import { AttachmentsListDisplay } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/attachments/AttachmentsListDisplay";
import { HistoryControls } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/toolbar/HistoryControls";
import { BlockFormatControl } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/toolbar/BlockFormatControl";
import { TextFormattingControls } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/toolbar/TextFormattingControls";
import { AlignmentControls } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/toolbar/AlignmentControls";
import { ListControls } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/toolbar/ListControls";
import { InsertControls } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/toolbar/InsertControls";
import { MarkdownControl } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/toolbar/MarkdownControl";
import { EditorContentArea } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/editor/EditorContentArea";
import { LinkInsertPopover } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/link-insertion/LinkInsertPopover";
import { MarkdownEditorDialog } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/markdown-editor/MarkdownEditorDialog";
import { CodeBlockInsertDialog } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/code-block/CodeBlockInsertDialog";

export interface RichTextEditorProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  minHeight?: number;
  storageKey?: string;
  [key: string]: any;
}

/**
 * Rich text editor with toolbar, undo/redo, active format indicators,
 * inline code block, persistence, image/attachment UI, and markdown editing.
 *
 * @param {RichTextEditorProps} props - Component props.
 * @returns {JSX.Element} The rendered rich text editor.
 */
export function RichTextEditor({
  value,
  onChange,
  placeholder = "Describe the issue in detail",
  disabled = false,
  minHeight = 160,
  storageKey,
  ...restProps
}: RichTextEditorProps): JSX.Element {
  const logger = useLogger();
  const editorRef = useRef<HTMLDivElement>(null);
  const internalChangeCountRef = useRef(0);
  const undoDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const hasRestoredRef = useRef(false);
  const savedRangeRef = useRef<Range | null>(null);

  const [history, setHistory] = useState<string[]>([value || ""]);
  const [historyIndex, setHistoryIndex] = useState(0);
  const historyIndexRef = useRef(0);

  const updateHistoryIndex = useCallback((index: number) => {
    setHistoryIndex(index);
    historyIndexRef.current = index;
  }, []);
  const [linkPopup, setLinkPopup] = useState<{
    url: string;
    text: string;
    anchor: HTMLElement | null;
  } | null>(null);
  const [markdownModalOpen, setMarkdownModalOpen] = useState(false);
  const [codeBlockDialogOpen, setCodeBlockDialogOpen] = useState(false);
  const [markdownEditContent, setMarkdownEditContent] = useState("");
  const [activeFormat, setActiveFormat] = useState({
    bold: false,
    italic: false,
    underline: false,
  });
  const [currentBlockTag, setCurrentBlockTag] = useState<string>("p");
  const [otherAttachments, setOtherAttachments] = useState<
    Array<{
      id: string;
      name: string;
      type: "image" | "file";
      dataUrl?: string;
    }>
  >([]);

  const updateActiveFormat = useCallback(() => {
    try {
      const formats = queryActiveFormats();
      setActiveFormat({
        bold: formats.bold,
        italic: formats.italic,
        underline: formats.underline,
      });
      setCurrentBlockTag(formats.block);
    } catch (error) {
      logger.warn("Failed to update active format:", error);
    }
  }, [logger]);

  const saveSelection = useCallback(() => {
    const sel = window.getSelection();
    if (
      sel &&
      sel.rangeCount > 0 &&
      editorRef.current?.contains(sel.anchorNode)
    ) {
      savedRangeRef.current = sel.getRangeAt(0).cloneRange();
    } else {
      savedRangeRef.current = null;
    }
  }, []);

  const pushHistory = useCallback(
    (nextHtml: string) => {
      setHistory((prev) => {
        const trimmed = prev.slice(0, historyIndexRef.current + 1);
        if (trimmed[trimmed.length - 1] === nextHtml) return prev;
        const next = [...trimmed, nextHtml].slice(-RICH_TEXT_HISTORY_LIMIT);
        updateHistoryIndex(next.length - 1);
        return next;
      });
    },
    [updateHistoryIndex],
  );

  const syncFromValue = useCallback(() => {
    if (!editorRef.current || internalChangeCountRef.current > 0) return;
    const html = value ?? "";
    if (editorRef.current.innerHTML !== html) {
      editorRef.current.innerHTML = html;
    }
  }, [value]);

  useEffect(() => {
    syncFromValue();
  }, [value, syncFromValue]);

  useEffect(() => {
    if (!storageKey || hasRestoredRef.current) return;
    hasRestoredRef.current = true;
    try {
      const stored = sessionStorage.getItem(storageKey);
      if (stored && stored.length > 0) {
        onChange(stored);
        if (editorRef.current) {
          editorRef.current.innerHTML = stored;
        }
        setTimeout(() => {
          setHistory([stored]);
          updateHistoryIndex(0);
        }, 0);
      }
    } catch (error) {
      logger.warn("Failed to restore from sessionStorage:", error);
    }
  }, [storageKey, onChange, logger]);

  const saveToStorage = useCallback(
    (html: string) => {
      if (storageKey) {
        try {
          sessionStorage.setItem(storageKey, html);
        } catch (error) {
          logger.warn("Failed to save to sessionStorage:", error);
        }
      }
    },
    [storageKey, logger],
  );

  const emitChange = useCallback(
    (html: string, pushToHistory = true) => {
      internalChangeCountRef.current += 1;
      onChange(html);
      saveToStorage(html);
      if (pushToHistory) {
        if (undoDebounceRef.current) {
          clearTimeout(undoDebounceRef.current);
          undoDebounceRef.current = null;
        }
        undoDebounceRef.current = setTimeout(() => {
          undoDebounceRef.current = null;
          pushHistory(html);
        }, RICH_TEXT_UNDO_DEBOUNCE_MS);
      }
      setTimeout(() => {
        internalChangeCountRef.current -= 1;
      }, 0);
    },
    [onChange, saveToStorage, pushHistory],
  );

  const handleMarkdownApply = useCallback(
    (html: string) => {
      emitChange(html, true);
      if (editorRef.current) editorRef.current.innerHTML = html;
    },
    [emitChange],
  );

  const handleInput = useCallback(() => {
    if (!editorRef.current || disabled) return;
    const html = editorRef.current.innerHTML;
    internalChangeCountRef.current += 1;
    onChange(html);
    saveToStorage(html);
    if (undoDebounceRef.current) clearTimeout(undoDebounceRef.current);
    undoDebounceRef.current = setTimeout(() => {
      undoDebounceRef.current = null;
      pushHistory(html);
    }, RICH_TEXT_UNDO_DEBOUNCE_MS);
    internalChangeCountRef.current -= 1;
    updateActiveFormat();
  }, [disabled, onChange, saveToStorage, pushHistory, updateActiveFormat]);

  const handleUndo = useCallback(() => {
    if (historyIndex <= 0) return;
    if (undoDebounceRef.current) {
      clearTimeout(undoDebounceRef.current);
      undoDebounceRef.current = null;
    }
    const nextIndex = historyIndex - 1;
    const html = history[nextIndex];
    updateHistoryIndex(nextIndex);
    if (editorRef.current) {
      editorRef.current.innerHTML = html;
      internalChangeCountRef.current += 1;
      onChange(html);
      saveToStorage(html);
      internalChangeCountRef.current -= 1;
    }
  }, [history, historyIndex, onChange, saveToStorage, updateHistoryIndex]);

  const handleRedo = useCallback(() => {
    if (historyIndex >= history.length - 1) return;
    const nextIndex = historyIndex + 1;
    const html = history[nextIndex];
    updateHistoryIndex(nextIndex);
    if (editorRef.current) {
      editorRef.current.innerHTML = html;
      internalChangeCountRef.current += 1;
      onChange(html);
      saveToStorage(html);
      internalChangeCountRef.current -= 1;
    }
  }, [history, historyIndex, onChange, saveToStorage, updateHistoryIndex]);

  const exec = useCallback(
    (cmd: string) => {
      const commands: Record<string, () => void> = {
        bold: () => toggleInlineFormat("strong"),
        italic: () => toggleInlineFormat("em"),
        underline: () => toggleInlineFormat("u"),
        strikeThrough: () => toggleInlineFormat("strike"),
        justifyLeft: () => setTextAlignment("left"),
        justifyCenter: () => setTextAlignment("center"),
        justifyRight: () => setTextAlignment("right"),
        justifyFull: () => setTextAlignment("justify"),
        indent: () => setIndentation("indent"),
        outdent: () => setIndentation("outdent"),
        insertUnorderedList: () => toggleList("ul"),
        insertOrderedList: () => toggleList("ol"),
      };

      const handler = commands[cmd];
      if (handler) {
        handler();
        if (editorRef.current) {
          emitChange(editorRef.current.innerHTML);
        }
        updateActiveFormat();
      } else {
        logger.warn(`Unsupported command: ${cmd}`);
      }
    },
    [emitChange, updateActiveFormat, logger],
  );

  const handleBlock = useCallback(
    (tag: string) => {
      const isHeading = /^h[1-6]$/i.test(tag);
      if (isHeading) {
        setBlockFormat(tag);
        setCurrentBlockTag(tag.toLowerCase());
      } else {
        const classMap: Record<string, string> = {
          subtitle1: "subtitle1",
          subtitle2: "subtitle2",
          body1: "body1",
          body2: "body2",
          caption: "caption",
        };

        const className = classMap[tag];
        if (className) {
          setBlockFormat("p");
          const sel = window.getSelection();
          const node = sel?.anchorNode;
          if (node) {
            const parent =
              node.nodeType === Node.TEXT_NODE
                ? node.parentElement
                : (node as Element);
            const pElement = parent?.closest?.("p") || parent;
            if (pElement && pElement.nodeName === "P") {
              Object.values(classMap).forEach((cls) => {
                pElement.classList.remove(cls);
              });
              pElement.classList.add(className);
            }
          }
          setCurrentBlockTag(tag);
        }
      }

      if (editorRef.current) {
        emitChange(editorRef.current.innerHTML);
      }
    },
    [emitChange],
  );

  const handleInsertLink = useCallback(
    (url: string, text: string) => {
      if (!url || !editorRef.current) {
        setLinkPopup(null);
        return;
      }

      // Validate URL scheme to prevent XSS
      const trimmedUrl = url.trim();
      const lowerUrl = trimmedUrl.toLowerCase();
      const isSafeScheme =
        lowerUrl.startsWith("http://") ||
        lowerUrl.startsWith("https://") ||
        lowerUrl.startsWith("mailto:") ||
        lowerUrl.startsWith("tel:") ||
        trimmedUrl.startsWith("/") ||
        trimmedUrl.startsWith("#") ||
        trimmedUrl.startsWith("?");

      if (!isSafeScheme) {
        logger.warn(`Blocked potentially unsafe URL: ${url}`);
        setLinkPopup(null);
        return;
      }

      const displayText = text || url;
      const safeUrl = trimmedUrl.replace(/"/g, "&quot;");
      const safeText = displayText.replace(/</g, "&lt;").replace(/>/g, "&gt;");
      const linkHtml = `<a href="${safeUrl}" target="_blank" rel="noopener">${safeText}</a>`;
      const savedRange = savedRangeRef.current;
      insertHtmlAtCursor(
        editorRef.current as HTMLElement,
        linkHtml,
        savedRange,
        logger,
      );
      savedRangeRef.current = null;
      emitChange(editorRef.current.innerHTML);
      setLinkPopup(null);
    },
    [emitChange, logger],
  );

  const handleInsertCodeBlock = useCallback(
    (code: string) => {
      if (!code.trim() || !editorRef.current) {
        setCodeBlockDialogOpen(false);
        return;
      }

      const html = createCodeBlockHtml(code);
      insertHtmlAtCursor(
        editorRef.current as HTMLElement,
        html,
        savedRangeRef.current,
        logger,
      );
      savedRangeRef.current = null;
      emitChange(editorRef.current.innerHTML);
      setCodeBlockDialogOpen(false);
    },
    [emitChange, logger],
  );

  const handleImageSelect = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (!file || !file.type.startsWith("image/")) return;
      const reader = new FileReader();
      reader.onload = () => {
        if (!editorRef.current) return;

        const dataUrl = reader.result as string;
        const id = `img-${Date.now()}-${Math.random().toString(36).slice(2)}`;
        const safeSrc = dataUrl.replace(/"/g, "&quot;");
        const wrapperHtml = `<span class="rich-text-image-wrap" contenteditable="false" data-image-id="${id}"><img src="${safeSrc}" alt="upload" class="rich-text-inline-img"/><button type="button" class="rich-text-image-delete" data-delete-id="${id}" aria-label="Remove image">×</button></span>`;
        insertHtmlAtCursor(
          editorRef.current as HTMLElement,
          wrapperHtml,
          savedRangeRef.current,
          logger,
        );
        savedRangeRef.current = null;
        if (editorRef.current) {
          emitChange(editorRef.current.innerHTML);
        }
      };
      reader.readAsDataURL(file);
      e.target.value = "";
    },
    [emitChange],
  );

  const handleOtherFileSelect = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (!file) return;
      const id = `file-${Date.now()}-${Math.random().toString(36).slice(2)}`;
      const isImage = file.type.startsWith("image/");
      if (isImage) {
        const reader = new FileReader();
        reader.onload = () => {
          setOtherAttachments((prev) => [
            ...prev,
            {
              id,
              name: file.name,
              type: "image" as const,
              dataUrl: reader.result as string,
            },
          ]);
        };
        reader.readAsDataURL(file);
      } else {
        setOtherAttachments((prev) => [
          ...prev,
          { id, name: file.name, type: "file" as const },
        ]);
      }
      e.target.value = "";
    },
    [],
  );

  const removeOtherAttachment = useCallback((id: string) => {
    setOtherAttachments((prev) => prev.filter((a) => a.id !== id));
  }, []);

  const handleEditorClick = useCallback(
    (e: React.MouseEvent<HTMLDivElement>) => {
      const target = e.target as HTMLElement;
      if (
        target.classList.contains("rich-text-image-delete") ||
        target.closest(".rich-text-image-delete")
      ) {
        e.preventDefault();
        const btn = target.classList.contains("rich-text-image-delete")
          ? target
          : target.closest(".rich-text-image-delete");
        const id = (btn as HTMLElement)?.getAttribute("data-delete-id");
        if (id && editorRef.current) {
          const safeId = CSS.escape(id);
          const wrap = editorRef.current.querySelector(
            `[data-image-id="${safeId}"]`,
          );
          wrap?.remove();
          emitChange(editorRef.current.innerHTML);
        }
      }
    },
    [emitChange],
  );

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLDivElement>) => {
      const el = editorRef.current;
      const sel = window.getSelection();
      const node = sel?.anchorNode;
      const inCodeBlock =
        el &&
        node &&
        el.contains(node) &&
        (node.nodeType === Node.ELEMENT_NODE
          ? (node as HTMLElement).closest?.("pre.inline-code-block, pre code")
          : node.parentElement?.closest?.("pre.inline-code-block, pre code"));
      if (e.key === "Enter" && inCodeBlock) {
        e.preventDefault();
        const sel = window.getSelection();
        if (sel && sel.rangeCount > 0) {
          insertHtmlAtCursor(editorRef.current as HTMLElement, "<br>");
        }
        if (el) emitChange(el.innerHTML);
        return;
      }
      if (e.key === "Tab") {
        e.preventDefault();
        if (e.shiftKey) {
          exec("outdent");
        } else {
          exec("indent");
        }
      }
    },
    [exec, emitChange],
  );

  const handlePaste = useCallback(
    (e: React.ClipboardEvent<HTMLDivElement>) => {
      const text = e.clipboardData.getData("text/plain");
      if (!text || !editorRef.current) return;
      const trimmed = text.trim();
      const looksLikeMarkdown =
        /^#\s/m.test(trimmed) ||
        /\*\*[^*]+\*\*/.test(trimmed) ||
        /^[-*]\s/m.test(trimmed) ||
        /^\d+\.\s/m.test(trimmed) ||
        /\[[^\]]+\]\([^)]+\)/.test(trimmed) ||
        /^```/m.test(trimmed) ||
        /^---\s*$/m.test(trimmed);
      if (!looksLikeMarkdown) return;
      e.preventDefault();
      const html = markdownToHtml(text) || "";
      insertHtmlAtCursor(editorRef.current as HTMLElement, html);
      emitChange(editorRef.current.innerHTML);
    },
    [emitChange],
  );

  const openLinkPopup = useCallback(
    (anchorEl: HTMLElement) => {
      saveSelection();
      const sel = window.getSelection();
      const node =
        sel?.anchorNode && sel.anchorNode.nodeType === Node.ELEMENT_NODE
          ? (sel.anchorNode as HTMLElement)
          : sel?.anchorNode?.parentElement;
      const link = node?.closest?.("a");
      const url = link?.getAttribute("href") ?? "";
      const text = link?.textContent ?? "";
      setLinkPopup({ url, text, anchor: anchorEl });
    },
    [saveSelection],
  );

  useEffect(() => {
    const el = editorRef.current;
    if (!el) return;

    const onSelectionChange = () => {
      // Only update if the selection is within our editor
      if (editorRef.current?.contains(document.activeElement)) {
        updateActiveFormat();
      }
    };

    document.addEventListener("selectionchange", onSelectionChange);
    el.addEventListener("mouseup", updateActiveFormat);
    el.addEventListener("keyup", updateActiveFormat);

    return () => {
      document.removeEventListener("selectionchange", onSelectionChange);
      el.removeEventListener("mouseup", updateActiveFormat);
      el.removeEventListener("keyup", updateActiveFormat);
    };
  }, [updateActiveFormat]);

  useEffect(() => {
    return () => {
      if (undoDebounceRef.current) clearTimeout(undoDebounceRef.current);
    };
  }, []);

  const canUndo = historyIndex > 0;
  const canRedo = historyIndex < history.length - 1;

  return (
    <Paper
      variant="outlined"
      {...restProps}
      sx={{
        overflow: "hidden",
        borderColor: (theme) =>
          disabled
            ? theme.palette.mode === "light"
              ? alpha(theme.palette.text.disabled, 0.21)
              : alpha(theme.palette.common.white, 0.5)
            : "divider",
      }}
    >
      <Box
        sx={{
          display: "flex",
          flexWrap: "nowrap",
          alignItems: "center",
          gap: 0,
          p: 0.75,
          bgcolor: "background.default",
          overflowX: "auto",
          minHeight: 40,
        }}
      >
        <HistoryControls
          canUndo={canUndo}
          canRedo={canRedo}
          onUndo={handleUndo}
          onRedo={handleRedo}
          disabled={disabled}
        />
        <Divider orientation="vertical" flexItem sx={{ mx: 0.5 }} />
        <BlockFormatControl
          currentTag={currentBlockTag}
          tags={RICH_TEXT_BLOCK_TAGS}
          onChange={handleBlock}
          disabled={disabled}
        />
        <Divider orientation="vertical" flexItem sx={{ mx: 0.5 }} />
        <TextFormattingControls
          activeFormats={activeFormat}
          onFormat={exec}
          disabled={disabled}
        />
        <Divider orientation="vertical" flexItem sx={{ mx: 0.5 }} />
        <AlignmentControls onAlign={exec} disabled={disabled} />
        <Divider orientation="vertical" flexItem sx={{ mx: 0.5 }} />
        <ListControls onCommand={exec} disabled={disabled} />
        <Divider orientation="vertical" flexItem sx={{ mx: 0.5 }} />
        <InsertControls
          onInsertLink={openLinkPopup}
          onOpenCodeDialog={() => {
            saveSelection();
            setCodeBlockDialogOpen(true);
          }}
          onImageSelect={(e) => {
            saveSelection();
            handleImageSelect(e);
          }}
          onFileSelect={handleOtherFileSelect}
          disabled={disabled}
        />
        <Divider orientation="vertical" flexItem sx={{ mx: 0.5 }} />
        <MarkdownControl
          onClick={() => {
            setMarkdownEditContent(htmlToMarkdown(value ?? ""));
            setMarkdownModalOpen(true);
          }}
          disabled={disabled}
        />
      </Box>
      <Divider />

      <EditorContentArea
        editorRef={editorRef}
        placeholder={placeholder}
        disabled={disabled}
        minHeight={minHeight}
        onInput={handleInput}
        onKeyDown={handleKeyDown}
        onPaste={handlePaste}
        onClick={handleEditorClick}
      />

      <AttachmentsListDisplay
        attachments={otherAttachments}
        onRemove={removeOtherAttachment}
      />

      <LinkInsertPopover
        open={Boolean(linkPopup)}
        anchor={linkPopup?.anchor ?? null}
        defaultUrl={linkPopup?.url ?? ""}
        defaultText={linkPopup?.text ?? ""}
        onInsert={handleInsertLink}
        onClose={() => setLinkPopup(null)}
      />

      <MarkdownEditorDialog
        open={markdownModalOpen}
        content={markdownEditContent}
        onChange={setMarkdownEditContent}
        onApply={() => {
          const html = markdownToHtml(markdownEditContent) || "";
          handleMarkdownApply(html);
        }}
        onClose={() => setMarkdownModalOpen(false)}
      />

      <CodeBlockInsertDialog
        open={codeBlockDialogOpen}
        onInsert={handleInsertCodeBlock}
        onClose={() => setCodeBlockDialogOpen(false)}
      />
    </Paper>
  );
}
