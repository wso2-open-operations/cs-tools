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
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Breadcrumbs,
  Button,
  Chip,
  Container,
  Divider,
  Grid,
  Link as OxygenLink,
  Paper,
  Snackbar,
  Stack,
  Typography,
  useMediaQuery,
  useTheme,
} from "@wso2/oxygen-ui";
import type { Theme } from "@mui/material/styles";
import { ArrowLeft, ChevronDown, Clock, Layers } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX, type MouseEvent } from "react";
import { Link, useParams } from "react-router";
import { isClientError, isNotFoundError } from "@api/backend/client";
import { isValidId } from "@api/backend/types";
import type { KBArticle, RelatedArticle } from "@api/backend/types";
import { useArticle, useKnowledgeBases, useRelatedArticles } from "@api/useKb";
import LoadingState from "@components/LoadingState";
import NotFoundState from "@components/NotFoundState";
import QueryErrorState from "@components/QueryErrorState";
import SupportCallout from "@components/SupportCallout";
import { accessibleLinkSx } from "@config/a11yThemeOverrides";
import { opaquePaper } from "@config/colorUtils";
import ArticleActions from "@features/kb/components/ArticleActions";
import ArticleCard from "@features/kb/components/ArticleCard";
import ArticleList, { ArticleListItem } from "@features/kb/components/ArticleList";
import TableOfContents from "@features/kb/components/TableOfContents";
import { useDocumentTitle } from "@hooks/useDocumentTitle";
import { buildArticleOutline, CODE_BLOCK_CLASS, CODE_COPY_ATTR, CODE_COPY_CLASS } from "@utils/articleOutline";
import { browseUrl } from "@utils/browseUrl";
import { formatDate, formatRelativeDate } from "@utils/formatDate";
import { formatReadingTime, readingTimeMinutes } from "@utils/readingTime";
import { sanitizeArticleHtml } from "@utils/sanitizeHtml";
import { toPlainText } from "@utils/snippet";

/** Height of the sticky header plus breathing room, so anchored headings are not hidden under it. */
const SCROLL_MARGIN_TOP = 96;
const DAY_MS = 24 * 60 * 60 * 1000;

const bodySx = (theme: Theme) => ({
  maxWidth: "76ch",
  fontSize: "1.0625rem",
  lineHeight: 1.75,
  color: "text.primary",
  overflowWrap: "anywhere",
  "& > *:first-of-type": { mt: 0 },
  "& h1, & h2, & h3, & h4": {
    fontWeight: 650,
    lineHeight: 1.3,
    mt: 5,
    mb: 1.5,
    scrollMarginTop: SCROLL_MARGIN_TOP,
    "& a.headerlink, & .headerlink": { display: "none" },
  },
  "& h1": { fontSize: "1.75rem" },
  "& h2": { fontSize: "1.45rem", pb: 1, borderBottom: 1, borderColor: "divider" },
  "& h3": { fontSize: "1.2rem" },
  "& h4": { fontSize: "1.05rem" },
  "& p": { my: 2 },
  "& ul, & ol": { my: 2, pl: 3 },
  "& li": { mb: 1 },
  "& li::marker": { color: "text.secondary" },
  "& a": accessibleLinkSx,
  "& code": {
    px: 0.75,
    py: 0.25,
    borderRadius: 0.75,
    bgcolor: "action.hover",
    fontFamily: "monospace",
    fontSize: "0.9em",
  },
  [`& .${CODE_BLOCK_CLASS}`]: { position: "relative", my: 3 },
  "& pre": {
    overflowX: "auto",
    p: 2,
    pr: 9,
    m: 0,
    borderRadius: 1.5,
    bgcolor: "action.hover",
    border: 1,
    borderColor: "divider",
    fontSize: "0.9rem",
    lineHeight: 1.6,
    "& code": { bgcolor: "transparent", px: 0, py: 0, fontSize: "inherit" },
  },
  [`& .${CODE_COPY_CLASS}`]: {
    position: "absolute",
    top: 8,
    right: 8,
    px: 1.25,
    py: 0.5,
    borderRadius: 1,
    border: 1,
    borderColor: "divider",
    bgcolor: opaquePaper(theme),
    boxShadow: 1,
    color: "text.secondary",
    font: "inherit",
    fontSize: "0.75rem",
    fontWeight: 600,
    lineHeight: 1.4,
    cursor: "pointer",
    "&:hover, &:focus-visible": { borderColor: "primary.main", color: "text.primary" },
    "@media print": { display: "none" },
  },
  "& blockquote": {
    my: 3,
    mx: 0,
    px: 2.5,
    py: 1.5,
    borderLeft: 4,
    borderColor: "primary.main",
    borderRadius: 1,
    bgcolor: "action.hover",
    color: "text.primary",
    "& p": { my: 0.5 },
  },
  "& img": { maxWidth: "100%", height: "auto", borderRadius: 1, my: 2 },
  "& table": { width: "100%", borderCollapse: "collapse", my: 3, fontSize: "0.9375rem", display: "block", overflowX: "auto" },
  "& td, & th": { border: 1, borderColor: "divider", p: 1.25, verticalAlign: "top" },
  "& th": { bgcolor: "action.hover", fontWeight: 600, textAlign: "left" },
  "& hr": { border: 0, borderTop: 1, borderColor: "divider", my: 4 },
});

export default function KBArticlePage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const idIsValid = isValidId(id);

  const articleQuery = useArticle(id);
  const { data: article } = articleQuery;
  const { data: kbData } = useKnowledgeBases();
  const { data: related } = useRelatedArticles(article?.id);

  const knowledgeBaseTitle = article
    ? kbData?.knowledgeBases.find((kb) => kb.id === article.knowledgeBaseId)?.title
    : undefined;

  const notFound = !idIsValid || isNotFoundError(articleQuery.error);

  // An id the backend would reject never reaches it: it cannot exist.
  if (notFound) {
    return (
      <NotFoundState
        title="Article not found"
        message="This article may have been retired, or the link may be incorrect."
      />
    );
  }

  if (articleQuery.isLoading) return <LoadingState label="Loading article" py={10} />;

  if (articleQuery.isError || !article) {
    return (
      <Container maxWidth="lg" sx={{ py: 8 }}>
        <ArticleErrorTitle />
        <QueryErrorState
          headingComponent="h1"
          error={articleQuery.error}
          message="The article could not be loaded."
          onRetry={isClientError(articleQuery.error) ? undefined : () => void articleQuery.refetch()}
        />
        <Stack alignItems="center">
          <Button component={Link} to="/" variant="outlined" startIcon={<ArrowLeft size={16} aria-hidden />}>
            Back to knowledge base
          </Button>
        </Stack>
      </Container>
    );
  }

  return (
    <ArticleView
      article={article}
      knowledgeBaseTitle={knowledgeBaseTitle}
      relatedArticles={related?.articles ?? []}
    />
  );
}

/** Title for the load-failure state. Rendered only in that state, so nothing competes with it. */
function ArticleErrorTitle(): null {
  useDocumentTitle("Article could not be loaded");
  return null;
}

interface ArticleViewProps {
  article: KBArticle;
  knowledgeBaseTitle?: string;
  relatedArticles: RelatedArticle[];
}

function ArticleView({ article, knowledgeBaseTitle, relatedArticles }: ArticleViewProps): JSX.Element {
  useDocumentTitle(article.title);
  const theme = useTheme();
  const wideLayout = useMediaQuery(theme.breakpoints.up("md"));
  const [notice, setNotice] = useState<string | null>(null);

  const outline = useMemo(() => buildArticleOutline(sanitizeArticleHtml(article.body)), [article.body]);
  const readingTime = useMemo(() => formatReadingTime(readingTimeMinutes(toPlainText(article.body))), [article.body]);

  const published = formatDate(article.publishedOn);
  const updated = formatRelativeDate(article.updatedOn);
  // "Updated" only when it adds information: a same-day edit after publishing is noise.
  const showUpdated =
    !!updated &&
    (!article.publishedOn || new Date(article.updatedOn).getTime() - new Date(article.publishedOn).getTime() > DAY_MS);

  // The copy buttons live inside the article HTML, so one delegated handler
  // serves all of them.
  const onBodyClick = (event: MouseEvent<HTMLDivElement>): void => {
    const button = (event.target as HTMLElement).closest<HTMLButtonElement>(`[${CODE_COPY_ATTR}]`);
    if (!button) return;
    const code = button.parentElement?.querySelector("pre")?.textContent ?? "";
    // navigator.clipboard is undefined outside a secure context (plain http),
    // so the call itself can throw: catch synchronously as well as the promise.
    void (async () => {
      try {
        await navigator.clipboard.writeText(code);
        button.textContent = "Copied";
        window.setTimeout(() => {
          button.textContent = "Copy";
        }, 2000);
        setNotice("Code copied to clipboard");
      } catch {
        setNotice("The code could not be copied");
      }
    })();
  };

  const rail = (
    <Stack spacing={4}>
      {wideLayout && <TableOfContents headings={outline.headings} />}
      {relatedArticles.length > 0 && (
        <Box component="aside" aria-labelledby="related-heading">
          <Typography
            id="related-heading"
            variant="overline"
            component="h2"
            color="text.secondary"
            sx={{ display: "block", mb: 1, letterSpacing: 1 }}
          >
            Related articles
          </Typography>
          <ArticleList label="Related articles">
            {relatedArticles.map((a) => (
              <ArticleListItem key={a.id}>
                <ArticleCard id={a.id} title={a.title} compact />
              </ArticleListItem>
            ))}
          </ArticleList>
        </Box>
      )}
      <SupportCallout compact />
    </Stack>
  );

  return (
    <Container maxWidth="xl" sx={{ py: { xs: 3, md: 5 } }}>
      <Breadcrumbs aria-label="Breadcrumb" className="kb-no-print" sx={{ mb: 3 }}>
        <OxygenLink component={Link} to="/" underline="hover" color="inherit">
          Knowledge Base
        </OxygenLink>
        {knowledgeBaseTitle && (
          <OxygenLink
            component={Link}
            to={browseUrl({ knowledgeBaseId: article.knowledgeBaseId })}
            underline="hover"
            color="inherit"
          >
            {knowledgeBaseTitle}
          </OxygenLink>
        )}
        <Typography color="text.primary" variant="body2" aria-current="page" sx={{ overflowWrap: "anywhere" }}>
          {article.title}
        </Typography>
      </Breadcrumbs>

      <Grid container spacing={{ xs: 4, md: 5 }}>
        <Grid size={{ xs: 12, md: 8, lg: 9 }}>
          <Paper component="article" variant="outlined" sx={{ p: { xs: 2.5, sm: 4, md: 5 }, borderRadius: 2 }}>
            <Stack
              direction="row"
              alignItems="center"
              flexWrap="wrap"
              useFlexGap
              sx={{ gap: 1, mb: 2, color: "text.secondary" }}
            >
              {knowledgeBaseTitle && (
                <Chip
                  size="small"
                  variant="outlined"
                  color="primary"
                  icon={<Layers size={14} aria-hidden />}
                  label={knowledgeBaseTitle}
                  component={Link}
                  to={browseUrl({ knowledgeBaseId: article.knowledgeBaseId })}
                  clickable
                />
              )}
              {article.number && (
                <Typography variant="caption" sx={{ fontFamily: "monospace", letterSpacing: 0.5 }}>
                  {article.number}
                </Typography>
              )}
            </Stack>
            <Typography
              variant="h3"
              component="h1"
              sx={{ fontWeight: 750, letterSpacing: "-0.01em", lineHeight: 1.2, fontSize: { xs: "1.75rem", md: "2.25rem" }, mb: 2 }}
            >
              {article.title}
            </Typography>
            <Stack
              direction={{ xs: "column", sm: "row" }}
              justifyContent="space-between"
              alignItems={{ xs: "flex-start", sm: "center" }}
              spacing={1.5}
              useFlexGap
            >
              <Stack
                direction="row"
                alignItems="center"
                flexWrap="wrap"
                useFlexGap
                sx={{ gap: 2, color: "text.secondary", typography: "body2" }}
              >
                {published && article.publishedOn && (
                  <span>
                    Published <time dateTime={article.publishedOn}>{published}</time>
                  </span>
                )}
                {showUpdated && updated && (
                  <span>
                    Updated{" "}
                    <time dateTime={updated.iso} title={updated.absolute}>
                      {updated.label}
                    </time>
                  </span>
                )}
                {readingTime && (
                  <Box component="span" sx={{ display: "inline-flex", alignItems: "center", gap: 0.5 }}>
                    <Clock size={14} aria-hidden />
                    {readingTime}
                  </Box>
                )}
              </Stack>
              <ArticleActions onNotify={setNotice} />
            </Stack>
            <Divider sx={{ my: 3 }} />

            {!wideLayout && outline.headings.length >= 2 && (
              <Accordion
                variant="outlined"
                disableGutters
                className="kb-no-print"
                sx={{ mb: 3, borderRadius: 1.5, "&::before": { display: "none" } }}
              >
                <AccordionSummary expandIcon={<ChevronDown size={18} aria-hidden />}>
                  <Typography variant="subtitle2">On this page</Typography>
                </AccordionSummary>
                <AccordionDetails sx={{ pt: 0 }}>
                  <TableOfContents headings={outline.headings} />
                </AccordionDetails>
              </Accordion>
            )}

            <Box sx={bodySx} onClick={onBodyClick} dangerouslySetInnerHTML={{ __html: outline.html }} />

            <Divider sx={{ mt: 5, mb: 3 }} className="kb-no-print" />
            <Stack
              direction={{ xs: "column", sm: "row" }}
              spacing={1.5}
              alignItems={{ sm: "center" }}
              justifyContent="space-between"
              className="kb-no-print"
            >
              <Typography variant="body2" color="text.secondary">
                {knowledgeBaseTitle ? `Part of the ${knowledgeBaseTitle} knowledge base.` : "Part of the WSO2 Knowledge Base."}
              </Typography>
              <Button
                component={Link}
                to={knowledgeBaseTitle ? browseUrl({ knowledgeBaseId: article.knowledgeBaseId }) : browseUrl()}
                variant="text"
                startIcon={<ArrowLeft size={16} aria-hidden />}
                sx={{ flexShrink: 0 }}
              >
                {knowledgeBaseTitle ? `More ${knowledgeBaseTitle} articles` : "Browse all articles"}
              </Button>
            </Stack>
          </Paper>
        </Grid>

        <Grid size={{ xs: 12, md: 4, lg: 3 }} className="kb-no-print">
          <Box sx={{ position: { md: "sticky" }, top: { md: 88 } }}>{rail}</Box>
        </Grid>
      </Grid>

      <Snackbar
        open={notice !== null}
        autoHideDuration={2500}
        onClose={() => setNotice(null)}
        message={notice}
        anchorOrigin={{ vertical: "bottom", horizontal: "center" }}
      />
    </Container>
  );
}
