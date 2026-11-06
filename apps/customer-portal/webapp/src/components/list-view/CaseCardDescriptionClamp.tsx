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

import { Box, Typography, type SxProps, type Theme } from "@wso2/oxygen-ui";
import { Image as ImageIcon } from "@wso2/oxygen-ui-icons-react";
import DOMPurify from "dompurify";
import type { JSX, MouseEvent } from "react";
import {
  isAnnouncementDescriptionEffectivelyEmpty,
  normalizeAnnouncementDescriptionHtml,
} from "@features/announcements/utils/announcements";

export type CaseCardDescriptionClampProps = {
  description: string | null | undefined;
  /** Shown when description is empty or whitespace-only after normalization. */
  emptyLabel?: string;
  /** Merged after base styles (e.g. `overflowWrap`, `minWidth`). */
  sx?: SxProps<Theme>;
  /**
   * When true, render nothing if there is no visible description (no placeholder
   * row). Used on compact overview cards next to a title line.
   */
  hideWhenEmpty?: boolean;
};

const baseHtmlSx: SxProps<Theme> = {
  typography: "body2",
  color: "text.secondary",
  mb: 2,
  display: "-webkit-box",
  WebkitLineClamp: 2,
  WebkitBoxOrient: "vertical",
  overflow: "hidden",
  wordBreak: "break-word",
  minWidth: 0,
  overflowWrap: "anywhere",
  "& p": { mb: 0.5 },
  "& p:last-child": { mb: 0 },
  "& a": {
    color: "primary.main",
    textDecoration: "underline",
  },
  // Prevent embedded rich-text images from expanding card width.
  "& img": {
    maxWidth: "100%",
    width: "auto",
    height: "auto",
  },
  "& figure": {
    margin: 0,
    maxWidth: "100%",
  },
};

const IMAGE_TAG_RE = /<img\b/gi;
const FORBIDDEN_CARD_TAGS = ["table", "thead", "tbody", "tfoot", "tr", "th", "td", "colgroup", "col", "code", "pre"];

/**
 * Two-line clamped HTML description for case-style list cards (sanitized; link
 * clicks do not bubble to parent `Form.CardButton`).
 *
 * @param props - Raw API description and optional empty label / sx overrides.
 * @returns {JSX.Element | null} Plain placeholder, sanitized HTML block, or null.
 */
export default function CaseCardDescriptionClamp({
  description,
  emptyLabel = "--",
  sx,
  hideWhenEmpty = false,
}: CaseCardDescriptionClampProps): JSX.Element | null {
  const isEmpty =
    !description || isAnnouncementDescriptionEffectivelyEmpty(description);

  // Images are never rendered inside a card: they would make the cards differ in
  // height. The card shows a small "N images" note instead, in the second line.
  const imageCount = description ? (description.match(IMAGE_TAG_RE) ?? []).length : 0;
  const cleanHtml = description
    ? DOMPurify.sanitize(normalizeAnnouncementDescriptionHtml(description), {
        FORBID_TAGS: [...FORBIDDEN_CARD_TAGS, "img", "figure"],
        FORBID_CONTENTS: FORBIDDEN_CARD_TAGS,
      })
    : "";
  const hasText =
    !!cleanHtml && !isAnnouncementDescriptionEffectivelyEmpty(cleanHtml);

  if (hideWhenEmpty && isEmpty && imageCount === 0) {
    return null;
  }

  if (imageCount > 0) {
    const imageNote = (
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 0.5,
          color: "text.secondary",
          typography: "caption",
          lineHeight: 1.43,
        }}
      >
        <ImageIcon size={12} aria-hidden />
        {imageCount === 1 ? "1 image" : `${imageCount} images`}
      </Box>
    );
    const imageSx = [
      { typography: "body2", color: "text.secondary", mb: 0, minWidth: 0 },
      ...(sx ? [sx] : []),
    ] as SxProps<Theme>;
    return (
      <Box sx={imageSx}>
        {hasText ? (
          <Box
            component="div"
            sx={{
              display: "-webkit-box",
              WebkitLineClamp: 1,
              WebkitBoxOrient: "vertical",
              overflow: "hidden",
              wordBreak: "break-word",
              overflowWrap: "anywhere",
              "& p": { m: 0 },
            }}
            // biome-ignore lint/security/noDangerouslySetInnerHtml: sanitized with DOMPurify
            dangerouslySetInnerHTML={{ __html: cleanHtml }}
          />
        ) : null}
        {imageNote}
      </Box>
    );
  }

  if (isEmpty) {
    const emptySx = [({ mb: 2 } as const), ...(sx ? [sx] : [])] as SxProps<Theme>;
    return (
      <Typography variant="body2" color="text.secondary" sx={emptySx}>
        {emptyLabel}
      </Typography>
    );
  }

  const htmlSx = [baseHtmlSx, ...(sx ? [sx] : [])] as SxProps<Theme>;

  return (
    <Box
      component="div"
      onClick={(e: MouseEvent) => {
        if ((e.target as HTMLElement).closest("a")) {
          e.stopPropagation();
        }
      }}
      sx={htmlSx}
      // biome-ignore lint/security/noDangerouslySetInnerHtml: sanitized with DOMPurify
      dangerouslySetInnerHTML={{
        __html: DOMPurify.sanitize(
          normalizeAnnouncementDescriptionHtml(description),
          {
            FORBID_TAGS: ["table", "thead", "tbody", "tfoot", "tr", "th", "td", "colgroup", "col", "code", "pre"],
            FORBID_CONTENTS: ["table", "thead", "tbody", "tfoot", "tr", "th", "td", "colgroup", "col", "code", "pre"],
          },
        ),
      }}
    />
  );
}
