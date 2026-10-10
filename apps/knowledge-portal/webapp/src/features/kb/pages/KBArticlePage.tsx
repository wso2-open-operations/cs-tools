// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.

import {
  Box,
  Breadcrumbs,
  Button,
  Card,
  CardActionArea,
  Chip,
  CircularProgress,
  Container,
  Divider,
  Grid,
  Link as OxygenLink,
  Paper,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, Eye } from "@wso2/oxygen-ui-icons-react";
import DOMPurify from "dompurify";
import { useMemo, type JSX } from "react";
import { Link, useNavigate, useParams } from "react-router";
import {
  useArticle,
  useKnowledgeBases,
  useRecordArticleView,
  useRelatedArticles,
} from "@features/kb/api/useKb";

export default function KBArticlePage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const { data: article, isLoading, isError } = useArticle(id);

  useRecordArticleView(article?.id);
  const { data: kbData } = useKnowledgeBases();
  const { data: related } = useRelatedArticles(article?.id);

  const knowledgeBaseTitle = useMemo(() => {
    if (!article) return undefined;
    return kbData?.knowledgeBases.find((kb) => kb.id === article.knowledgeBaseId)
      ?.title;
  }, [article, kbData]);

  const sanitizedBody = useMemo(
    () => (article ? DOMPurify.sanitize(article.body) : ""),
    [article],
  );

  const relatedArticles = related?.articles ?? [];

  if (isLoading) {
    return (
      <Stack alignItems="center" sx={{ py: 10 }}>
        <CircularProgress size={28} />
      </Stack>
    );
  }

  if (isError || !article) {
    return (
      <Container maxWidth="lg" sx={{ py: 8 }}>
        <Stack spacing={2} alignItems="flex-start">
          <Typography variant="h6">Article not found</Typography>
          <Typography variant="body2" color="text.secondary">
            This article may have been retired, or the link may be incorrect.
          </Typography>
          <Button
            variant="outlined"
            startIcon={<ArrowLeft size={16} />}
            onClick={() => navigate("/")}
          >
            Back to knowledge base
          </Button>
        </Stack>
      </Container>
    );
  }

  return (
    <Container maxWidth="xl" sx={{ py: 5 }}>
      <Breadcrumbs sx={{ mb: 3 }}>
        <OxygenLink component={Link} to="/" underline="hover" color="inherit">
          Knowledge Base
        </OxygenLink>
        {knowledgeBaseTitle && (
          <OxygenLink
            component={Link}
            to={`/browse?knowledgeBaseId=${article.knowledgeBaseId}`}
            underline="hover"
            color="inherit"
          >
            {knowledgeBaseTitle}
          </OxygenLink>
        )}
        <Typography color="text.primary" variant="body2">
          {article.title}
        </Typography>
      </Breadcrumbs>

      <Grid container spacing={5}>
        <Grid size={{ xs: 12, md: 9 }}>
          <Paper variant="outlined" sx={{ p: { xs: 3, md: 5 }, borderRadius: 2 }}>
            {article.number && (
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{ fontFamily: "monospace", mb: 1 }}
              >
                {article.number}
              </Typography>
            )}
            <Typography variant="h4" sx={{ fontWeight: 700, mb: 1.5 }}>
              {article.title}
            </Typography>
            <Stack
              direction="row"
              spacing={1}
              alignItems="center"
              flexWrap="wrap"
              sx={{ color: "text.secondary" }}
            >
              {knowledgeBaseTitle && (
                <Chip
                  size="small"
                  label={knowledgeBaseTitle}
                  variant="outlined"
                  onClick={() =>
                    navigate(`/browse?knowledgeBaseId=${article.knowledgeBaseId}`)
                  }
                />
              )}
              {article.publishedOn && (
                <Typography variant="body2">
                  Published {new Date(article.publishedOn).toLocaleDateString()}
                </Typography>
              )}
              {article.viewCount != null && (
                <Stack direction="row" spacing={0.5} alignItems="center">
                  <Eye size={14} />
                  <Typography variant="body2">
                    {article.viewCount} {article.viewCount === 1 ? "view" : "views"}
                  </Typography>
                </Stack>
              )}

            </Stack>
            <Divider sx={{ my: 3 }} />
            <Box
              sx={{
                maxWidth: "80ch",
                fontSize: "1.0625rem",
                lineHeight: 1.75,
                color: "text.primary",
                "& > *:first-of-type": { mt: 0 },
                "& h1, & h2, & h3, & h4": {
                  fontWeight: 600,
                  lineHeight: 1.3,
                  mt: 5,
                  mb: 1.5,
                  "& a.headerlink, & .headerlink": { display: "none" },
                },
                "& h1": { fontSize: "1.75rem" },
                "& h2": { fontSize: "1.4rem" },
                "& h3": { fontSize: "1.2rem" },
                "& h4": { fontSize: "1.05rem" },
                "& p": { my: 2 },
                "& ul, & ol": { my: 2, pl: 3 },
                "& li": { mb: 1 },
                "& li::marker": { color: "text.secondary" },
                "& a": { color: "primary.main" },
                "& code": {
                  px: 0.75,
                  py: 0.25,
                  borderRadius: 0.75,
                  bgcolor: "action.hover",
                  fontFamily: "monospace",
                  fontSize: "0.9em",
                },
                "& pre": {
                  overflowX: "auto",
                  p: 2,
                  my: 3,
                  borderRadius: 1,
                  bgcolor: "action.hover",
                  border: 1,
                  borderColor: "divider",
                  "& code": { bgcolor: "transparent", px: 0, py: 0 },
                },
                "& blockquote": {
                  my: 3,
                  ml: 0,
                  pl: 2,
                  borderLeft: 3,
                  borderColor: "divider",
                  color: "text.secondary",
                },
                "& img": { maxWidth: "100%", height: "auto", borderRadius: 1, my: 2 },
                "& table": {
                  width: "100%",
                  borderCollapse: "collapse",
                  my: 3,
                  fontSize: "0.9375rem",
                },
                "& td, & th": { border: 1, borderColor: "divider", p: 1.25 },
                "& th": { bgcolor: "action.hover", fontWeight: 600, textAlign: "left" },
              }}
              dangerouslySetInnerHTML={{ __html: sanitizedBody }}
            />
          </Paper>
        </Grid>

        <Grid size={{ xs: 12, md: 3 }}>
          {relatedArticles.length > 0 && (
            <>
              <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1.5 }}>
                Related articles
              </Typography>
              <Stack spacing={1.5}>
                {relatedArticles.map((a) => (
                  <Card key={a.id} variant="outlined">
                    <CardActionArea
                      onClick={() => navigate(`/articles/${a.id}`)}
                      sx={{ p: 2, alignItems: "flex-start" }}
                    >
                      <Typography variant="body2" sx={{ fontWeight: 500 }}>
                        {a.title}
                      </Typography>
                    </CardActionArea>
                  </Card>
                ))}
              </Stack>
            </>
          )}
        </Grid>
      </Grid>
    </Container>
  );
}
