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
  $createRangeSelection,
  $getSelection,
  $isRangeSelection,
  $setSelection,
  FORMAT_ELEMENT_COMMAND,
  FORMAT_TEXT_COMMAND,
  type ElementFormatType,
  INDENT_CONTENT_COMMAND,
  OUTDENT_CONTENT_COMMAND,
  SELECTION_CHANGE_COMMAND,
  COMMAND_PRIORITY_CRITICAL,
  UNDO_COMMAND,
  REDO_COMMAND,
  CAN_UNDO_COMMAND,
  CAN_REDO_COMMAND,
} from "lexical";
import {
  INSERT_ORDERED_LIST_COMMAND,
  INSERT_UNORDERED_LIST_COMMAND,
} from "@lexical/list";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import {
  ToggleButton,
  Stack,
  Divider,
  Tooltip,
  Box,
  useTheme,
  Popover,
  TextField,
  Button,
  IconButton,
  Typography,
} from "@wso2/oxygen-ui";
import { useEffect, useState, useCallback, useRef } from "react";
import {
  Bold,
  Italic,
  Underline,
  Strikethrough,
  List,
  ListOrdered,
  AlignLeft,
  AlignCenter,
  AlignRight,
  AlignJustify,
  ImageIcon,
  Code,
  Link as LinkIcon,
  Indent,
  Outdent,
  Undo as UndoIcon,
  Redo as RedoIcon,
  Paperclip,
  ChevronLeft,
  ChevronRight,
} from "@wso2/oxygen-ui-icons-react";
import { mergeRegister } from "@lexical/utils";
import {
  MAX_IMAGE_SIZE_BYTES,
  RICH_TEXT_BLOCK_TAGS,
} from "@constants/supportConstants";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import {
  INSERT_IMAGE_COMMAND,
  scrollElement,
  sanitizeUrl,
} from "@utils/richTextEditor";
import { $createCodeNode, $isCodeNode } from "@lexical/code";
import { $patchStyleText, $setBlocksType } from "@lexical/selection";
import { TOGGLE_LINK_COMMAND, $isLinkNode } from "@lexical/link";
import {
  $createHeadingNode,
  $createQuoteNode,
  $isHeadingNode,
  $isQuoteNode,
} from "@lexical/rich-text";
import { $createParagraphNode } from "lexical";

export type ToolbarVariant = "full" | "describeIssue";

const Toolbar = ({
  onAttachmentClick,
  disabled = false,
  variant = "full",
  showKeyboardHint = false,
}: {
  onAttachmentClick?: () => void;
  disabled?: boolean;
  variant?: ToolbarVariant;
  showKeyboardHint?: boolean;
}) => {
  const isDescribeIssue = variant === "describeIssue";
  const [editor] = useLexicalComposerContext();
  const theme = useTheme();
  const { showError } = useErrorBanner();

  const [isBold, setIsBold] = useState(false);
  const [isItalic, setIsItalic] = useState(false);
  const [isUnderline, setIsUnderline] = useState(false);
  const [isStrikethrough, setIsStrikethrough] = useState(false);
  const [isLink, setIsLink] = useState(false);
  const [isCode, setIsCode] = useState(false);
  const [canUndo, setCanUndo] = useState(false);
  const [canRedo, setCanRedo] = useState(false);
  const [blockVariant, setBlockVariant] = useState("body1");

  const [linkAnchorEl, setLinkAnchorEl] = useState<HTMLButtonElement | null>(
    null,
  );
  const [linkUrl, setLinkUrl] = useState("");
  const [linkText, setLinkText] = useState("");

  // Focus editor helper
  const focusEditor = useCallback(() => {
    editor.focus();
  }, [editor]);

  const updateToolbar = useCallback(() => {
    const selection = $getSelection();
    if ($isRangeSelection(selection)) {
      setIsBold(selection.hasFormat("bold"));
      setIsItalic(selection.hasFormat("italic"));
      setIsUnderline(selection.hasFormat("underline"));
      setIsStrikethrough(selection.hasFormat("strikethrough"));

      const node = selection.anchor.getNode();
      const parent = node?.getParent();
      setIsLink($isLinkNode(parent) || $isLinkNode(node));

      let element: ReturnType<typeof selection.anchor.getNode>;
      try {
        element =
          selection.anchor.type === "element"
            ? selection.anchor.getNode()
            : selection.anchor.getNode().getTopLevelElementOrThrow();
      } catch {
        setBlockVariant("body1");
        setIsCode(false);
        return;
      }
      setIsCode($isCodeNode(element));

      if ($isHeadingNode(element)) {
        setBlockVariant(element.getTag());
      } else if ($isQuoteNode(element)) {
        setBlockVariant("quote");
      } else if ($isCodeNode(element)) {
        setBlockVariant("body1");
      } else {
        setBlockVariant("body1");
      }
    }
  }, []);

  useEffect(() => {
    return mergeRegister(
      editor.registerCommand(
        SELECTION_CHANGE_COMMAND,
        () => {
          updateToolbar();
          return false;
        },
        COMMAND_PRIORITY_CRITICAL,
      ),
      editor.registerUpdateListener(({ editorState }) => {
        editorState.read(() => {
          updateToolbar();
        });
      }),
      editor.registerCommand(
        CAN_UNDO_COMMAND,
        (payload: boolean) => {
          setCanUndo(payload);
          return false;
        },
        COMMAND_PRIORITY_CRITICAL,
      ),
      editor.registerCommand(
        CAN_REDO_COMMAND,
        (payload: boolean) => {
          setCanRedo(payload);
          return false;
        },
        COMMAND_PRIORITY_CRITICAL,
      ),
    );
  }, [editor, updateToolbar]);

  const onFormatText = (
    format: "bold" | "italic" | "underline" | "strikethrough",
  ) => {
    focusEditor();
    editor.dispatchCommand(FORMAT_TEXT_COMMAND, format);
  };

  const onFormatAlign = (format: ElementFormatType) => {
    focusEditor();
    editor.dispatchCommand(FORMAT_ELEMENT_COMMAND, format);
  };

  const imageInputRef = useRef<HTMLInputElement | null>(null);

  const onImageUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    focusEditor();
    const file = e.target.files?.[0];
    if (file) {
      const resetInput = () => {
        if (imageInputRef.current) imageInputRef.current.value = "";
      };
      if (file.size > MAX_IMAGE_SIZE_BYTES) {
        showError(
          `Image "${file.name}" exceeds the maximum allowed size of ${MAX_IMAGE_SIZE_BYTES / (1024 * 1024)}MB. Please choose a smaller file.`,
        );
        resetInput();
        return;
      }
      const reader = new FileReader();
      reader.onload = () => {
        if (typeof reader.result === "string") {
          editor.dispatchCommand(INSERT_IMAGE_COMMAND, {
            src: reader.result,
            altText: file.name,
          });
        }
        resetInput();
      };
      reader.onerror = () => {
        showError(
          `Failed to read image file "${file.name}". Please try again.`,
        );
        resetInput();
      };
      reader.readAsDataURL(file);
    }
  };

  const onFormatCode = () => {
    focusEditor();
    editor.update(() => {
      const selection = $getSelection();
      if ($isRangeSelection(selection)) {
        if (isCode) {
          $setBlocksType(selection, () => $createParagraphNode());
        } else {
          $setBlocksType(selection, () => $createCodeNode());
        }
      }
    });
  };

  const onBlockChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    focusEditor();
    const blockType = e.target.value;
    setBlockVariant(blockType);
    editor.update(() => {
      const selection = $getSelection();
      if ($isRangeSelection(selection)) {
        if (blockType.startsWith("h")) {
          const level = parseInt(blockType.substring(1)) as
            | 1
            | 2
            | 3
            | 4
            | 5
            | 6;
          $setBlocksType(selection, () =>
            $createHeadingNode(
              `h${level}` as "h1" | "h2" | "h3" | "h4" | "h5" | "h6",
            ),
          );
          $patchStyleText(selection, { "font-size": null });
        } else if (blockType === "quote") {
          $setBlocksType(selection, () => $createQuoteNode());
          $patchStyleText(selection, { "font-size": null });
        } else {
          $setBlocksType(selection, () => $createParagraphNode());
          const typo = theme.typography[
            blockType as keyof typeof theme.typography
          ] as { fontSize?: string | number };
          const fontSize = typo?.fontSize ?? theme.typography.body1.fontSize;
          $patchStyleText(selection, { "font-size": String(fontSize) });
        }
      }
    });
  };

  const onLinkClick = (event: React.MouseEvent<HTMLElement>) => {
    focusEditor();
    if (isLink) {
      editor.dispatchCommand(TOGGLE_LINK_COMMAND, null);
    } else {
      setLinkAnchorEl(event.currentTarget as HTMLButtonElement);
      const selection = editor.getEditorState().read(() => $getSelection());
      if ($isRangeSelection(selection)) {
        setLinkText(selection.getTextContent());
      }
    }
  };

  const onLinkSubmit = () => {
    focusEditor();
    const sanitized = sanitizeUrl(linkUrl);
    if (sanitized) {
      editor.update(() => {
        const selection = $getSelection();
        if ($isRangeSelection(selection)) {
          if (linkText && linkText !== selection.getTextContent()) {
            selection.insertText(linkText);
            const afterInsert = $getSelection();
            if ($isRangeSelection(afterInsert)) {
              const endKey = afterInsert.anchor.key;
              const endOffset = afterInsert.anchor.offset;
              const startOffset = Math.max(0, endOffset - linkText.length);
              const rangeSelection = $createRangeSelection();
              rangeSelection.anchor.set(endKey, startOffset, "text");
              rangeSelection.focus.set(endKey, endOffset, "text");
              $setSelection(rangeSelection);
            }
          }
          editor.dispatchCommand(TOGGLE_LINK_COMMAND, sanitized);
        }
      });
    }
    setLinkAnchorEl(null);
    setLinkUrl("");
    setLinkText("");
  };

  const [canScrollLeft, setCanScrollLeft] = useState(false);
  const [canScrollRight, setCanScrollRight] = useState(false);
  const toolbarScrollRef = useRef<HTMLDivElement | null>(null);

  const scrollRef = useCallback((node: HTMLDivElement | null) => {
    toolbarScrollRef.current = node;
  }, []);

  useEffect(() => {
    const node = toolbarScrollRef.current;
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
  }, []);

  const scroll = useCallback((direction: "left" | "right") => {
    scrollElement(toolbarScrollRef, direction);
  }, []);

  return (
    <Box
      sx={{
        position: "relative",
        opacity: disabled ? 0.5 : 1,
        transition: "opacity 0.2s",
        width: "100%",
        display: "flex",
        alignItems: "center",
        mb: 1, // Move margin here
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
            onClick={() => scroll("left")}
            aria-label="Scroll toolbar left"
            sx={{
              color: "text.secondary",
              "&:hover": { color: "primary.main", bgcolor: "action.hover" },
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
          spacing={0.25}
          sx={{
            flexWrap: "nowrap",
            gap: 0.25,
            width: "max-content",
            ...(disabled && { pointerEvents: "none" }),
          }}
        >
          <Tooltip title="Undo">
            <span>
              <ToggleButton
                size="small"
                value="undo"
                disabled={!canUndo}
                onClick={() => {
                  focusEditor();
                  editor.dispatchCommand(UNDO_COMMAND, undefined);
                }}
              >
                <UndoIcon size={16} />
              </ToggleButton>
            </span>
          </Tooltip>
          <Tooltip title="Redo">
            <span>
              <ToggleButton
                size="small"
                value="redo"
                disabled={!canRedo}
                onClick={() => {
                  focusEditor();
                  editor.dispatchCommand(REDO_COMMAND, undefined);
                }}
              >
                <RedoIcon size={16} />
              </ToggleButton>
            </span>
          </Tooltip>

          {!isDescribeIssue && (
            <>
              <Divider orientation="vertical" flexItem sx={{ mx: 1 }} />

              <Tooltip title="Font variant">
                <Box
                  component="span"
                  sx={{
                    display: "inline-flex",
                    alignItems: "center",
                  }}
                >
                  <select
                    aria-label="Font variant"
                    value={blockVariant}
                    onChange={onBlockChange}
                    onClick={(e) => e.stopPropagation()}
                    style={{
                      height: 28,
                      padding: "0 4px",
                      fontSize: theme.typography.body2.fontSize as string,
                      border: `1px solid ${theme.palette.divider}`,
                      borderRadius: 4,
                      backgroundColor: "transparent",
                      color: "inherit",
                      cursor: "pointer",
                      outline: "none",
                    }}
                  >
                    {[
                      ...RICH_TEXT_BLOCK_TAGS,
                      {
                        value: "quote",
                        label: "Quote",
                        variant: "quote" as const,
                      },
                    ].map(({ value, label }) => (
                      <option
                        key={value}
                        value={value}
                        style={{
                          backgroundColor: theme.palette.background.paper,
                          color: theme.palette.text.primary,
                        }}
                      >
                        {label}
                      </option>
                    ))}
                  </select>
                </Box>
              </Tooltip>

              <Divider orientation="vertical" flexItem sx={{ mx: 1 }} />
            </>
          )}

          <Tooltip title="Bold">
            <ToggleButton
              size="small"
              value="bold"
              selected={isBold}
              onClick={() => onFormatText("bold")}
            >
              <Bold size={16} />
            </ToggleButton>
          </Tooltip>

          <Tooltip title="Italic">
            <ToggleButton
              size="small"
              value="italic"
              selected={isItalic}
              onClick={() => onFormatText("italic")}
            >
              <Italic size={16} />
            </ToggleButton>
          </Tooltip>

          <Tooltip title="Underline">
            <ToggleButton
              size="small"
              value="underline"
              selected={isUnderline}
              onClick={() => onFormatText("underline")}
            >
              <Underline size={16} />
            </ToggleButton>
          </Tooltip>

          <Tooltip title="Strikethrough">
            <ToggleButton
              size="small"
              value="strikethrough"
              selected={isStrikethrough}
              onClick={() => onFormatText("strikethrough")}
            >
              <Strikethrough size={16} />
            </ToggleButton>
          </Tooltip>

          <Divider orientation="vertical" flexItem sx={{ mx: 1 }} />

          <Tooltip title="Align Left">
            <ToggleButton
              size="small"
              value="left"
              onClick={() => onFormatAlign("left")}
            >
              <AlignLeft size={16} />
            </ToggleButton>
          </Tooltip>

          <Tooltip title="Align Center">
            <ToggleButton
              size="small"
              value="center"
              onClick={() => onFormatAlign("center")}
            >
              <AlignCenter size={16} />
            </ToggleButton>
          </Tooltip>

          <Tooltip title="Align Right">
            <ToggleButton
              size="small"
              value="right"
              onClick={() => onFormatAlign("right")}
            >
              <AlignRight size={16} />
            </ToggleButton>
          </Tooltip>

          <Tooltip title="Justify">
            <ToggleButton
              size="small"
              value="justify"
              onClick={() => onFormatAlign("justify")}
            >
              <AlignJustify size={16} />
            </ToggleButton>
          </Tooltip>

          <Divider orientation="vertical" flexItem sx={{ mx: 1 }} />

          <Tooltip title="Indent">
            <ToggleButton
              size="small"
              value="indent"
              onClick={() => {
                focusEditor();
                editor.dispatchCommand(INDENT_CONTENT_COMMAND, undefined);
              }}
            >
              <Indent size={16} />
            </ToggleButton>
          </Tooltip>

          <Tooltip title="Outdent">
            <ToggleButton
              size="small"
              value="outdent"
              onClick={() => {
                focusEditor();
                editor.dispatchCommand(OUTDENT_CONTENT_COMMAND, undefined);
              }}
            >
              <Outdent size={16} />
            </ToggleButton>
          </Tooltip>

          <Divider orientation="vertical" flexItem sx={{ mx: 1 }} />

          <Tooltip title="Bullet List">
            <ToggleButton
              size="small"
              value="bullet"
              onClick={() => {
                focusEditor();
                editor.dispatchCommand(
                  INSERT_UNORDERED_LIST_COMMAND,
                  undefined,
                );
              }}
            >
              <List size={16} />
            </ToggleButton>
          </Tooltip>

          <Tooltip title="Numbered List">
            <ToggleButton
              size="small"
              value="numbered"
              onClick={() => {
                focusEditor();
                editor.dispatchCommand(INSERT_ORDERED_LIST_COMMAND, undefined);
              }}
            >
              <ListOrdered size={16} />
            </ToggleButton>
          </Tooltip>

          <Divider orientation="vertical" flexItem sx={{ mx: 1 }} />

          <Tooltip title="Link">
            <ToggleButton
              size="small"
              value="link"
              selected={isLink}
              onClick={onLinkClick}
            >
              <LinkIcon size={16} />
            </ToggleButton>
          </Tooltip>

          <Popover
            open={Boolean(linkAnchorEl)}
            anchorEl={linkAnchorEl}
            onClose={() => setLinkAnchorEl(null)}
            anchorOrigin={{
              vertical: "bottom",
              horizontal: "left",
            }}
            PaperProps={{
              sx: { borderRadius: 2, boxShadow: theme.shadows[3] },
            }}
          >
            <Box
              sx={{
                p: 1.5,
                display: "flex",
                flexDirection: "column",
                gap: 1.5,
              }}
            >
              <TextField
                label="URL"
                size="small"
                fullWidth
                value={linkUrl}
                onChange={(e) => setLinkUrl(e.target.value)}
                autoFocus
                slotProps={{
                  input: { sx: { fontSize: "0.8125rem" } },
                  inputLabel: { sx: { fontSize: "0.8125rem" } },
                }}
              />
              <TextField
                label="Text (Optional)"
                size="small"
                fullWidth
                value={linkText}
                onChange={(e) => setLinkText(e.target.value)}
                slotProps={{
                  input: { sx: { fontSize: "0.8125rem" } },
                  inputLabel: { sx: { fontSize: "0.8125rem" } },
                }}
              />
              <Button
                variant="contained"
                size="small"
                onClick={onLinkSubmit}
                disabled={!linkUrl}
                fullWidth
                sx={{ textTransform: "none", py: 0.5 }}
              >
                Add Link
              </Button>
            </Box>
          </Popover>

          <Tooltip title="Code Snippet">
            <ToggleButton
              size="small"
              value="code"
              selected={isCode}
              onClick={onFormatCode}
            >
              <Code size={16} />
            </ToggleButton>
          </Tooltip>

          {!isDescribeIssue && (
            <>
              <Tooltip title="Upload Image">
                <ToggleButton size="small" component="label" value="image">
                  <ImageIcon size={16} />
                  <input
                    ref={imageInputRef}
                    type="file"
                    hidden
                    accept="image/*"
                    onChange={onImageUpload}
                  />
                </ToggleButton>
              </Tooltip>

              {onAttachmentClick && (
                <Tooltip title="Attach File">
                  <ToggleButton
                    size="small"
                    value="attachment"
                    onClick={() => {
                      focusEditor();
                      onAttachmentClick();
                    }}
                  >
                    <Paperclip size={16} />
                  </ToggleButton>
                </Tooltip>
              )}
            </>
          )}
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
            onClick={() => scroll("right")}
            aria-label="Scroll toolbar right"
            sx={{
              color: "text.secondary",
              "&:hover": { color: "primary.main", bgcolor: "action.hover" },
            }}
          >
            <ChevronRight size={16} />
          </IconButton>
        </Box>
      )}

      {showKeyboardHint && !disabled && (
        <Typography
          variant="caption"
          sx={{
            color: "text.secondary",
            ml: 2,
            flexShrink: 0,
            whiteSpace: "nowrap",
          }}
        >
          Press <strong>Enter</strong> to send, <strong>Shift+Enter</strong> for
          newline
        </Typography>
      )}
    </Box>
  );
};

export default Toolbar;
