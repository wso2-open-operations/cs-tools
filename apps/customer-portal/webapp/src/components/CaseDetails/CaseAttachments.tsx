import React from "react";
import { Box, Typography, Paper, Grid } from "@mui/material";
import type { Attachment } from "@/types/case.types";
import { FileTextIcon } from "@/assets/icons/common-icons";

interface CaseAttachmentsProps {
  attachments: Attachment[];
}

export const CaseAttachments: React.FC<CaseAttachmentsProps> = ({
  attachments,
}) => {
  if (attachments.length === 0) {
    return (
      <Box sx={{ p: 4, textAlign: "center", color: "#94A3B8" }}>
        <Typography>No attachments found.</Typography>
      </Box>
    );
  }

  return (
    <Grid container spacing={2}>
      {attachments.map((attachment) => (
        <Grid size={{ xs: 12, sm: 6, md: 4 }} key={attachment.sysId}>
          <Paper
            variant="outlined"
            sx={{
              p: 2,
              display: "flex",
              alignItems: "center",
              gap: 2,
              cursor: "pointer",
              "&:hover": { borderColor: "#3B82F6", bgcolor: "#EFF6FF" },
            }}
          >
            <Box
              sx={{
                p: 1,
                bgcolor: "#F1F5F9",
                borderRadius: 1,
                color: "#64748B",
              }}
            >
              <FileTextIcon width={24} height={24} />
            </Box>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography
                variant="body2"
                sx={{
                  fontWeight: 500,
                  color: "#0F172A",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
              >
                {attachment.name}
              </Typography>
              <Typography variant="caption" sx={{ color: "#64748B" }}>
                {(attachment.sizeBytes / 1024).toFixed(1)} KB •{" "}
                {new Date(attachment.uploadedDate).toLocaleDateString()}
              </Typography>
            </Box>
            {/* <IconButton size="small">
                    <DownloadIcon width={16} height={16} /> 
                </IconButton> */}
          </Paper>
        </Grid>
      ))}
    </Grid>
  );
};
