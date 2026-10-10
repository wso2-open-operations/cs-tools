// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.

import {
  Card,
  CardActionArea,
  Skeleton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { Eye } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useNavigate } from "react-router";
import { useSearchArticles } from "@features/kb/api/useKb";
import type { KBSortOrder } from "@features/kb/types/kb";

interface ArticleSectionProps {
  title: string;
  sortBy: KBSortOrder;
  metric?: "views" | "rating";
  limit?: number;
}

export default function ArticleSection({
  title,
  sortBy,
  metric,
  limit = 5,
}: ArticleSectionProps): JSX.Element | null {
  const navigate = useNavigate();
  const { data, isLoading, isError } = useSearchArticles({ sortBy, limit });

  if (isError) return null;

  const articles = data?.articles ?? [];
  if (!isLoading && articles.length === 0) return null;

  return (
    <Stack spacing={1.5} sx={{ mb: 5 }}>
      <Typography variant="h6" sx={{ fontWeight: 600 }}>
        {title}
      </Typography>

      {isLoading &&
        Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} variant="rounded" height={56} />
        ))}

      {articles.map((article) => (
        <Card key={article.id} variant="outlined">
          <CardActionArea
            onClick={() => navigate(`/articles/${article.id}`)}
            sx={{ px: 2.5, py: 1.75 }}
          >
            <Stack
              direction="row"
              alignItems="center"
              justifyContent="space-between"
              spacing={2}
            >
              <Typography variant="body2" sx={{ fontWeight: 500, minWidth: 0 }}>
                {article.title}
              </Typography>

              {metric === "views" && article.viewCount != null && (
                <Stack
                  direction="row"
                  spacing={0.5}
                  alignItems="center"
                  sx={{ color: "text.secondary", flexShrink: 0 }}
                >
                  <Eye size={14} />
                  <Typography variant="caption">{article.viewCount}</Typography>
                </Stack>
              )}

              {metric === "rating" && article.rating != null && (
                <Typography
                  variant="caption"
                  sx={{ color: "text.secondary", flexShrink: 0 }}
                >
                  {article.rating.toFixed(1)}
                </Typography>
              )}
            </Stack>
          </CardActionArea>
        </Card>
      ))}
    </Stack>
  );
}
