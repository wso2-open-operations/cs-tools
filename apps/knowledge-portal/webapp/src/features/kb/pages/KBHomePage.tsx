// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.

import {
  Box,
  Card,
  CardActionArea,
  CircularProgress,
  Container,
  Grid,
  Link as OxygenLink,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowRight, BookOpen, Search } from "@wso2/oxygen-ui-icons-react";
import { useState, type FormEvent, type JSX } from "react";
import { Link, useNavigate } from "react-router";
import { useKnowledgeBases } from "@features/kb/api/useKb";
import ArticleSection from "@features/kb/components/ArticleSection";

// How many product cards to show on the landing page before the
// "view all" link. The rest are reachable from /browse.
const FEATURED_KB_LIMIT = 6;

export default function KBHomePage(): JSX.Element {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const { data, isLoading, isError } = useKnowledgeBases();

  const onSearch = (e: FormEvent) => {
    e.preventDefault();
    const trimmed = query.trim();
    navigate(trimmed ? `/browse?q=${encodeURIComponent(trimmed)}` : "/browse");
  };

  const allKnowledgeBases = data?.knowledgeBases ?? [];
  const featured = allKnowledgeBases.slice(0, FEATURED_KB_LIMIT);
  const hasMore = allKnowledgeBases.length > FEATURED_KB_LIMIT;

  return (
    <>
      {/* Hero: a compact, confident band with the search front and centre.
          Search is the primary action for a knowledge base, so there are
          no buttons -- the search box itself is the call to action. */}
      <Box
        sx={{
          borderBottom: 1,
          borderColor: "divider",
          bgcolor: "background.paper",
        }}
      >
        <Container maxWidth="md" sx={{ py: { xs: 6, md: 9 } }}>
          <Stack spacing={2.5} alignItems="center" textAlign="center">
            <Typography
              variant="h2"
              sx={{
                fontWeight: 800,
                lineHeight: 1.1,
                fontSize: { xs: "2rem", sm: "2.75rem", md: "3.25rem" },
                maxWidth: 800,
              }}
            >
              WSO2 Knowledge Hub
            </Typography>
            <Typography
              variant="h6"
              color="text.secondary"
              sx={{ fontWeight: 400, maxWidth: 660 }}
            >
              Welcome to the WSO2 Knowledge Hub. Search all knowledge bases or
              browse a specific one. See troubleshooting guides, how-to guides,
              FAQs (Frequently Asked Questions) and more.
            </Typography>

            <Box
              component="form"
              onSubmit={onSearch}
              sx={{ width: "100%", maxWidth: 680, mt: 2 }}
            >
              <TextField
                fullWidth
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search for a product, error, or how-to..."
                slotProps={{
                  input: {
                    startAdornment: (
                      <Search size={20} style={{ marginRight: 10, opacity: 0.6 }} />
                    ),
                    sx: { py: 0.5, fontSize: "1.05rem" },
                  },
                }}
              />
            </Box>
          </Stack>
        </Container>
      </Box>

      <Container maxWidth="lg" sx={{ py: { xs: 5, md: 7 } }}>
        {/* Browse by product */}
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          sx={{ mb: 2.5 }}
        >
          <Typography variant="h5" sx={{ fontWeight: 700 }}>
            Browse by product
          </Typography>
          {hasMore && (
            <OxygenLink
              component={Link}
              to="/browse"
              underline="hover"
              sx={{
                display: "inline-flex",
                alignItems: "center",
                gap: 0.5,
                fontWeight: 600,
              }}
            >
              View all knowledge bases
              <ArrowRight size={16} />
            </OxygenLink>
          )}
        </Stack>

        {isLoading && (
          <Stack alignItems="center" sx={{ py: 6 }}>
            <CircularProgress size={28} />
          </Stack>
        )}

        {isError && (
          <Typography color="error" variant="body2">
            The knowledge base is temporarily unavailable. Please try again later.
          </Typography>
        )}

        {!isLoading && !isError && featured.length === 0 && (
          <Typography variant="body2" color="text.secondary">
            No knowledge bases are available yet.
          </Typography>
        )}

        <Grid container spacing={2.5} sx={{ mb: 7 }}>
          {featured.map((kb) => (
            <Grid key={kb.id} size={{ xs: 12, sm: 6, md: 4 }}>
              <Card
                variant="outlined"
                sx={{
                  height: "100%",
                  transition: "border-color 120ms, transform 120ms",
                  "&:hover": {
                    borderColor: "primary.main",
                    transform: "translateY(-2px)",
                  },
                }}
              >
                <CardActionArea
                  onClick={() => navigate(`/browse?knowledgeBaseId=${kb.id}`)}
                  sx={{ p: 3, height: "100%", alignItems: "flex-start" }}
                >
                  <Stack spacing={1.5}>
                    <Box
                      sx={{
                        width: 40,
                        height: 40,
                        borderRadius: 1.5,
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        bgcolor: "action.hover",
                        color: "primary.main",
                      }}
                    >
                      <BookOpen size={20} />
                    </Box>
                    <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>
                      {kb.title}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      Browse articles and guides for {kb.title}.
                    </Typography>
                  </Stack>
                </CardActionArea>
              </Card>
            </Grid>
          ))}
        </Grid>

        {/* Popular / recent content */}
        <Typography variant="h5" sx={{ fontWeight: 700, mb: 2.5 }}>
          Popular right now
        </Typography>
        <Grid container spacing={4}>
          <Grid size={{ xs: 12, md: 4 }}>
            <ArticleSection title="Recent articles" sortBy="recent" />
          </Grid>
          <Grid size={{ xs: 12, md: 4 }}>
            <ArticleSection title="Most viewed" sortBy="mostViewed" metric="views" />
          </Grid>
          <Grid size={{ xs: 12, md: 4 }}>
            <ArticleSection title="Top rated" sortBy="topRated" metric="rating" />
          </Grid>
        </Grid>
      </Container>
    </>
  );
}
