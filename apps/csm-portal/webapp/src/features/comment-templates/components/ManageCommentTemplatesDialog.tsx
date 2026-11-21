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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Pencil, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type { BeCommentTemplate } from "@api/backend/types";
import { useDeleteCommentTemplate } from "@features/comment-templates/api/useCommentTemplates";
import {
  APPLIES_TO_LABELS,
  templatePlainText,
} from "@features/comment-templates/utils/commentTemplates";

interface ManageCommentTemplatesDialogProps {
  open: boolean;
  templates: BeCommentTemplate[];
  /** Admins may edit and delete shared templates too. */
  isAdmin: boolean;
  onEdit: (template: BeCommentTemplate) => void;
  onClose: () => void;
}

/** Lists every template the user sees, with Edit and Delete where allowed. */
export default function ManageCommentTemplatesDialog({
  open,
  templates,
  isAdmin,
  onEdit,
  onClose,
}: ManageCommentTemplatesDialogProps): JSX.Element {
  const remove = useDeleteCommentTemplate();
  const [confirmId, setConfirmId] = useState<string | null>(null);
  const mine = templates.filter((t) => !t.shared);
  const shared = templates.filter((t) => t.shared);

  const section = (title: string, list: BeCommentTemplate[], empty: string) => (
    <Box component="section" aria-label={title} sx={{ mb: 2 }}>
      <Typography variant="subtitle2" sx={{ mb: 1 }}>
        {title}
      </Typography>
      {list.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          {empty}
        </Typography>
      ) : (
        list.map((t) => {
          const canChange = !t.shared || isAdmin;
          return (
            <Box
              key={t.id}
              sx={{
                display: "flex",
                alignItems: "flex-start",
                gap: 1,
                py: 1,
                borderBottom: 1,
                borderColor: "divider",
              }}
            >
              <Box sx={{ flex: 1, minWidth: 0 }}>
                <Typography variant="body2" sx={{ fontWeight: 600 }}>
                  {t.name}
                </Typography>
                <Typography variant="caption" color="text.secondary" component="div">
                  {APPLIES_TO_LABELS[t.appliesTo]}
                </Typography>
                <Typography variant="caption" color="text.secondary" noWrap component="div">
                  {templatePlainText(t.body)}
                </Typography>
              </Box>
              {canChange &&
                (confirmId === t.id ? (
                  <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                    <Typography variant="caption">Delete?</Typography>
                    <Button
                      size="small"
                      color="error"
                      disabled={remove.isPending}
                      onClick={() => {
                        remove.mutate(
                          { id: t.id, shared: t.shared },
                          { onSuccess: () => setConfirmId(null) },
                        );
                      }}
                    >
                      Delete
                    </Button>
                    <Button size="small" onClick={() => setConfirmId(null)}>
                      Keep
                    </Button>
                  </Box>
                ) : (
                  <Box sx={{ display: "flex" }}>
                    <Tooltip title="Edit">
                      <IconButton size="small" aria-label={`Edit ${t.name}`} onClick={() => onEdit(t)}>
                        <Pencil size={16} />
                      </IconButton>
                    </Tooltip>
                    <Tooltip title="Delete">
                      <IconButton
                        size="small"
                        aria-label={`Delete ${t.name}`}
                        onClick={() => setConfirmId(t.id)}
                      >
                        <Trash2 size={16} />
                      </IconButton>
                    </Tooltip>
                  </Box>
                ))}
            </Box>
          );
        })
      )}
    </Box>
  );

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>Comment templates</DialogTitle>
      <DialogContent>
        {section("My templates", mine, "You have no templates yet. Use “Save draft as template” in the comment box.")}
        {section("Shared", shared, "No shared templates yet.")}
        {remove.error && <Alert severity="error">{remove.error.message}</Alert>}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}
