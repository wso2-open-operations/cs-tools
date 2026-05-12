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

import { useState, type JSX } from "react";
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
  InputAdornment,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { CheckCircle, Copy, Eye, EyeOff, X } from "@wso2/oxygen-ui-icons-react";
import { useRegenerateRegistryToken } from "@features/settings/api/useRegenerateRegistryToken";
import type { RegenerateTokenModalProps } from "@features/settings/types/settings";

/**
 * Confirmation + secret display dialog for regenerating a registry token secret.
 *
 * Step 1 – Confirm regeneration (warns the old secret will stop working).
 * Step 2 – Show the new secret with copy button.
 */
export default function RegenerateTokenModal({
  open,
  onClose,
  projectId,
  token,
}: RegenerateTokenModalProps): JSX.Element {
  const [secret, setSecret] = useState<string | null>(null);
  const [fullTokenName, setFullTokenName] = useState<string | null>(null);
  const [showSecret, setShowSecret] = useState(false);
  const [copiedSecret, setCopiedSecret] = useState(false);
  const [copiedName, setCopiedName] = useState(false);

  const regenerateMutation = useRegenerateRegistryToken(projectId);

  function handleRegenerate() {
    if (!token?.id) return;
    regenerateMutation.mutate(token.id, {
      onSuccess: (data) => {
        setSecret(data.secret);
        setFullTokenName(data.name ?? null);
      },
    });
  }

  function handleClose() {
    setSecret(null);
    setFullTokenName(null);
    setShowSecret(false);
    setCopiedSecret(false);
    setCopiedName(false);
    regenerateMutation.reset();
    onClose();
  }

  async function handleCopy(textToCopy: string, type: "name" | "secret") {
    if (!textToCopy) return;
    try {
      await navigator.clipboard.writeText(textToCopy);
      if (type === "name") {
        setCopiedName(true);
        setTimeout(() => setCopiedName(false), 2000);
      } else {
        setCopiedSecret(true);
        setTimeout(() => setCopiedSecret(false), 2000);
      }
    } catch (err) {
      console.error("Failed to copy text: ", err);
    }
  }

  const isSecretStep = secret !== null;

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        {isSecretStep
          ? "Token Regenerated Successfully"
          : "Regenerate Token Secret"}
        <IconButton size="small" onClick={handleClose} aria-label="close">
          <X size={18} />
        </IconButton>
      </DialogTitle>

      <DialogContent dividers>
        {isSecretStep ? (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
            <Alert severity="warning" sx={{ borderRadius: 1 }}>
              <strong>Important:</strong> Copy and save this new secret now. You
              will not be able to see it again after closing this dialog.
            </Alert>
            <TextField
              label="Token Name"
              fullWidth
              value={fullTokenName ?? token?.name}
              InputProps={{
                readOnly: true,
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton
                      size="small"
                      onClick={() =>
                        handleCopy(
                          fullTokenName ?? token?.name ?? "",
                          "name",
                        )
                      }
                      color={copiedName ? "success" : "default"}
                      aria-label="Copy token name"
                    >
                      {copiedName ? (
                        <CheckCircle size={18} />
                      ) : (
                        <Copy size={18} />
                      )}
                    </IconButton>
                  </InputAdornment>
                ),
              }}
            />
            <TextField
              label="New Token Secret"
              fullWidth
              value={secret}
              InputProps={{
                readOnly: true,
                sx: {
                  fontFamily: "monospace",
                  fontSize: "0.85rem",
                  wordBreak: "break-all",
                },
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton
                      size="small"
                      onClick={() => setShowSecret((p) => !p)}
                      aria-label={showSecret ? "Hide secret" : "Show secret"}
                    >
                      {showSecret ? <EyeOff size={18} /> : <Eye size={18} />}
                    </IconButton>
                    <IconButton
                      size="small"
                      onClick={() => handleCopy(secret ?? "", "secret")}
                      color={copiedSecret ? "success" : "default"}
                      aria-label="Copy secret"
                    >
                      {copiedSecret ? (
                        <CheckCircle size={18} />
                      ) : (
                        <Copy size={18} />
                      )}
                    </IconButton>
                  </InputAdornment>
                ),
              }}
              type={showSecret ? "text" : "password"}
            />
          </Box>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
            <Typography variant="body2">
              Are you sure you want to regenerate the secret for token{" "}
              <strong>
                {token?.displayName ?? token?.name ?? "this token"}
              </strong>
              ?
            </Typography>
            <Alert severity="warning" sx={{ borderRadius: 1 }}>
              The current secret will be invalidated immediately. Any systems
              using the old secret will lose access until updated with the new
              one.
            </Alert>

            {regenerateMutation.isError && (
              <Alert severity="error" sx={{ borderRadius: 1 }}>
                {regenerateMutation.error?.message ??
                  "Failed to regenerate token."}
              </Alert>
            )}
          </Box>
        )}
      </DialogContent>

      <DialogActions sx={{ px: 3, py: 2 }}>
        {isSecretStep ? (
          <Button variant="contained" onClick={handleClose}>
            Close
          </Button>
        ) : (
          <>
            <Button
              variant="outlined"
              color="secondary"
              onClick={handleClose}
              disabled={regenerateMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="contained"
              color="warning"
              onClick={handleRegenerate}
              disabled={regenerateMutation.isPending || !token?.id}
              startIcon={
                regenerateMutation.isPending ? (
                  <CircularProgress size={16} color="inherit" />
                ) : undefined
              }
            >
              {regenerateMutation.isPending
                ? "Regenerating…"
                : "Regenerate Secret"}
            </Button>
          </>
        )}
      </DialogActions>
    </Dialog>
  );
}
