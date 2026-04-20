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
import { getFileIcon, scrollElement } from "@features/support/utils/richTextEditor";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import { type ReactNode, useEffect, useState, useCallback, useMemo, useRef } from "react";
import Toolbar, {
  type ToolbarVariant,
} from "@components/rich-text-editor/ToolBar";
import type { JSX } from "react";
import { RichTextPlugin } from "@lexical/react/LexicalRichTextPlugin";
import { ImageNode } from "@components/rich-text-editor/ImageNode";
import ImagesPlugin from "@components/rich-text-editor/ImagesPlugin";
import { LinkPlugin } from "@lexical/react/LexicalLinkPlugin";
import { ClickableLinkPlugin } from "@lexical/react/LexicalClickableLinkPlugin";
import { LinkNode } from "@lexical/link";
import { HeadingNode, QuoteNode } from "@lexical/rich-text";
import { CodeNode } from "@lexical/code";
import { useLogger } from "@hooks/useLogger";
import { $generateHtmlFromNodes, $generateNodesFromDOM } from "@lexical/html";
import { $getRoot } from "lexical";
import { OnChangePlugin } from "@lexical/react/LexicalOnChangePlugin";
import { KEY_ENTER_COMMAND, COMMAND_PRIORITY_HIGH } from "lexical";

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
          onChange?.(html);
        });
      }}
    />
  );
};

/**
 * Plugin to call onSubmit only on Ctrl+Enter / Cmd+Enter.
 * Plain Enter is left to Lexical (new lines, list items, paragraphs).
 */
const EnterSubmitPlugin = ({
  onSubmit,
  disabled,
}: {
  onSubmit?: () => void;
  disabled?: boolean;
}) => {
  const [editor] = useLexicalComposerContext();

  useEffect(() => {
    if (!onSubmit || disabled) return;

    const unregEnter = editor.registerCommand(
      KEY_ENTER_COMMAND,
      (event: KeyboardEvent | null) => {
        if (event === null) return false;
        if (!event.ctrlKey && !event.metaKey) {
          return false;
        }
        event.preventDefault?.();
        onSubmit();
        return true;
      },
      COMMAND_PRIORITY_HIGH,
    );

    return unregEnter;
  }, [editor, onSubmit, disabled]);

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
    },
    link: "editor-link",
    code: "editor-code",
  },
};

const Editor = ({
  onAttachmentClick,
  attachments = [],
  onAttachmentRemove,
  disabled = false,
  value,
  onChange,
  resetTrigger,
  minHeight = 150,
  showToolbar = true,
  toolbarVariant = "full",
  onSubmitKeyDown,
  placeholder = "Enter description...",
  id,
  showKeyboardHint = false,
  maxHeight = "300px",
  onFocus,
  onBlur,
  overlayElement,
}: {
  onAttachmentClick?: () => void;
  attachments?: File[];
  onAttachmentRemove?: (index: number) => void;
  disabled?: boolean;
  value?: string;
  onChange?: (html: string) => void;
  resetTrigger?: number;
  minHeight?: number | string;
  showToolbar?: boolean;
  toolbarVariant?: ToolbarVariant;
  /** When provided, Ctrl+Enter / Cmd+Enter triggers submit; plain Enter stays in the editor (lists, paragraphs). */
  onSubmitKeyDown?: () => void;
  placeholder?: string;
  id?: string;
  showKeyboardHint?: boolean;
  maxHeight?: string | number;
  onFocus?: () => void;
  onBlur?: () => void;
  /** Optional element rendered as an absolute overlay at the bottom-right inside the editor. */
  overlayElement?: ReactNode;
}): JSX.Element => {
  const oxygenTheme = useTheme();
  const logger = useLogger();

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
          backgroundColor: "background.acrylic",
          backdropFilter: "blur(10px)",
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
              disabled={disabled}
              variant={toolbarVariant}
              showKeyboardHint={showKeyboardHint}
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
            },
            "& .editor-text-bold": {
              fontWeight: oxygenTheme.typography.fontWeightBold || "bold",
            },
            "& .editor-text-italic": { fontStyle: "italic" },
            "& .editor-text-underline": { textDecoration: "underline" },
            "& .editor-text-strikethrough": {
              textDecoration: "line-through",
            },
            "& .editor-list-ul": { ml: 3, listStyleType: "disc" },
            "& .editor-list-ol": { ml: 3, listStyleType: "decimal" },
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
          }}
        >
          <RichTextPlugin
            contentEditable={
              <ContentEditable
                id={id}
                className="editor-input"
                data-testid="case-description-editor"
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
          <HistoryPlugin />
          <ListPlugin />
          <ImagesPlugin />
          <LinkPlugin />
          <ClickableLinkPlugin />
          <InitialValuePlugin initialHtml={value} />
          <OnChangeHTMLPlugin onChange={onChange} />
          <ResetPlugin resetTrigger={resetTrigger} />
          <EnterSubmitPlugin onSubmit={onSubmitKeyDown} disabled={disabled} />
          {overlayElement && (
            <Box
              sx={{
                position: "absolute",
                bottom: 8,
                right: 8,
                zIndex: 10,
                display: "flex",
                gap: 1,
                pointerEvents: "auto",
              }}
            >
              {overlayElement}
            </Box>
          )}
        </Box>
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
