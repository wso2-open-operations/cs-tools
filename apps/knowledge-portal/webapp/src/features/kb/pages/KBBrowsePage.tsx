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
  Divider,
  Grid,
  List,
  ListItemButton,
  ListItemText,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Eye, Search } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useState, type FormEvent, type JSX } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useKnowledgeBases, useSearchArticles } from "@features/kb/api/useKb";

const toSnippet = (body: string, max = 180): string => {
  const plain = body
    .replace(/<[^>]+>/g, " ")
    .replace(/[#*_`>[\]()]/g, "")
    .replace(/\s+/g, " ")
    .trim();
  return plain.length > max ? `${plain.slice(0, max)}...` : plain;
};

export default function KBBrowsePage(): JSX.Element {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();

  const knowledgeBaseId = params.get("knowledgeBaseId") ?? "";
  const searchQuery = params.get("q") ?? "";
  const [draftQuery, setDraftQuery] = useState(searchQuery);

  useEffect(() => {
    setDraftQuery(searchQuery);
  }, [searchQuery]);

  const { data: kbData } = useKnowledgeBases();
  const { data, isLoading, isError } = useSearchArticles({
    ...(knowledgeBaseId ? { knowledgeBaseId } : {}),
    ...(searchQuery ? { searchQuery } : {}),
    limit: 20,
  });

  const knowledgeBases = kbData?.knowledgeBases ?? [];
  const articles = data?.articles ?? [];

  const onSearch = (e: FormEvent) => {
    e.preventDefault();
    const next = new URLSearchParams(params);
    const trimmed = draftQuery.trim();
    if (trimmed) {
      next.set("q", trimmed);
    } else {
      next.delete("q");
    }
    setParams(next);
  };

  const selectKnowledgeBase = (id: string) => {
    const next = new URLSearchParams(params);
    if (id) {
      next.set("knowledgeBaseId", id);
    } else {
      next.delete("knowledgeBaseId");
    }
    setParams(next);
  };

  return (
    <Container maxWidth="lg" sx={{ py: 5 }}>
      <Box component="form" onSubmit={onSearch} sx={{ mb: 4 }}>
        <TextField
          fullWidth
          value={draftQuery}
          onChange={(e) => setDraftQuery(e.target.value)}
          placeholder="Search articles..."
          slotProps={{
            input: {
              startAdornment: (
                <Search size={18} style={{ marginRight: 8, opacity: 0.6 }} />
              ),
            },
          }}
        />
      </Box>

      <Grid container spacing={4}>
        <Grid size={{ xs: 12, md: 3 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1 }}>
            Knowledge bases
          </Typography>
          <Divider sx={{ mb: 1 }} />
          <List dense disablePadding>
            <ListItemButton
              selected={!knowledgeBaseId}
              onClick={() => selectKnowledgeBase("")}
              sx={{ borderRadius: 1 }}
            >
              <ListItemText primary="All" />
            </ListItemButton>
            {knowledgeBases.map((kb) => (
              <ListItemButton
                key={kb.id}
                selected={kb.id === knowledgeBaseId}
                onClick={() => selectKnowledgeBase(kb.id)}
                sx={{ borderRadius: 1 }}
              >
                <ListItemText primary={kb.title} />
              </ListItemButton>
            ))}
          </List>
        </Grid>

        <Grid size={{ xs: 12, md: 9 }}>
          {isLoading && (
            <Stack alignItems="center" sx={{ py: 8 }}>
              <CircularProgress size={28} />
            </Stack>
          )}

          {isError && (
            <Typography color="error" variant="body2">
              The knowledge base is temporarily unavailable. Please try again
              later.
            </Typography>
          )}

          {!isLoading && !isError && (
            <>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                {data?.total ?? 0} article{(data?.total ?? 0) === 1 ? "" : "s"}
              </Typography>

              {articles.length === 0 && (
                <Typography variant="body2" color="text.secondary">
                  No articles matched. Try a different search or knowledge base.
                </Typography>
              )}

              <Stack spacing={1.5}>
                {articles.map((article) => (
                  <Card key={article.id} variant="outlined">
                    <CardActionArea
                      onClick={() => navigate(`/articles/${article.id}`)}
                      sx={{ p: 2.5, alignItems: "flex-start" }}
                    >
                      <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                        {article.title}
                      </Typography>
                      <Typography
                        variant="body2"
                        color="text.secondary"
                        sx={{ mt: 0.5 }}
                      >
                        {toSnippet(article.body)}
                      </Typography>
                      <Stack
                        direction="row"
                        spacing={1.5}
                        alignItems="center"
                        sx={{ mt: 1, color: "text.secondary" }}
                      >
                        {article.viewCount != null && (
                          <Stack direction="row" spacing={0.5} alignItems="center">
                            <Eye size={13} />
                            <Typography variant="caption">
                              {article.viewCount} views
                            </Typography>
                          </Stack>
                        )}
                        {article.publishedOn && (
                          <Typography variant="caption">
                            {new Date(article.publishedOn).toLocaleDateString()}
                          </Typography>
                        )}

                      </Stack>
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
