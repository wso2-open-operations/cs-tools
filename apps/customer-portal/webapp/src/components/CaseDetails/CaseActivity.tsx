import React, { useRef } from "react";
import { Box, Typography, Divider, Button } from "@mui/material";
import { CommentCard } from "./CommentCard";
import type { Comment } from "../../types/case.types";
import { Maximize2Icon } from "../../assets/icons/support/Maximize2Icon";

interface CaseActivityProps {
  comments: Comment[];
  createdDate: string;
  height?: string | number;
}

export const CaseActivity: React.FC<CaseActivityProps> = ({
  comments,
  createdDate,
  height = "100%", // Default to filling parent or explicit height
}) => {
  const bottomRef = useRef<HTMLDivElement>(null);

  //Auto-scroll disabled per user request
//   useEffect(() => {
//     if (bottomRef.current) {
//       bottomRef.current.scrollIntoView({ behavior: "smooth" });
//     }
//   }, [comments]);

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        height: height,
        bgcolor: "#F8FAFC",
      }}
    >
      {/* Timeline Header */}
      <Box
        sx={{
          borderBottom: "1px solid #E2E8F0",
          bgcolor: "white",
          px: 2,
          py: 1,
        }}
      >
        <Box
          sx={{
            maxWidth: "1280px",
            mx: "auto",
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
          }}
        >
          <Typography
            variant="caption"
            sx={{ color: "#6B7280", fontSize: "0.75rem" }}
          >
            Activity timeline
          </Typography>
          <Button
            size="small"
            startIcon={<Maximize2Icon width={14} height={14} />}
            sx={{
              color: "#64748B",
              textTransform: "none",
              fontSize: "0.75rem",
              px: 1.5,
              height: 28,
              minWidth: 0,
              "&:hover": { bgcolor: "#F1F5F9", color: "#0F172A" },
            }}
          >
            Focus Mode
          </Button>
        </Box>
      </Box>

      {/* Scrollable Content */}
      <Box
        sx={{
          flex: 1,
          overflowY: "auto",
          bgcolor: "#F8FAFC",
        }}
      >
        <Box
          sx={{
            p: 1.5,
            maxWidth: "1280px",
            mx: "auto",
            display: "flex",
            flexDirection: "column",
            gap: 1.5,
          }}
        >
          {/* Date Divider */}
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 2,
              color: "#6B7280",
              fontSize: "0.75rem",
            }}
          >
            <Divider sx={{ flex: 1, borderColor: "#D1D5DB" }} />
            <Typography variant="caption" sx={{ color: "inherit" }}>
              Case created on {new Date(createdDate).toLocaleDateString()} at{" "}
              {new Date(createdDate).toLocaleTimeString([], {
                hour: "2-digit",
                minute: "2-digit",
              })}
            </Typography>
            <Divider sx={{ flex: 1, borderColor: "#D1D5DB" }} />
          </Box>

          {comments.map((comment) => (
            <CommentCard key={comment.id} comment={comment} />
          ))}
          <div ref={bottomRef} />
        </Box>
      </Box>
    </Box>
  );
};
