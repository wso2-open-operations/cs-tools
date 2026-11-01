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
  Box,
  Chip,
  Container,
  Grid,
  Link as OxygenLink,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowRight } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link, useNavigate } from "react-router";
import { useKnowledgeBaseCounts, useKnowledgeBases, useRecentArticles } from "@api/useKb";
import EmptyState from "@components/EmptyState";
import LoadingState from "@components/LoadingState";
import QueryErrorState from "@components/QueryErrorState";
import SupportCallout from "@components/SupportCallout";
import { accessibleLinkSx } from "@config/a11yThemeOverrides";
import { primaryTint } from "@config/colorUtils";
import ArticleCard from "@features/kb/components/ArticleCard";
import ArticleList, { ArticleListItem } from "@features/kb/components/ArticleList";
import ArticleListSkeleton from "@features/kb/components/ArticleListSkeleton";
import KnowledgeBaseCard from "@features/kb/components/KnowledgeBaseCard";
import LoadMoreButton from "@features/kb/components/LoadMoreButton";
import SearchBox from "@features/kb/components/SearchBox";
import { useDocumentTitle } from "@hooks/useDocumentTitle";
import { browseUrl } from "@utils/browseUrl";
import { normalizeSearchQuery } from "@utils/searchQuery";
import { toSnippet } from "@utils/snippet";

// Product cards shown on the landing page; the rest are reachable from /knowledge-bases.
const FEATURED_KB_LIMIT = 8;
// Quick links under the hero search.
const QUICK_LINK_LIMIT = 5;
const RECENT_LIMIT = 5;
// Fetch a bigger page than is shown: articles in inactive knowledge bases are
// dropped by the backend after paging, so a page of 5 can come back empty.
const RECENT_FETCH_LIMIT = 20;

export default function KBHomePage(): JSX.Element {
  useDocumentTitle();
  const navigate = useNavigate();
  const kbs = useKnowledgeBases();
  const recent = useRecentArticles(RECENT_FETCH_LIMIT);

  const allKnowledgeBases = kbs.data?.knowledgeBases ?? [];
  const featured = allKnowledgeBases.slice(0, FEATURED_KB_LIMIT);
  const quickLinks = allKnowledgeBases.slice(0, QUICK_LINK_LIMIT);
  const hasMore = allKnowledgeBases.length > FEATURED_KB_LIMIT;
  const { counts, pending: countPending } = useKnowledgeBaseCounts(featured.map((kb) => kb.id));
  const kbTitle = (id: string): string | undefined => allKnowledgeBases.find((kb) => kb.id === id)?.title;

  const recentArticles = recent.articles.slice(0, RECENT_LIMIT);
  const recentLoading = recent.isLoading || (recent.isSkippingEmpty && recentArticles.length === 0);
  const recentFailed = recent.isError && recentArticles.length === 0;
  const recentEmpty = !recentLoading && !recentFailed && recentArticles.length === 0;

  return (
    <>
      <Box
        sx={(theme) => ({
          borderBottom: 1,
          borderColor: "divider",
          background: `linear-gradient(180deg, ${primaryTint(theme, 0.1)} 0%, ${primaryTint(theme, 0.02)} 70%, transparent 100%)`,
        })}
      >
        <Container maxWidth="md" sx={{ py: { xs: 7, md: 11 } }}>
          <Stack spacing={2.5} alignItems="center" textAlign="center">
            <Typography
              variant="overline"
              component="p"
              color="text.secondary"
              sx={{ letterSpacing: 2, fontWeight: 700 }}
            >
              WSO2 Knowledge Base
            </Typography>
            <Typography
              variant="h2"
              component="h1"
              sx={{
                fontWeight: 800,
                lineHeight: 1.1,
                letterSpacing: "-0.02em",
                fontSize: { xs: "2.125rem", sm: "2.75rem", md: "3.25rem" },
                maxWidth: 800,
              }}
            >
              How can we help you?
            </Typography>
            <Typography variant="h6" component="p" color="text.secondary" sx={{ fontWeight: 400, maxWidth: 620 }}>
              Troubleshooting guides, how-tos and answers for every WSO2 product, written by the
              engineers who support them.
            </Typography>
            <Box sx={{ width: "100%", maxWidth: 720, pt: 1.5 }}>
              <SearchBox
                large
                label="Search all articles"
                placeholder="Search by product, error message or topic"
                onSearch={(q) => navigate(browseUrl({ q: normalizeSearchQuery(q) }))}
              />
            </Box>
            {quickLinks.length > 0 && (
              <Stack
                direction="row"
                alignItems="center"
                justifyContent="center"
                flexWrap="wrap"
                useFlexGap
                sx={{ gap: 1, pt: 0.5 }}
              >
                <Typography variant="body2" color="text.secondary" sx={{ mr: 0.5 }}>
                  Browse:
                </Typography>
                {quickLinks.map((kb) => (
                  <Chip
                    key={kb.id}
                    component={Link}
                    to={browseUrl({ knowledgeBaseId: kb.id })}
                    clickable
                    size="small"
                    variant="outlined"
                    label={kb.title}
                    sx={{ bgcolor: "background.paper" }}
                  />
                ))}
              </Stack>
            )}
          </Stack>
        </Container>
      </Box>

      <Container maxWidth="lg" sx={{ py: { xs: 5, md: 8 } }}>
        <Stack direction="row" alignItems="baseline" justifyContent="space-between" sx={{ mb: 2.5 }}>
          <Box>
            <Typography variant="h5" component="h2" sx={{ fontWeight: 700 }}>
              Browse by product
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
              Every knowledge base is maintained by that product's support team.
            </Typography>
          </Box>
          {hasMore && (
            <OxygenLink
              component={Link}
              to={browseUrl()}
              underline="hover"
              sx={[
                { display: "inline-flex", alignItems: "center", gap: 0.5, fontWeight: 600, whiteSpace: "nowrap" },
                accessibleLinkSx,
              ]}
            >
              View all
              <ArrowRight size={16} aria-hidden />
            </OxygenLink>
          )}
        </Stack>

        {kbs.isLoading && <LoadingState label="Loading knowledge bases" py={6} />}
        {kbs.isError && (
          <QueryErrorState
            error={kbs.error}
            message="The knowledge bases could not be loaded."
            onRetry={() => void kbs.refetch()}
          />
        )}
        {kbs.isSuccess && featured.length === 0 && (
          <EmptyState title="No knowledge bases yet" message="Check back soon." />
        )}

        {featured.length > 0 && (
          <Grid container spacing={2} component="ul" sx={{ mb: { xs: 6, md: 8 }, p: 0, listStyle: "none" }}>
            {featured.map((kb) => (
              <Grid key={kb.id} component="li" size={{ xs: 12, sm: 6, lg: 3 }}>
                <KnowledgeBaseCard
                  id={kb.id}
                  title={kb.title}
                  count={counts[kb.id]}
                  countLoading={counts[kb.id] === undefined && countPending[kb.id] === true}
                />
              </Grid>
            ))}
          </Grid>
        )}

        <Grid container spacing={{ xs: 4, md: 6 }}>
          <Grid size={{ xs: 12, md: 8 }}>
            <Stack direction="row" alignItems="baseline" justifyContent="space-between" sx={{ mb: 2.5 }}>
              <Typography variant="h5" component="h2" sx={{ fontWeight: 700 }}>
                Recently published
              </Typography>
              <OxygenLink
                component={Link}
                to={browseUrl()}
                underline="hover"
                sx={[
                  { display: "inline-flex", alignItems: "center", gap: 0.5, fontWeight: 600, whiteSpace: "nowrap" },
                  accessibleLinkSx,
                ]}
              >
                All articles
                <ArrowRight size={16} aria-hidden />
              </OxygenLink>
            </Stack>
            {recentLoading && <ArticleListSkeleton rows={3} label="Loading recent articles" />}
            {recentFailed && (
              <QueryErrorState
                error={recent.error}
                message="Recent articles could not be loaded."
                onRetry={recent.isLoadMoreError ? recent.loadMore : recent.refetch}
              />
            )}
            {recentEmpty && !recent.hasMore && <EmptyState title="No articles yet" />}
            {recentEmpty && recent.hasMore && (
              <>
                <EmptyState
                  title="No recent articles to show here"
                  message="Load more to keep looking, or browse a knowledge base."
                />
                <LoadMoreButton onClick={recent.loadMore} loading={recent.isLoadingMore} />
              </>
            )}
            {recentArticles.length > 0 && (
              <ArticleList label="Recent articles">
                {recentArticles.map((a) => (
                  <ArticleListItem key={a.id}>
                    <ArticleCard
                      id={a.id}
                      title={a.title}
                      snippet={toSnippet(a.body)}
                      publishedOn={a.publishedOn}
                      knowledgeBaseTitle={kbTitle(a.knowledgeBaseId)}
                    />
                  </ArticleListItem>
                ))}
              </ArticleList>
            )}
          </Grid>
          <Grid size={{ xs: 12, md: 4 }}>
            <Stack spacing={3} sx={{ position: { md: "sticky" }, top: { md: 96 } }}>
              <Box>
                <Typography variant="h6" component="h2" sx={{ fontWeight: 700, mb: 1 }}>
                  Search tips
                </Typography>
                <Box
                  component="ul"
                  sx={{ m: 0, pl: 2.5, color: "text.secondary", typography: "body2", "& li + li": { mt: 0.75 } }}
                >
                  <li>Paste the exact error code or message, for example a numeric error code.</li>
                  <li>Pick a product first to narrow the results to one knowledge base.</li>
                  <li>Search for the symptom, not the cause: the title of an article usually names what you see.</li>
                </Box>
              </Box>
              <SupportCallout compact />
            </Stack>
          </Grid>
        </Grid>
      </Container>
    </>
  );
}
