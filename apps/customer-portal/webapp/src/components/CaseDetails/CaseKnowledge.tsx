import React from "react";
import { Box, Typography, Chip, Card } from "@mui/material";
import type { KBArticle } from "@/types/case.types";
import { BookOpenIcon } from "@/assets/icons/common-icons";

interface CaseKnowledgeProps {
  kbArticles: KBArticle[];
}

const suggestedByConfig: Record<
  string,
  { bg: string; text: string; border: string }
> = {
  AI: {
    bg: "#DBEAFE",
    text: "#1D4ED8",
    border: "#BFDBFE",
  },
  Engineer: {
    bg: "#DCFCE7",
    text: "#15803D",
    border: "#BBF7D0",
  },
  Customer: {
    bg: "#FEF3C7",
    text: "#A16207",
    border: "#FDE047",
  },
};

export const CaseKnowledge: React.FC<CaseKnowledgeProps> = ({ kbArticles }) => {
  if (kbArticles.length === 0) {
    return (
      <Box sx={{ p: 4, textAlign: "center", color: "#94A3B8" }}>
        <Typography>No knowledge base articles found.</Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {kbArticles.map((article) => {
        const config =
          suggestedByConfig[article.suggestedBy] || suggestedByConfig.AI;

        return (
          <Card
            key={article.id}
            sx={{
              p: 3,
              border: "1px solid #E5E7EB",
              borderRadius: "12px",
              boxShadow: "none",
              cursor: "pointer",
              transition: "all 0.2s",
              "&:hover": {
                borderColor: "#EA580C",
                bgcolor: "#FFF7ED",
                boxShadow: "0 2px 8px rgba(234, 88, 12, 0.1)",
              },
            }}
          >
            {/* Header */}
            <Box
              sx={{
                display: "flex",
                alignItems: "flex-start",
                justifyContent: "space-between",
                mb: 2,
              }}
            >
              <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
                <Box
                  sx={{
                    p: 0.75,
                    bgcolor: "#FFF7ED",
                    borderRadius: "8px",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                  }}
                >
                  <BookOpenIcon width={20} height={20} color="#EA580C" />
                </Box>
                <Typography
                  variant="h6"
                  sx={{
                    fontSize: "1rem",
                    fontWeight: 600,
                    color: "#111827",
                  }}
                >
                  {article.title}
                </Typography>
              </Box>
              <Chip
                label={`Suggested by ${article.suggestedBy}`}
                size="small"
                sx={{
                  bgcolor: config.bg,
                  color: config.text,
                  border: `1px solid ${config.border}`,
                  fontSize: "0.75rem",
                  fontWeight: 500,
                  height: "24px",
                }}
              />
            </Box>

            {/* Summary */}
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "#6B7280",
                mb: 2,
                lineHeight: 1.6,
              }}
            >
              {article.summary}
            </Typography>

            {/* Footer */}
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 2,
                pt: 2,
                borderTop: "1px solid #F3F4F6",
              }}
            >
              <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                <Typography
                  sx={{
                    fontSize: "0.75rem",
                    color: "#9CA3AF",
                  }}
                >
                  Category:
                </Typography>
                <Typography
                  sx={{
                    fontSize: "0.75rem",
                    color: "#374151",
                    fontWeight: 500,
                  }}
                >
                  {article.category}
                </Typography>
              </Box>
              <Box
                sx={{
                  width: 4,
                  height: 4,
                  borderRadius: "50%",
                  bgcolor: "#D1D5DB",
                }}
              />
              <Typography
                sx={{
                  fontSize: "0.75rem",
                  color: "#9CA3AF",
                }}
              >
                {article.views} views
              </Typography>
              <Box
                sx={{
                  width: 4,
                  height: 4,
                  borderRadius: "50%",
                  bgcolor: "#D1D5DB",
                }}
              />
              <Typography
                sx={{
                  fontSize: "0.75rem",
                  color: "#9CA3AF",
                }}
              >
                {new Date(article.suggestedDate).toLocaleDateString("en-US", {
                  month: "short",
                  day: "numeric",
                  year: "numeric",
                })}
              </Typography>
            </Box>
          </Card>
        );
      })}
    </Box>
  );
};
