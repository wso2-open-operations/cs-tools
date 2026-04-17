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
  Autocomplete,
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
import { useCreateRegistryToken } from "@features/settings/api/useCreateRegistryToken";
import { useGetIntegrationUsers } from "@features/settings/api/useGetIntegrationUsers";
import type { IntegrationUser } from "@features/settings/types/users";
import { RegistryTokenType } from "@features/settings/types/registryTokens";
import { REGISTRY_TOKEN_ROBOT_NAME_REGEX } from "@features/settings/constants/settingsConstants";
import type { GenerateTokenModalProps } from "@features/settings/types/settings";

/**
 * Modal dialog for generating a new registry token.
 *
 * Step 1 – User provides inputs (robotName, createdFor for service tokens).
 * Step 2 – After creation, the secret is shown with a copy button.
 */
export default function GenerateTokenModal({
  open,
  onClose,
  projectId,
  tokenType,
  isAdmin,
}: GenerateTokenModalProps): JSX.Element {
  const [robotName, setRobotName] = useState("");
  const [robotNameError, setRobotNameError] = useState<string | null>(null);
  const [selectedUser, setSelectedUser] = useState<IntegrationUser | null>(
    null,
  );
  const [createdForError, setCreatedForError] = useState<string | null>(null);

  const [secret, setSecret] = useState<string | null>(null);
  const [showSecret, setShowSecret] = useState(false);
  const [copiedSecret, setCopiedSecret] = useState(false);
  const [copiedName, setCopiedName] = useState(false);

  const createMutation = useCreateRegistryToken(projectId);
  const { data: integrationUsers = [], isLoading: isLoadingUsers } =
    useGetIntegrationUsers(
      projectId,
      tokenType === RegistryTokenType.SERVICE && isAdmin,
    );

  function validateRobotName(value: string): string | null {
    if (!value.trim()) return "Token name is required.";
    if (!REGISTRY_TOKEN_ROBOT_NAME_REGEX.test(value))
      return "Only alphanumeric characters and dashes are allowed.";
    return null;
  }

  async function handleGenerate() {
    const nameErr = validateRobotName(robotName);
    setRobotNameError(nameErr);

    let userErr: string | null = null;
    if (tokenType === RegistryTokenType.SERVICE && !selectedUser) {
      userErr = "Please select an integration user for service tokens.";
    }
    setCreatedForError(userErr);

    if (nameErr || userErr) return;

    createMutation.mutate(
      {
        robotName: robotName.trim(),
        tokenType,
        createdFor:
          tokenType === RegistryTokenType.SERVICE && selectedUser
            ? selectedUser.email
            : undefined,
      },
      {
        onSuccess: (data) => {
          setSecret(data.secret);
        },
      },
    );
  }

  function handleClose() {
    if (createMutation.isPending) return;
    setRobotName("");
    setRobotNameError(null);
    setSelectedUser(null);
    setCreatedForError(null);
    setSecret(null);
    setShowSecret(false);
    setCopiedSecret(false);
    setCopiedName(false);
    createMutation.reset();
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
          ? "Token Created Successfully"
          : `Generate ${tokenType === RegistryTokenType.SERVICE ? "Service" : "User"} Token`}
        <IconButton size="small" onClick={handleClose} aria-label="close">
          <X size={18} />
        </IconButton>
      </DialogTitle>

      <DialogContent dividers>
        {isSecretStep ? (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
            <Alert severity="warning" sx={{ borderRadius: 1 }}>
              <strong>Important:</strong> Copy and save this secret now. You
              will not be able to see it again after closing this dialog.
            </Alert>
            <TextField
              label="Token Name"
              fullWidth
              value={robotName}
              InputProps={{
                readOnly: true,
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton
                      size="small"
                      onClick={() => handleCopy(robotName, "name")}
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
              label="Token Secret"
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
          <Box
            sx={{ display: "flex", flexDirection: "column", gap: 2.5, pt: 1 }}
          >
            <Typography variant="body2" color="text.secondary">
              {tokenType === RegistryTokenType.USER
                ? "Generate a personal registry token for WSO2 Updates 2.0 access. The token will be created for your account."
                : "Generate a service registry token for automation and CI/CD pipelines. You must assign it to an integration user."}
            </Typography>

            <TextField
              label="Token Name"
              placeholder="e.g. my-ci-token"
              fullWidth
              required
              value={robotName}
              onChange={(e) => {
                setRobotName(e.target.value);
                if (robotNameError)
                  setRobotNameError(validateRobotName(e.target.value));
              }}
              onBlur={() => setRobotNameError(validateRobotName(robotName))}
              error={!!robotNameError}
              helperText={
                robotNameError ??
                "Only alphanumeric characters and dashes are allowed."
              }
            />

            {/* Service token: select integration user */}
            {tokenType === RegistryTokenType.SERVICE && (
              <Autocomplete
                options={integrationUsers}
                getOptionLabel={(opt: IntegrationUser) => opt.email}
                value={selectedUser}
                onChange={(_e, value) => {
                  setSelectedUser(value);
                  if (createdForError && value) setCreatedForError(null);
                }}
                loading={isLoadingUsers}
                renderInput={(params) => (
                  <TextField
                    {...params}
                    label="Integration User (Created For)"
                    required
                    error={!!createdForError}
                    helperText={
                      createdForError ??
                      "Select the integration user this service token is created for."
                    }
                    InputProps={{
                      ...params.InputProps,
                      endAdornment: (
                        <>
                          {isLoadingUsers && (
                            <CircularProgress color="inherit" size={20} />
                          )}
                          {params.InputProps.endAdornment}
                        </>
                      ),
                    }}
                  />
                )}
              />
            )}

            {createMutation.isError && (
              <Alert severity="error" sx={{ borderRadius: 1 }}>
                {createMutation.error?.message ?? "Failed to create token."}
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
              disabled={createMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              variant="contained"
              color="warning"
              onClick={handleGenerate}
              disabled={createMutation.isPending}
              startIcon={
                createMutation.isPending ? (
                  <CircularProgress size={16} color="inherit" />
                ) : undefined
              }
            >
              {createMutation.isPending ? "Generating…" : "Generate Token"}
            </Button>
          </>
        )}
      </DialogActions>
    </Dialog>
  );
}
