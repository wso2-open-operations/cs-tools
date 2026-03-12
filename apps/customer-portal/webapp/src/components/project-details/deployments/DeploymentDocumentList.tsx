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

import type { DeploymentDocument } from "@models/responses";
import {
  displayValue,
  formatProjectDate,
  formatBytes,
} from "@utils/projectDetails";
import {
  Box,
  Button,
  Chip,
  IconButton,
  Skeleton,
  Typography,
  alpha,
} from "@wso2/oxygen-ui";
import {
  Download,
  FileArchive,
  FileImage,
  FileText,
  PencilLine,
  Trash2,
  Upload,
} from "@wso2/oxygen-ui-icons-react";
import { useState, useMemo, type JSX } from "react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useDeleteAttachment } from "@api/useDeleteAttachment";

const IMAGE_EXT = /\.(png|jpe?g|gif|webp|svg|bmp|ico)$/i;
const ARCHIVE_EXT = /\.(zip|tar|gz|7z|rar|bz2)$/i;

import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import UploadAttachmentModal from "@case-details-attachments/UploadAttachmentModal";
import DeleteAttachmentModal from "@case-details-attachments/DeleteAttachmentModal";
import EditDeploymentAttachmentModal from "@components/project-details/deployments/EditDeploymentAttachmentModal";
import {
  useInfiniteDeploymentDocuments,
  flattenDeploymentDocuments,
} from "@api/useInfiniteDeploymentDocuments";
import { useQueryClient } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";

interface DeploymentDocumentListProps {
  deploymentId: string;
}

/**
 * Renders the list of documents for a deployment with Add Document button.
 *
 * @param {DeploymentDocumentListProps} props - Props containing deploymentId.
 * @returns {JSX.Element} The document list component.
 */
export default function DeploymentDocumentList({
  deploymentId,
}: DeploymentDocumentListProps): JSX.Element {
  const { showError } = useErrorBanner();
  const queryClient = useQueryClient();
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const {
    data,
    isLoading,
    isError: hasError,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteDeploymentDocuments(deploymentId);

  const documents = flattenDeploymentDocuments(data);

  const handleAddSuccess = () => {
    setIsAddModalOpen(false);
    queryClient.invalidateQueries({
      queryKey: [ApiQueryKeys.DEPLOYMENT_ATTACHMENTS, deploymentId],
    });
  };

  return (
    <Box>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          mb: 2,
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
            Documents
          </Typography>
          {isLoading ? (
            <Skeleton
              variant="rounded"
              width={32}
              height={20}
              sx={{ flexShrink: 0 }}
            />
          ) : hasError ? (
            <Typography variant="body2" color="text.secondary">
              (?)
            </Typography>
          ) : (
            <Typography variant="body2" color="text.secondary">
              ({documents.length})
            </Typography>
          )}
        </Box>
        <Button
          variant="outlined"
          size="small"
          startIcon={<Upload size={16} aria-hidden />}
          sx={{ height: 32, fontSize: "0.75rem" }}
          onClick={() => setIsAddModalOpen(true)}
        >
          Upload
        </Button>
      </Box>
      {isLoading ? (
        <DocumentsSkeleton />
      ) : hasError ? (
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, py: 2 }}>
          <ErrorIndicator entityName="documents" size="small" />
          <Typography variant="body2" color="text.secondary">
            Failed to load documents
          </Typography>
        </Box>
      ) : documents.length === 0 ? (
        <Typography
          variant="body2"
          color="text.secondary"
          sx={{ py: 2, textAlign: "center" }}
        >
          No documents uploaded
        </Typography>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          {documents.map((doc) => (
            <DocumentRow
              key={doc.id}
              doc={doc}
              deploymentId={deploymentId}
              onError={showError}
            />
          ))}
          {hasNextPage && (
            <Button
              variant="outlined"
              size="small"
              disabled={isFetchingNextPage}
              onClick={() => fetchNextPage()}
              sx={{ alignSelf: "flex-start", mt: 1 }}
            >
              {isFetchingNextPage ? "Loading..." : "Load more"}
            </Button>
          )}
        </Box>
      )}

      <UploadAttachmentModal
        open={isAddModalOpen}
        deploymentId={deploymentId}
        onClose={() => setIsAddModalOpen(false)}
        onSuccess={handleAddSuccess}
      />
    </Box>
  );
}

function DocumentsSkeleton(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {[1, 2, 3].map((i) => (
        <Box
          key={i}
          sx={{
            display: "flex",
            flexDirection: "column",
            p: 2,
            bgcolor: (theme) => alpha(theme.palette.grey[500], 0.05),
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "flex-start",
              justifyContent: "space-between",
              gap: 2,
            }}
          >
            <Box sx={{ display: "flex", gap: 2, flex: 1, minWidth: 0 }}>
              <Box sx={{ mt: 0.5, flexShrink: 0 }}>
                <Skeleton variant="rounded" width={20} height={20} />
              </Box>
              <Box sx={{ flex: 1, minWidth: 0 }}>
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 1,
                    mb: 0.5,
                    flexWrap: "wrap",
                  }}
                >
                  <Skeleton variant="text" width="40%" height={20} />
                  <Skeleton variant="rounded" width={70} height={20} />
                </Box>
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 1.5,
                    flexWrap: "wrap",
                  }}
                >
                  <Skeleton variant="text" width={50} height={16} />
                  <Skeleton variant="text" width={90} height={16} />
                  <Skeleton variant="text" width={60} height={16} />
                </Box>
              </Box>
            </Box>
            <Box sx={{ display: "flex", gap: 0.25, flexShrink: 0 }}>
              <Skeleton variant="rounded" width={32} height={32} />
              <Skeleton variant="rounded" width={32} height={32} />
              <Skeleton variant="rounded" width={32} height={32} />
            </Box>
          </Box>
          {i === 1 && (
            <Box sx={{ pl: 4.5, pt: 1 }}>
              <Skeleton variant="text" width="70%" height={16} />
              <Skeleton variant="text" width="50%" height={16} sx={{ mt: 0.25 }} />
            </Box>
          )}
        </Box>
      ))}
    </Box>
  );
}

interface DocumentRowProps {
  doc: DeploymentDocument;
  deploymentId: string;
  onError: (message: string) => void;
}

function DocumentRow({
  doc,
  deploymentId,
  onError,
}: DocumentRowProps): JSX.Element {
  const [deleteModalOpen, setDeleteModalOpen] = useState(false);
  const [editModalOpen, setEditModalOpen] = useState(false);
  const deleteAttachment = useDeleteAttachment();
  const sizeBytes = doc.sizeBytes ?? doc.size ?? 0;
  const dateStr = formatProjectDate(doc.uploadedAt ?? doc.createdOn ?? "");
  const emptyVal = "Not Available";
  const name = displayValue(doc.name, emptyVal);
  const uploadedBy = displayValue(doc.uploadedBy ?? doc.createdBy, emptyVal);
  const sizeStr = formatBytes(sizeBytes);
  const category = doc.category;

  const fileType = useMemo(() => {
    const ext = name.split(".").pop() ?? "";
    if (IMAGE_EXT.test(`.${ext}`)) return "image";
    if (ARCHIVE_EXT.test(`.${ext}`)) return "archive";
    return "text";
  }, [name]);

  const handleConfirmDelete = () => {
    if (!doc.id) return;
    deleteAttachment.mutate(
      { attachmentId: doc.id, deploymentId },
      {
        onSuccess: () => setDeleteModalOpen(false),
        onError: (error) => {
          onError(error.message);
        },
      },
    );
  };

  return (
    <>
      <Box
        sx={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          p: 2,
          bgcolor: (theme) => alpha(theme.palette.grey[500], 0.05),
        }}
      >
        <Box sx={{ display: "flex", gap: 2, flex: 1, minWidth: 0, flexDirection: "column" }}>
          <Box sx={{ display: "flex", gap: 2, flex: 1, minWidth: 0 }}>
            <Box sx={{ mt: 0.5, flexShrink: 0, color: "text.secondary" }}>
              {fileType === "image" ? (
                <FileImage size={20} aria-hidden />
              ) : fileType === "archive" ? (
                <FileArchive size={20} aria-hidden />
              ) : (
                <FileText size={20} aria-hidden />
              )}
            </Box>
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                  mb: 0.5,
                  flexWrap: "wrap",
                }}
              >
                <Typography
                  variant="body2"
                  sx={{
                    fontWeight: 500,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {name}
                </Typography>
                {category && (
                  <Chip
                    label={category}
                    size="small"
                    sx={{
                      height: 20,
                      fontSize: "0.75rem",
                      bgcolor: "warning.lighter",
                      color: "warning.dark",
                    }}
                  />
                )}
              </Box>
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1.5,
                  flexWrap: "wrap",
                }}
              >
                <Typography variant="caption" color="text.secondary">
                  {sizeStr}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  •
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  Uploaded {dateStr}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  •
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {uploadedBy}
                </Typography>
              </Box>
            </Box>
            <Box sx={{ display: "flex", gap: 0.25, flexShrink: 0 }}>
              <IconButton
                size="small"
                aria-label={`Edit ${name}`}
                sx={{ color: "text.secondary" }}
                onClick={() => setEditModalOpen(true)}
              >
                <PencilLine size={16} aria-hidden />
              </IconButton>
              <IconButton
                size="small"
                aria-label={`Delete ${name}`}
                sx={{ color: "text.secondary" }}
                onClick={() => setDeleteModalOpen(true)}
              >
                <Trash2 size={16} aria-hidden />
              </IconButton>
              {doc.content || doc.downloadUrl ? (
                <IconButton
                  size="small"
                  aria-label={`Download ${name}`}
                  sx={{ color: "text.secondary" }}
                  onClick={() => {
                    if (doc.content) {
                      const isDataUrl = doc.content.startsWith("data:");
                      const href = isDataUrl
                        ? doc.content
                        : `data:application/octet-stream;base64,${doc.content}`;
                      const link = document.createElement("a");
                      link.href = href;
                      link.download = name || "document";
                      link.target = "_blank";
                      link.rel = "noopener noreferrer";
                      link.click();
                    } else if (doc.downloadUrl) {
                      window.open(doc.downloadUrl, "_blank", "noopener,noreferrer");
                    }
                  }}
                >
                  <Download size={16} aria-hidden />
                </IconButton>
              ) : (
                <IconButton size="small" aria-label={`Download ${name}`} disabled>
                  <Download size={16} aria-hidden />
                </IconButton>
              )}
            </Box>
          </Box>
          {doc.description && (
            <Box sx={{ pl: 4.5, pt: 0.5 }}>
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-word",
                }}
              >
                {doc.description}
              </Typography>
            </Box>
          )}
        </Box>
      </Box>
      <DeleteAttachmentModal
        open={deleteModalOpen}
        attachmentName={name}
        onClose={() => setDeleteModalOpen(false)}
        onConfirm={handleConfirmDelete}
        isDeleting={deleteAttachment.isPending}
      />
      <EditDeploymentAttachmentModal
        open={editModalOpen}
        document={doc}
        deploymentId={deploymentId}
        onClose={() => setEditModalOpen(false)}
        onSuccess={() => {}}
        onError={onError}
      />
    </>
  );
}
