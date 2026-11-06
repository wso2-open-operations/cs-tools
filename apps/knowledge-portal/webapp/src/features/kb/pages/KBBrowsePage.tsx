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

import { Box, Button, Container, Grid, Stack, Typography } from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link, useParams, useSearchParams } from "react-router";
import { isValidId } from "@api/backend/types";
import { isClientError, isNotFoundError } from "@api/backend/client";
import { useArticleSearch, useKnowledgeBaseCounts, useKnowledgeBases } from "@api/useKb";
import EmptyState from "@components/EmptyState";
import QueryErrorState from "@components/QueryErrorState";
import SupportCallout from "@components/SupportCallout";
import ArticleCard from "@features/kb/components/ArticleCard";
import ArticleList, { ArticleListItem } from "@features/kb/components/ArticleList";
import ArticleListSkeleton from "@features/kb/components/ArticleListSkeleton";
import KnowledgeBaseFilter from "@features/kb/components/KnowledgeBaseFilter";
import LoadMoreButton from "@features/kb/components/LoadMoreButton";
import SearchBox from "@features/kb/components/SearchBox";
import { useDocumentTitle } from "@hooks/useDocumentTitle";
import { browseUrl } from "@utils/browseUrl";
import { normalizeSearchQuery } from "@utils/searchQuery";
import { toSnippet } from "@utils/snippet";

const NO_MATCH_TIPS = [
  "Check the spelling of product names and error codes.",
  "Try fewer or more general keywords.",
  "Search all knowledge bases instead of one product.",
] as const;

export default function KBBrowsePage(): JSX.Element {
  const [params, setParams] = useSearchParams();

  const { id: rawKb } = useParams();
  const knowledgeBaseId = isValidId(rawKb) ? rawKb : "";
  const searchQuery = normalizeSearchQuery(params.get("q"));

  const { data: kbData } = useKnowledgeBases();
  const search = useArticleSearch({ knowledgeBaseId, searchQuery });

  const knowledgeBases = kbData?.knowledgeBases ?? [];
  const { counts } = useKnowledgeBaseCounts(knowledgeBases.map((kb) => kb.id));
  const selectedKb = knowledgeBases.find((kb) => kb.id === knowledgeBaseId);
  const kbTitle = (id: string): string | undefined => knowledgeBases.find((kb) => kb.id === id)?.title;

  const heading = searchQuery
    ? `Search results for "${searchQuery}"`
    : (selectedKb?.title ?? "All articles");
  useDocumentTitle(heading);

  const onSearch = (raw: string): void => {
    const next = new URLSearchParams(params);
    const q = normalizeSearchQuery(raw);
    if (q) next.set("q", q);
    else next.delete("q");
    setParams(next);
  };

  const { articles, total } = search;
  const showInitialError = search.isError && articles.length === 0;
  const unknownKnowledgeBase = !!knowledgeBaseId && isNotFoundError(search.error);
  const showSkeleton = search.isLoading || (search.isSkippingEmpty && articles.length === 0);

  // With a knowledge base filter the backend's total is exact. Without one the
  // backend filters each upstream page afterwards, so total only bounds the
  // real count from above: say so, or say nothing.
  let countLine = "";
  if (articles.length > 0 || !search.hasMore) {
    if (knowledgeBaseId) {
      countLine = `${total} article${total === 1 ? "" : "s"}`;
      if (articles.length > 0 && total > articles.length) countLine += `, showing ${articles.length}`;
    } else if (search.hasMore) {
      countLine = `Showing ${articles.length} of up to ${total} articles`;
    } else {
      countLine = `${articles.length} article${articles.length === 1 ? "" : "s"}`;
    }
  }

  const scopeLine = searchQuery
    ? selectedKb
      ? `In ${selectedKb.title}`
      : "In all knowledge bases"
    : selectedKb
      ? "Every published article in this knowledge base, newest first."
      : "Every published article across all knowledge bases, newest first.";

  return (
    <Container maxWidth="lg" sx={{ py: { xs: 4, md: 5 } }}>
      <Box sx={{ mb: { xs: 3, md: 4 } }}>
        <Typography
          variant="h4"
          component="h1"
          sx={{ fontWeight: 700, mb: 0.5, overflowWrap: "anywhere", fontSize: { xs: "1.75rem", md: "2.125rem" } }}
        >
          {heading}
        </Typography>
        <Typography variant="body1" color="text.secondary">
          {scopeLine}
        </Typography>
      </Box>

      <Grid container spacing={{ xs: 3, md: 5 }}>
        <Grid size={{ xs: 12, md: 3 }}>
          <Box sx={{ position: { md: "sticky" }, top: { md: 88 } }}>
            <KnowledgeBaseFilter
              knowledgeBases={knowledgeBases}
              selectedId={knowledgeBaseId}
              searchQuery={searchQuery}
              counts={counts}
            />
          </Box>
        </Grid>

        <Grid size={{ xs: 12, md: 9 }}>
          <Stack direction={{ xs: "column", sm: "row" }} spacing={1.5} alignItems={{ sm: "center" }} sx={{ mb: 3 }}>
            {/* Remount when the URL query changes so the input follows back/forward. */}
            <SearchBox
              key={`${knowledgeBaseId}:${searchQuery}`}
              label="Search articles"
              placeholder={selectedKb ? `Search in ${selectedKb.title}` : "Search all articles"}
              initialValue={searchQuery}
              onSearch={onSearch}
            />
            {searchQuery && (
              <Button
                component={Link}
                to={browseUrl({ knowledgeBaseId: knowledgeBaseId || undefined })}
                color="inherit"
                size="small"
                startIcon={<X size={16} aria-hidden />}
                sx={{ flexShrink: 0, whiteSpace: "nowrap" }}
              >
                Clear search
              </Button>
            )}
          </Stack>

          {showSkeleton && <ArticleListSkeleton label="Loading articles" />}

          {showInitialError && (
            <QueryErrorState
              error={search.error}
              message={
                unknownKnowledgeBase
                  ? "This knowledge base does not exist or is no longer available."
                  : "Articles could not be loaded."
              }
              onRetry={isClientError(search.error) ? undefined : search.refetch}
            />
          )}

          {!showSkeleton && !showInitialError && (
            <>
              {countLine && (
                <Typography
                  variant="body2"
                  color="text.secondary"
                  role="status"
                  aria-live="polite"
                  sx={{ mb: 1.5 }}
                >
                  {countLine}
                </Typography>
              )}

              {articles.length === 0 ? (
                search.hasMore ? (
                  <EmptyState
                    title="No articles found yet"
                    message="The articles checked so far did not match. Load more to keep looking."
                  />
                ) : (
                  <EmptyState
                    title="No articles matched"
                    message="Try a different search or knowledge base."
                    tips={searchQuery ? NO_MATCH_TIPS : undefined}
                    action={
                      searchQuery || knowledgeBaseId ? (
                        <Button component={Link} to={browseUrl()} variant="outlined" size="small">
                          Browse all articles
                        </Button>
                      ) : undefined
                    }
                  />
                )
              ) : (
                <ArticleList label="Articles" busy={search.isRefreshing}>
                  {articles.map((article) => (
                    <ArticleListItem key={article.id}>
                      <ArticleCard
                        id={article.id}
                        title={article.title}
                        snippet={toSnippet(article.body)}
                        updatedOn={article.updatedOn}
                        publishedOn={article.publishedOn}
                        knowledgeBaseTitle={knowledgeBaseId ? undefined : kbTitle(article.knowledgeBaseId)}
                        query={searchQuery}
                      />
                    </ArticleListItem>
                  ))}
                </ArticleList>
              )}

              {search.isLoadMoreError && (
                <QueryErrorState
                  error={search.error}
                  message="More articles could not be loaded."
                />
              )}

              {search.hasMore && (
                <LoadMoreButton onClick={search.loadMore} loading={search.isLoadingMore} />
              )}

              {search.isCapped && (
                <Typography variant="body2" color="text.secondary" role="status" sx={{ mt: 3, textAlign: "center" }}>
                  Only the first {articles.length} results can be shown. Refine your search or pick a
                  knowledge base to narrow them down.
                </Typography>
              )}
            </>
          )}

          <Box sx={{ mt: 6 }}>
            <SupportCallout />
          </Box>
        </Grid>
      </Grid>
    </Container>
  );
}
