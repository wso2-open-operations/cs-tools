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

import { Box, Card, CardActionArea, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { ArrowRight, BookOpen } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link } from "react-router";
import { primaryTint } from "@config/colorUtils";
import { browseUrl } from "@utils/browseUrl";

interface KnowledgeBaseCardProps {
  id: string;
  title: string;
  /** Published article count. Undefined while loading or when unavailable. */
  count?: number;
  countLoading?: boolean;
}

const formatArticleCount = (count: number): string => `${count} article${count === 1 ? "" : "s"}`;

/** A product's knowledge base as a card linking to its browse page. Render inside a list item. */
export default function KnowledgeBaseCard({ id, title, count, countLoading }: KnowledgeBaseCardProps): JSX.Element {
  return (
    <Card
      variant="outlined"
      sx={{
        height: "100%",
        borderRadius: 2,
        transition: "border-color 120ms, box-shadow 120ms, transform 120ms",
        "&:hover, &:focus-within": {
          borderColor: "primary.main",
          boxShadow: 2,
          transform: "translateY(-2px)",
        },
        "@media (prefers-reduced-motion: reduce)": { transition: "none", "&:hover": { transform: "none" } },
      }}
    >
      <CardActionArea
        component={Link}
        to={browseUrl({ knowledgeBaseId: id })}
        sx={{ p: 2.5, height: "100%", display: "flex", alignItems: "stretch" }}
      >
        <Stack direction="row" alignItems="center" spacing={1.75} sx={{ width: "100%" }}>
          <Box
            sx={(theme) => ({
              width: 44,
              height: 44,
              borderRadius: 1.5,
              flexShrink: 0,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              bgcolor: primaryTint(theme, 0.12),
              color: theme.palette.primary.main,
              ...theme.applyStyles("light", { color: theme.palette.primary.dark }),
            })}
          >
            <BookOpen size={22} aria-hidden />
          </Box>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography variant="subtitle1" component="span" sx={{ fontWeight: 700, display: "block", lineHeight: 1.3 }}>
              {title}
            </Typography>
            <Typography variant="body2" color="text.secondary" component="span" sx={{ display: "block", mt: 0.25 }}>
              {countLoading ? (
                <Skeleton width={76} sx={{ display: "inline-block" }} aria-hidden />
              ) : count !== undefined ? (
                formatArticleCount(count)
              ) : (
                "Browse articles"
              )}
            </Typography>
          </Box>
          <ArrowRight size={18} aria-hidden style={{ flexShrink: 0, opacity: 0.5 }} />
        </Stack>
      </CardActionArea>
    </Card>
  );
}
