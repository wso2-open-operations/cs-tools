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
  Alert,
  Box,
  Button,
  Paper,
  Skeleton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import {
  Download,
  File,
  FileArchive,
  FileText,
  Image,
  Paperclip,
} from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import useGetCaseAttachments from "@api/useGetCaseAttachments";
import type { CaseAttachment } from "@models/responses";
import { formatFileSize, getAttachmentFileCategory } from "@utils/support";
import UploadAttachmentModal from "@case-details-attachments/UploadAttachmentModal";

const INITIAL_LIMIT = 50;

export interface CaseDetailsAttachmentsPanelProps {
  caseId: string;
}

function getAttachmentIcon(att: CaseAttachment): JSX.Element {
  const category = getAttachmentFileCategory(att.name ?? "", att.type ?? "");
  switch (category) {
    case "image":
      return <Image size={20} aria-hidden />;
    case "pdf":
      return <FileText size={20} aria-hidden />;
    case "text":
      return <FileText size={20} aria-hidden />;
    case "archive":
      return <FileArchive size={20} aria-hidden />;
    default:
      return <File size={20} aria-hidden />;
  }
}

/**
 * Renders the Attachments tab: upload button, modal, and list from GET /cases/:id/attachments.
 * Fetches first 50, then remaining when totalRecords > 50; shows skeleton until all loaded.
 *
 * @param {CaseDetailsAttachmentsPanelProps} props - caseId.
 * @returns {JSX.Element} The attachments panel.
 */
export default function CaseDetailsAttachmentsPanel({
  caseId,
}: CaseDetailsAttachmentsPanelProps): JSX.Element {
  const [uploadOpen, setUploadOpen] = useState(false);

  const first = useGetCaseAttachments(caseId, { limit: INITIAL_LIMIT, offset: 0 });
  const totalRecords = first.data?.totalRecords ?? 0;
  const needMore = totalRecords > INITIAL_LIMIT;
  const secondEnabled =
    !!caseId && needMore && !!first.data && totalRecords > INITIAL_LIMIT;
  const second = useGetCaseAttachments(caseId, {
    limit: Math.max(0, totalRecords - INITIAL_LIMIT),
    offset: INITIAL_LIMIT,
    enabled: secondEnabled,
  });

  const allAttachments = useMemo(() => {
    if (!first.data) return [];
    if (!needMore) return first.data.attachments;
    if (!second.data) return first.data.attachments;
    return [...first.data.attachments, ...second.data.attachments];
  }, [first.data, needMore, second.data]);

  const combinedLength =
    (first.data?.attachments?.length ?? 0) +
    (second.data?.attachments?.length ?? 0);
  const hasPartialError =
    secondEnabled &&
    (second.isError ||
      (first.data != null &&
        totalRecords > 0 &&
        first.data.totalRecords !== combinedLength));

  const isLoading =
    first.isLoading ||
    (secondEnabled &&
      !second.isError &&
      (second.isLoading || !second.data));

  const handleDownload = (att: CaseAttachment) => {
    if (att.downloadUrl) {
      window.open(att.downloadUrl, "_blank", "noopener,noreferrer");
    }
  };

  if (!caseId) {
    return (
      <Box sx={{ border: 1, borderColor: "divider", p: 2 }}>
        <Typography variant="body2" color="text.secondary">
          No case selected.
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ border: 1, borderColor: "divider", p: 2 }}>
      <Stack spacing={3}>
        <Button
          variant="contained"
          color="primary"
          startIcon={<Paperclip size={16} aria-hidden />}
          sx={{ alignSelf: "flex-start" }}
          onClick={() => setUploadOpen(true)}
        >
          Upload Attachment
        </Button>

        {isLoading ? (
          <Stack spacing={2}>
            {[1, 2, 3, 4].map((i) => (
              <Paper
                key={i}
                variant="outlined"
                sx={{
                  p: 2,
                  display: "flex",
                  alignItems: "center",
                  gap: 2,
                }}
              >
                <Skeleton variant="rectangular" width={40} height={40} />
                <Box sx={{ flex: 1 }}>
                  <Skeleton variant="text" width="60%" height={24} />
                  <Skeleton variant="text" width="40%" height={16} sx={{ mt: 0.5 }} />
                </Box>
                <Skeleton
                  variant="rectangular"
                  width={100}
                  height={32}
                  sx={{ borderRadius: 1 }}
                />
              </Paper>
            ))}
          </Stack>
        ) : first.isError ? (
          <Typography variant="body2" color="error">
            Failed to load attachments.
          </Typography>
        ) : allAttachments.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            No attachments yet.
          </Typography>
        ) : (
          <Stack spacing={2}>
            {hasPartialError && (
              <Alert severity="warning" sx={{ mb: 1 }}>
                Some attachments may not be shown. The list may be incomplete due
                to a load error or API limits.
              </Alert>
            )}
            {allAttachments.map((att) => (
              <Paper
                key={att.id}
                variant="outlined"
                sx={{
                  p: 2,
                  display: "flex",
                  alignItems: "center",
                  gap: 2,
                  "&:hover": {
                    bgcolor: "action.hover",
                  },
                }}
              >
                <Box
                  sx={{
                    width: 40,
                    height: 40,
                    bgcolor: "action.hover",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    color: "text.secondary",
                    flexShrink: 0,
                  }}
                  aria-hidden
                >
                  {getAttachmentIcon(att)}
                </Box>
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography variant="body2" color="text.primary" noWrap>
                    {att.name}
                  </Typography>
                  <Stack
                    direction="row"
                    spacing={1}
                    alignItems="center"
                    flexWrap="wrap"
                    sx={{ mt: 0.5 }}
                  >
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      component="span"
                    >
                      {formatFileSize(att.size ?? att.sizeBytes)}
                    </Typography>
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      component="span"
                    >
                      •
                    </Typography>
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      component="span"
                    >
                      Uploaded by {att.createdBy}
                    </Typography>
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      component="span"
                    >
                      •
                    </Typography>
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      component="span"
                    >
                      {att.createdOn}
                    </Typography>
                  </Stack>
                </Box>
                <Button
                  variant="outlined"
                  size="small"
                  startIcon={<Download size={16} aria-hidden />}
                  onClick={() => handleDownload(att)}
                >
                  Download
                </Button>
              </Paper>
            ))}
          </Stack>
        )}
      </Stack>

      <UploadAttachmentModal
        open={uploadOpen}
        caseId={caseId}
        onClose={() => setUploadOpen(false)}
      />
    </Box>
  );
}
