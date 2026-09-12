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
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Download, Pencil, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import AttachmentsField from "@components/attachments/AttachmentsField";
import {
  POST_CREATE_ATTACHMENTS_MAX_ENCODED_BYTES,
  type EncodedAttachment,
} from "@components/attachments/encodeAttachment";
import QueryErrorState from "@components/QueryErrorState";
import { formatBytes } from "@utils/formatBytes";
import { useSearchDeploymentAttachments } from "@features/csm-projects/api/useSearchDeploymentAttachments";
import { useCreateDeploymentAttachment } from "@features/csm-projects/api/useCreateDeploymentAttachment";
import { useUpdateDeploymentAttachment } from "@features/csm-projects/api/useUpdateDeploymentAttachment";
import { useDeleteDeploymentAttachment } from "@features/csm-projects/api/useDeleteDeploymentAttachment";
import { useDownloadDeploymentAttachment } from "@features/csm-projects/api/useDownloadDeploymentAttachment";
import type { DeploymentAttachment } from "@features/csm-projects/types/csmProjects";

interface DeploymentAttachmentsPanelProps {
  deploymentId: string;
}

interface Feedback {
  message: string;
  severity: "success" | "error";
}

function formatUploadedOn(iso: string): string {
  const d = new Date(iso);
  return isNaN(d.getTime()) ? iso : d.toLocaleString();
}

/**
 * Deployment-level attachments: upload, list, edit (name/description), and
 * delete files against the shared reference-generic `/attachments*`
 * endpoints scoped to `referenceType: "deployment"`.
 *
 * New files are staged locally via {@link AttachmentsField} (the shared
 * multi-file picker), then uploaded one at a time on "Upload" — matching the
 * BE's one-file-per-request `POST /attachments` contract.
 */
export default function DeploymentAttachmentsPanel({
  deploymentId,
}: DeploymentAttachmentsPanelProps): JSX.Element {
  const { data, isLoading, isError, error } = useSearchDeploymentAttachments(deploymentId);
  const createAttachment = useCreateDeploymentAttachment();
  const updateAttachment = useUpdateDeploymentAttachment();
  const deleteAttachment = useDeleteDeploymentAttachment();
  const downloadAttachment = useDownloadDeploymentAttachment();

  const [pending, setPending] = useState<EncodedAttachment[]>([]);
  const [uploading, setUploading] = useState(false);
  const [downloadingId, setDownloadingId] = useState<string | null>(null);
  const [editing, setEditing] = useState<DeploymentAttachment | null>(null);
  const [editName, setEditName] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [deleting, setDeleting] = useState<DeploymentAttachment | null>(null);
  const [feedback, setFeedback] = useState<Feedback | null>(null);

  const attachments = data ?? [];

  const handleUploadPending = async (): Promise<void> => {
    if (pending.length === 0) return;
    setUploading(true);
    let failed = 0;
    for (const p of pending) {
      try {
        await createAttachment.mutateAsync({ deploymentId, file: p.raw, name: p.name });
      } catch {
        failed += 1;
      }
    }
    setUploading(false);
    setPending([]);
    if (failed > 0) {
      setFeedback({
        message: `${failed} of ${pending.length} file(s) failed to upload.`,
        severity: "error",
      });
    } else {
      setFeedback({ message: "Attachment(s) uploaded.", severity: "success" });
    }
  };

  const openEdit = (a: DeploymentAttachment): void => {
    setEditing(a);
    setEditName(a.name);
    setEditDescription(a.description ?? "");
  };

  const handleSaveEdit = (): void => {
    if (!editing) return;
    const nameChanged = editName.trim() !== editing.name;
    const descriptionChanged = editDescription.trim() !== (editing.description ?? "");
    if (!nameChanged && !descriptionChanged) {
      setEditing(null);
      return;
    }
    updateAttachment.mutate(
      {
        deploymentId,
        attachmentId: editing.id,
        ...(nameChanged ? { name: editName.trim() } : {}),
        ...(descriptionChanged
          ? { description: editDescription.trim() ? editDescription.trim() : null }
          : {}),
      },
      {
        onSuccess: () => {
          setEditing(null);
          setFeedback({ message: "Attachment updated.", severity: "success" });
        },
        onError: (err) => {
          setEditing(null);
          setFeedback({
            message: `Could not update the attachment: ${err.message}`,
            severity: "error",
          });
        },
      },
    );
  };

  const handleConfirmDelete = (): void => {
    if (!deleting) return;
    const target = deleting;
    deleteAttachment.mutate(
      { deploymentId, attachmentId: target.id },
      {
        onSuccess: () => {
          setDeleting(null);
          setFeedback({ message: `Deleted ${target.name}.`, severity: "success" });
        },
        onError: (err) => {
          setDeleting(null);
          setFeedback({
            message: `Could not delete ${target.name}: ${err.message}`,
            severity: "error",
          });
        },
      },
    );
  };

  const handleDownload = async (a: DeploymentAttachment): Promise<void> => {
    setDownloadingId(a.id);
    try {
      await downloadAttachment(a);
    } catch (err) {
      setFeedback({
        message: `Could not download ${a.name}: ${err instanceof Error ? err.message : String(err)}`,
        severity: "error",
      });
    } finally {
      setDownloadingId(null);
    }
  };

  return (
    <Box sx={{ py: 1 }}>
      {feedback && (
        <Alert severity={feedback.severity} onClose={() => setFeedback(null)} sx={{ mb: 1 }}>
          {feedback.message}
        </Alert>
      )}

      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4, display: "block", px: 1, mb: 0.5 }}
      >
        Attachments
      </Typography>

      <Box sx={{ px: 1, display: "flex", flexDirection: "column", gap: 1 }}>
        <AttachmentsField
          attachments={pending}
          onChange={setPending}
          onError={(message) => setFeedback({ message, severity: "error" })}
          maxEncodedBytes={POST_CREATE_ATTACHMENTS_MAX_ENCODED_BYTES}
          disabled={uploading}
        />
        {pending.length > 0 && (
          <Box>
            <Button
              size="small"
              variant="contained"
              onClick={() => void handleUploadPending()}
              disabled={uploading}
              startIcon={uploading ? <CircularProgress size={14} color="inherit" /> : undefined}
            >
              {uploading ? "Uploading…" : `Upload ${pending.length} file(s)`}
            </Button>
          </Box>
        )}
      </Box>

      {isLoading ? (
        <Box sx={{ display: "flex", justifyContent: "center", py: 3 }}>
          <CircularProgress size={20} />
        </Box>
      ) : isError ? (
        <QueryErrorState
          message={
            error instanceof Error && error.message.trim()
              ? error.message
              : "Failed to load attachments."
          }
          error={error}
        />
      ) : attachments.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ py: 1, px: 1 }}>
          No attachments on this deployment.
        </Typography>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1, mt: 1, px: 1 }}>
          {attachments.map((a) => (
            <Box
              key={a.id}
              sx={{
                display: "flex",
                alignItems: "flex-start",
                justifyContent: "space-between",
                gap: 1,
                border: 1,
                borderColor: "divider",
                borderRadius: 1,
                px: 1.5,
                py: 1,
              }}
            >
              <Box sx={{ minWidth: 0, flex: 1 }}>
                <Typography variant="body2" sx={{ fontWeight: 500 }} noWrap title={a.name}>
                  {a.name}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {formatBytes(a.sizeBytes)} · {a.uploadedBy} · {formatUploadedOn(a.uploadedOn)}
                </Typography>
                {a.description && (
                  <Typography
                    variant="body2"
                    color="text.secondary"
                    sx={{ mt: 0.5, whiteSpace: "pre-wrap", wordBreak: "break-word" }}
                  >
                    {a.description}
                  </Typography>
                )}
              </Box>
              <Box sx={{ display: "flex", gap: 0.25, flexShrink: 0 }}>
                <Tooltip title="Edit">
                  <IconButton size="small" aria-label={`Edit ${a.name}`} onClick={() => openEdit(a)}>
                    <Pencil size={14} />
                  </IconButton>
                </Tooltip>
                <Tooltip title="Delete">
                  <IconButton
                    size="small"
                    aria-label={`Delete ${a.name}`}
                    onClick={() => setDeleting(a)}
                  >
                    <Trash2 size={14} />
                  </IconButton>
                </Tooltip>
                <Tooltip title="Download">
                  <IconButton
                    size="small"
                    aria-label={`Download ${a.name}`}
                    disabled={downloadingId === a.id}
                    onClick={() => void handleDownload(a)}
                  >
                    {downloadingId === a.id ? (
                      <CircularProgress size={14} />
                    ) : (
                      <Download size={14} />
                    )}
                  </IconButton>
                </Tooltip>
              </Box>
            </Box>
          ))}
        </Box>
      )}

      {editing && (
        <Dialog open onClose={() => setEditing(null)} maxWidth="sm" fullWidth>
          <DialogTitle>Edit attachment</DialogTitle>
          <DialogContent dividers>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
              <TextField
                label="Name"
                value={editName}
                onChange={(e) => setEditName(e.target.value)}
                size="small"
                fullWidth
                autoFocus
              />
              <TextField
                label="Description"
                value={editDescription}
                onChange={(e) => setEditDescription(e.target.value)}
                size="small"
                fullWidth
                multiline
                minRows={2}
              />
            </Box>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setEditing(null)} disabled={updateAttachment.isPending}>
              Cancel
            </Button>
            <Button
              variant="contained"
              disabled={updateAttachment.isPending || editName.trim() === ""}
              onClick={handleSaveEdit}
            >
              Save changes
            </Button>
          </DialogActions>
        </Dialog>
      )}

      <Dialog open={!!deleting} onClose={() => setDeleting(null)} maxWidth="xs" fullWidth>
        <DialogTitle>Delete attachment</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Delete{" "}
            <Box component="span" sx={{ fontWeight: 600, color: "text.primary" }}>
              {deleting?.name ?? "this attachment"}
            </Box>
            ? This cannot be undone.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleting(null)} disabled={deleteAttachment.isPending}>
            Cancel
          </Button>
          <Button
            color="error"
            variant="contained"
            onClick={handleConfirmDelete}
            disabled={deleteAttachment.isPending}
          >
            Delete
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
