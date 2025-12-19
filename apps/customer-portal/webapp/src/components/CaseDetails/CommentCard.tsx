import React, { useState } from "react";
import {
  Box,
  Typography,
  Avatar,
  Button,
  Paper,
  Chip,
  Divider,
} from "@mui/material";
import type { Comment } from "@/types/case.types";
import {
  ChevronDownIcon,
  ChevronUpIcon,
  BotIcon,
} from "@/assets/icons/common-icons";

interface CommentCardProps {
  comment: Comment;
}

function parseCommentContent(content: string): {
  isHtml: boolean;
  renderedContent: string;
} {
  const codeTagRegex = /\[code\](.+?)\[\/code\]/gs;
  const match = codeTagRegex.exec(content);

  if (match && match[1]) {
    let htmlContent = match[1].trim();
    // Fix image paths if necessary
    htmlContent = htmlContent.replace(
      /src="\/([^"]+\.iix)"/g,
      'src="https://wso2sndev.wso2.com/$1"'
    );
    return { isHtml: true, renderedContent: htmlContent };
  }

  return { isHtml: false, renderedContent: content };
}

export const CommentCard: React.FC<CommentCardProps> = ({ comment }) => {
  const [expanded, setExpanded] = useState(false);

  const isCustomer = comment.author.role === "customer";
  const isSystem = comment.author.name === "System";
  const isNoveraAI = comment.author.name === "Novera AI";

  // Check if content is long
  const contentLines = comment.content.split("\n").length;
  const shouldTruncate = !expanded && (comment.isLarge || contentLines > 4);
  const { isHtml, renderedContent } = parseCommentContent(comment.content);

  // --- Styles mimicking Tailwind classes ---

  // Customer: bg-orange-100 text-orange-700
  // Support: bg-blue-100 text-blue-700
  const avatarBg = isCustomer ? "#ffedd5" : "#dbeafe";
  const avatarColor = isCustomer ? "#c2410c" : "#1d4ed8";

  // Customer Card: bg-orange-50 border-orange-200
  // Support Card: bg-white (border default gray)
  const cardBg = isCustomer ? "#fff7ed" : "#ffffff";
  const cardBorderColor = isCustomer ? "#fed7aa" : "#e5e7eb";

  if (isSystem) {
    // Keeping system messages simple and centered
    return (
      <Box sx={{ display: "flex", alignItems: "center", gap: 2, my: 1 }}>
        <Divider sx={{ flex: 1 }} />
        <Chip
          label={comment.content}
          size="small"
          sx={{
            bgcolor: "#F3F4F6",
            color: "#6B7280",
            fontSize: "0.75rem",
            height: 24,
          }}
        />
        <Divider sx={{ flex: 1 }} />
      </Box>
    );
  }

  return (
    <Box
      sx={{
        display: "flex",
        gap: 1.5, // gap-3 (12px)
        flexDirection: isCustomer ? "row-reverse" : "row",
        width: "100%",
      }}
    >
      {/* Avatar Section */}
      <Avatar
        sx={{
          width: 32, // h-8
          height: 32, // w-8
          bgcolor: isNoveraAI ? "#FFEDD5" : avatarBg,
          color: isNoveraAI ? "#C2410C" : avatarColor,
          fontSize: "0.75rem",
          flexShrink: 0,
        }}
      >
        {isNoveraAI ? (
          <BotIcon width={16} height={16} />
        ) : (
          comment.author.name.substring(0, 2).toUpperCase()
        )}
      </Avatar>

      {/* Content Column */}
      <Box
        sx={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          gap: 0.75, // space-y-1.5 (6px)
          alignItems: isCustomer ? "flex-end" : "flex-start",
          maxWidth: "85%", // prevent bubble from stretching too wide on large screens
        }}
      >
        {/* Metadata Row (Name, Date, Badge) */}
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1,
            flexDirection: isCustomer ? "row-reverse" : "row",
          }}
        >
          <Typography
            sx={{
              fontSize: "0.75rem", // text-xs
              color: "#111827", // text-gray-900
              fontWeight: 500,
            }}
          >
            {isCustomer ? "You" : comment.author.name}
          </Typography>
          <Typography
            sx={{
              fontSize: "0.75rem", // text-xs
              color: "#6B7280", // text-gray-500
            }}
          >
            {new Date(comment.timestamp).toLocaleString("en-US", {
              month: "short",
              day: "numeric",
              hour: "2-digit",
              minute: "2-digit",
            })}
          </Typography>

          {/* Badge for Engineer */}
          {comment.author.role === "engineer" && !isNoveraAI && (
            <Chip
              label="Support Engineer"
              size="small"
              variant="outlined"
              sx={{
                height: 20,
                fontSize: "0.65rem", // text-xs
                bgcolor: "#f3f4f6", // bg-secondary
                color: "#1f2937", // text-secondary-foreground
                borderColor: "transparent", // border-transparent
                fontWeight: 500,
                borderRadius: "6px", // rounded-md
                "& .MuiChip-label": { px: 0.75 },
              }}
            />
          )}
        </Box>

        {/* Message Card */}
        <Paper
          elevation={0}
          variant="outlined"
          sx={{
            p: 1.5, // p-3
            bgcolor: cardBg,
            borderColor: cardBorderColor,
            borderRadius: 3, // rounded-xl
            width: "100%",
            display: "flex",
            flexDirection: "column",
            gap: 2, // gap-6 (24px) - gap between content and button
          }}
        >
          {/* Content Text */}
          <Box
            sx={{
              fontSize: "0.875rem", // text-sm (increased from text-xs)
              color: "#334155", // text-card-foreground equivalent
              fontFamily: comment.hasCode ? "monospace" : "inherit",
              // Truncation logic matching line-clamp-4
              ...(shouldTruncate && {
                overflow: "hidden",
                display: "-webkit-box",
                WebkitBoxOrient: "vertical",
                WebkitLineClamp: 4,
              }),
              "& img": {
                maxWidth: "100%",
                height: "auto",
                borderRadius: 1,
                border: "1px solid #E2E8F0",
                mt: 1,
              },
            }}
          >
            {isHtml ? (
              <div dangerouslySetInnerHTML={{ __html: renderedContent }} />
            ) : (
              <Typography
                variant="body2"
                sx={{
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-word",
                  fontSize: "inherit",
                  fontFamily: "inherit",
                }}
              >
                {renderedContent}
              </Typography>
            )}
          </Box>

          {/* Show More / Show Less Button */}
          {(comment.isLarge || contentLines > 4) && (
            <Box sx={{ width: "100%" }}>
              <Divider sx={{ mb: 1 }} />
              <Button
                size="small"
                onClick={() => setExpanded(!expanded)}
                endIcon={
                  // HTML had icon on right, though input React had it on left.
                  // HTML SVG is chevron-down.
                  expanded ? (
                    <ChevronUpIcon width={14} height={14} />
                  ) : (
                    <ChevronDownIcon width={14} height={14} />
                  )
                }
                sx={{
                  textTransform: "none",
                  color: "#4B5563", // text-gray-600
                  fontSize: "0.75rem", // text-xs
                  fontWeight: 500,
                  width: "100%",
                  height: 28, // h-7
                  justifyContent: "center",
                  "&:hover": {
                    bgcolor: "#F3F4F6", // hover:bg-accent
                    color: "#111827", // hover:text-gray-900
                  },
                }}
              >
                {expanded ? "Show less" : "Show more"}
              </Button>
            </Box>
          )}
        </Paper>
      </Box>
    </Box>
  );
};
