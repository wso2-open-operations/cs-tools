// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License
// at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { type JSX } from "react";
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useDeleteRegistryToken } from "@features/settings/api/useDeleteRegistryToken";
import type { DeleteTokenModalProps } from "@features/settings/types/settings";

/**
 * Confirmation dialog for deleting a registry token.
 */
export default function DeleteTokenModal({
  open,
  onClose,
  projectId,
  token,
}: DeleteTokenModalProps): JSX.Element {
  const deleteMutation = useDeleteRegistryToken(projectId);

  function handleDelete() {
    if (!token?.id) return;
    deleteMutation.mutate(token.id, {
      onSuccess: () => {
        handleClose();
      },
    });
  }

  function handleClose() {
    deleteMutation.reset();
    onClose();
  }

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="xs" fullWidth>
      <DialogTitle
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        Delete Registry Token
        <IconButton size="small" onClick={handleClose} aria-label="close">
          <X size={18} />
        </IconButton>
      </DialogTitle>

      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
          <Typography variant="body2">
            Are you sure you want to delete the token{" "}
            <strong>{token?.displayName ?? token?.name ?? "this token"}</strong>
            ?
          </Typography>
          <Typography variant="body2" color="text.secondary">
            This action cannot be undone. Any systems using this token will lose
            access immediately.
          </Typography>

          {deleteMutation.isError && (
            <Alert severity="error" sx={{ borderRadius: 1 }}>
              {deleteMutation.error?.message ?? "Failed to delete token."}
            </Alert>
          )}
        </Box>
      </DialogContent>

      <DialogActions sx={{ px: 3, py: 2 }}>
        <Button
          variant="outlined"
          color="secondary"
          onClick={handleClose}
          disabled={deleteMutation.isPending}
        >
          Cancel
        </Button>
        <Button
          variant="contained"
          color="error"
          onClick={handleDelete}
          disabled={deleteMutation.isPending || !token?.id}
          startIcon={
            deleteMutation.isPending ? (
              <CircularProgress size={16} color="inherit" />
            ) : undefined
          }
        >
          {deleteMutation.isPending ? "Deleting…" : "Delete"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
