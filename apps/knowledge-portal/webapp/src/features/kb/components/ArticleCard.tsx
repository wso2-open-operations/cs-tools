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

import { Box, Card, CardActionArea, Stack, Typography } from "@wso2/oxygen-ui";
import { ChevronRight, FileText, Layers } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link } from "react-router";
import Highlight from "@components/Highlight";
import { articleUrl } from "@utils/browseUrl";
import { formatRelativeDate } from "@utils/formatDate";

interface ArticleCardProps {
  id: string;
  title: string;
  /** Plain-text excerpt. Omit for a compact title-only card. */
  snippet?: string;
  publishedOn?: string | null;
  updatedOn?: string | null;
  /** Name of the knowledge base the article belongs to, shown as a label. */
  knowledgeBaseTitle?: string;
  /** Search terms to emphasise in the title and snippet. */
  query?: string;
  /** Title-only row for sidebars. */
  compact?: boolean;
}

/**
 * One article as a real link (CardActionArea renders an anchor), so
 * middle-click, "open in new tab" and screen-reader link lists all work.
 * Render inside a list item.
 */
export default function ArticleCard({
  id,
  title,
  snippet,
  publishedOn,
  updatedOn,
  knowledgeBaseTitle,
  query,
  compact,
}: ArticleCardProps): JSX.Element {
  const date = updatedOn ? formatRelativeDate(updatedOn) : formatRelativeDate(publishedOn);
  const dateVerb = updatedOn ? "Updated" : "Published";
  const showMeta = !compact && (knowledgeBaseTitle || date);

  return (
    <Card
      variant="outlined"
      sx={{
        borderRadius: 2,
        transition: "border-color 120ms, box-shadow 120ms",
        "&:hover, &:focus-within": { borderColor: "primary.main", boxShadow: 1 },
      }}
    >
      <CardActionArea
        component={Link}
        to={articleUrl(id)}
        sx={{
          px: { xs: 2, sm: 2.5 },
          py: compact ? 1.5 : 2.25,
          display: "flex",
          alignItems: "flex-start",
          gap: { xs: 1.5, sm: 2 },
          textAlign: "left",
        }}
      >
        {!compact && (
          <Box
            sx={{
              width: 36,
              height: 36,
              borderRadius: 1.5,
              flexShrink: 0,
              display: { xs: "none", sm: "flex" },
              alignItems: "center",
              justifyContent: "center",
              bgcolor: "action.hover",
              color: "text.secondary",
            }}
          >
            <FileText size={18} aria-hidden />
          </Box>
        )}
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography
            variant={compact ? "body2" : "subtitle1"}
            component="span"
            sx={{ fontWeight: 600, display: "block", lineHeight: 1.35 }}
          >
            <Highlight text={title} query={query} />
          </Typography>
          {!compact && snippet && (
            <Typography
              variant="body2"
              color="text.secondary"
              sx={{
                mt: 0.5,
                display: "-webkit-box",
                WebkitLineClamp: 2,
                WebkitBoxOrient: "vertical",
                overflow: "hidden",
              }}
            >
              <Highlight text={snippet} query={query} />
            </Typography>
          )}
          {showMeta && (
            <Stack
              direction="row"
              alignItems="center"
              flexWrap="wrap"
              sx={{ mt: 1, columnGap: 1.5, rowGap: 0.5, color: "text.secondary" }}
            >
              {knowledgeBaseTitle && (
                <Typography
                  variant="caption"
                  sx={{ fontWeight: 600, display: "inline-flex", alignItems: "center", gap: 0.5 }}
                >
                  <Layers size={12} aria-hidden />
                  {knowledgeBaseTitle}
                </Typography>
              )}
              {date && (
                <Typography variant="caption" component="time" dateTime={date.iso} title={date.absolute}>
                  {dateVerb} {date.label}
                </Typography>
              )}
            </Stack>
          )}
        </Box>
        {!compact && (
          <ChevronRight size={18} aria-hidden style={{ flexShrink: 0, marginTop: 4, opacity: 0.5 }} />
        )}
      </CardActionArea>
    </Card>
  );
}
