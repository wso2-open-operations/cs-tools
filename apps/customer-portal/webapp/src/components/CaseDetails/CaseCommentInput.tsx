import React from "react";
import { Box, TextField, Button, Avatar } from "@mui/material";
import { PaperclipIcon } from "../../assets/icons/support/PaperclipIcon";
import { SendIcon } from "../../assets/icons/support/SendIcon";


interface CaseCommentInputProps {
  onSendComment?: (comment: string) => void;
  disabled?: boolean;
}

export const CaseCommentInput: React.FC<CaseCommentInputProps> = ({
  onSendComment,
  disabled = false,
}) => {
  const [comment, setComment] = React.useState("");

  const handleSend = () => {
    if (comment.trim() && onSendComment) {
      onSendComment(comment);
      setComment("");
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.ctrlKey && e.key === "Enter") {
      handleSend();
    }
  };

  return (
    <Box
      sx={{
        borderTop: "1px solid #E2E8F0",
        bgcolor: "white",
      }}
    >
      <Box sx={{ p: 1.5, maxWidth: "1280px", mx: "auto" }}>
        <Box sx={{ display: "flex", gap: 1.25 }}>
          {/* Avatar */}
          <Avatar
            sx={{
              width: 32,
              height: 32,
              bgcolor: "#FFEDD5",
              color: "#C2410C",
              fontSize: "0.75rem",
              flexShrink: 0,
            }}
          >
            YO
          </Avatar>

          {/* Input Area */}
          <Box
            sx={{
              flex: 1,
              display: "flex",
              flexDirection: "column",
              gap: 0.75,
            }}
          >
            {/* Textarea */}
            <TextField
              multiline
              minRows={3}
              maxRows={10}
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Add a comment... You can include error logs, stack traces, or detailed explanations."
              disabled={disabled}
              sx={{
                "& .MuiOutlinedInput-root": {
                  fontSize: "0.75rem",
                  bgcolor: "#F8FAFC",
                  borderColor: "#E2E8F0",
                  borderRadius: "6px",
                  px: 1.5,
                  py: 1,
                  "&:hover .MuiOutlinedInput-notchedOutline": {
                    borderColor: "#CBD5E1",
                  },
                  "&.Mui-focused .MuiOutlinedInput-notchedOutline": {
                    borderColor: "#EA580C",
                    borderWidth: "2px",
                  },
                },
                "& .MuiOutlinedInput-notchedOutline": {
                  borderColor: "#E2E8F0",
                },
                "& .MuiInputBase-input": {
                  fontSize: "0.75rem",
                  color: "#0F172A",
                  "&::placeholder": {
                    color: "#94A3B8",
                    opacity: 1,
                  },
                },
              }}
            />

            {/* Action Row */}
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
              }}
            >
              {/* Left side - Attach files button and hint */}
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Button
                  size="small"
                  startIcon={<PaperclipIcon width={14} height={14} />}
                  sx={{
                    color: "#64748B",
                    textTransform: "none",
                    fontSize: "0.75rem",
                    px: 1.5,
                    py: 0.5,
                    height: 28,
                    minWidth: 0,
                    "&:hover": {
                      bgcolor: "#F1F5F9",
                      color: "#0F172A",
                    },
                  }}
                >
                  Attach files
                </Button>
                <Box
                  component="span"
                  sx={{ fontSize: "0.75rem", color: "#6B7280" }}
                >
                  Ctrl+Enter to send
                </Box>
              </Box>

              {/* Right side - Send button */}
              <Button
                variant="contained"
                size="small"
                startIcon={<SendIcon width={14} height={14} />}
                disabled={!comment.trim() || disabled}
                onClick={handleSend}
                sx={{
                  bgcolor: "#EA580C",
                  color: "white",
                  textTransform: "none",
                  fontSize: "0.75rem",
                  px: 1.5,
                  height: 32,
                  minWidth: 0,
                  "&:hover": {
                    bgcolor: "#C2410C",
                  },
                  "&.Mui-disabled": {
                    bgcolor: "#E2E8F0",
                    color: "#94A3B8",
                  },
                }}
              >
                Send Comment
              </Button>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
};
