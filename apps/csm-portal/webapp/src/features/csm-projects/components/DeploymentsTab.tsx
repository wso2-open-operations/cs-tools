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
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
  Menu,
  MenuItem,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Ban, Eye, MoreVertical, Pencil, Plus } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useSearchDeployments } from "@features/csm-cases/api/useSearchDeployments";
import { useUpdateDeployment } from "@features/csm-projects/api/useUpdateDeployment";
import { useCreateDeployment } from "@features/csm-projects/api/useCreateDeployment";
import {
  deploymentTypeLabel,
  formatDeploymentDate,
} from "@features/csm-projects/utils/deployments";
import type {
  BeDeployment,
  BeDeploymentDetailUpdatePayload,
  BeDeploymentCreatePayload,
} from "@api/backend/types";
import EditDeploymentDialog from "@features/csm-projects/components/EditDeploymentDialog";
import DeploymentDetailsDialog from "@features/csm-projects/components/DeploymentDetailsDialog";
import CreateDeploymentDialog from "@features/csm-projects/components/CreateDeploymentDialog";

/** Columns rendered per deployment row (5 data + actions). */
const COLUMN_COUNT = 6;

interface DeploymentsTabProps {
  projectId: string;
}

interface Feedback {
  message: string;
  severity: "success" | "error";
}

/**
 * Lists a project's deployments (`POST /deployments/search`) and lets engineers
 * create, edit, or deactivate deployments. Type is editable via a select
 * (string enum per PR #957; `typeKey` integer is gone).
 */
export default function DeploymentsTab({ projectId }: DeploymentsTabProps): JSX.Element {
  const { data, isLoading, isError, error, isFetching } = useSearchDeployments(projectId);
  const updateDeployment = useUpdateDeployment(projectId);
  const createDeployment = useCreateDeployment(projectId);

  // Row-action menu anchored to the deployment whose "⋮" was clicked.
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [menuTarget, setMenuTarget] = useState<BeDeployment | null>(null);
  const [editing, setEditing] = useState<BeDeployment | null>(null);
  const [deactivating, setDeactivating] = useState<BeDeployment | null>(null);
  const [viewing, setViewing] = useState<BeDeployment | null>(null);
  const [creating, setCreating] = useState(false);
  const [feedback, setFeedback] = useState<Feedback | null>(null);

  const deployments = data ?? [];

  const openMenu = (e: React.MouseEvent<HTMLElement>, d: BeDeployment): void => {
    setMenuAnchor(e.currentTarget);
    setMenuTarget(d);
  };
  const closeMenu = (): void => {
    setMenuAnchor(null);
    setMenuTarget(null);
  };

  const handleSaveEdit = (payload: BeDeploymentDetailUpdatePayload): void => {
    if (!editing) return;
    const name = editing.name ?? "this deployment";
    updateDeployment.mutate(
      { deploymentId: editing.id, payload },
      {
        onSuccess: () => {
          setEditing(null);
          setFeedback({ message: `Updated ${name}.`, severity: "success" });
        },
        onError: (err) => {
          // Close the dialog first so the page-level alert is not hidden
          // behind the modal backdrop.
          setEditing(null);
          setFeedback({
            message: `Could not update ${name}: ${err.message}`,
            severity: "error",
          });
        },
      },
    );
  };

  const handleDeactivate = (): void => {
    if (!deactivating) return;
    const target = deactivating;
    const name = target.name ?? "this deployment";
    updateDeployment.mutate(
      { deploymentId: target.id, payload: { active: false } },
      {
        onSuccess: () => {
          setDeactivating(null);
          setFeedback({ message: `Deactivated ${name}.`, severity: "success" });
        },
        onError: (err) => {
          // Close the dialog first so the page-level alert is not hidden
          // behind the modal backdrop.
          setDeactivating(null);
          setFeedback({
            message: `Could not deactivate ${name}: ${err.message}`,
            severity: "error",
          });
        },
      },
    );
  };

  const handleCreate = (payload: BeDeploymentCreatePayload): void => {
    createDeployment.mutate(payload, {
      onSuccess: () => {
        setCreating(false);
        setFeedback({ message: "Deployment created.", severity: "success" });
      },
      onError: (err) => {
        setCreating(false);
        setFeedback({
          message: `Could not create deployment: ${err.message}`,
          severity: "error",
        });
      },
    });
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {feedback && (
        <Alert severity={feedback.severity} onClose={() => setFeedback(null)}>
          {feedback.message}
        </Alert>
      )}

      {isError && (
        <Alert severity="error">
          Failed to load deployments:{" "}
          {error instanceof Error ? error.message : "unknown error"}
        </Alert>
      )}

      <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
        <Button
          variant="contained"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => setCreating(true)}
        >
          Create deployment
        </Button>
      </Box>

      <Paper variant="outlined">
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Name</TableCell>
                <TableCell>Type</TableCell>
                <TableCell>Description</TableCell>
                <TableCell>Created</TableCell>
                <TableCell>Updated</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={COLUMN_COUNT} align="center" sx={{ py: 4 }}>
                    <CircularProgress size={24} />
                  </TableCell>
                </TableRow>
              ) : deployments.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={COLUMN_COUNT} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No deployments for this project.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                deployments.map((d) => (
                  <TableRow
                    key={d.id}
                    hover
                    onClick={() => setViewing(d)}
                    sx={{ cursor: "pointer" }}
                  >
                    <TableCell>{d.name || "—"}</TableCell>
                    <TableCell>
                      <Chip size="small" variant="outlined" label={deploymentTypeLabel(d.type)} />
                    </TableCell>
                    <TableCell sx={{ maxWidth: 320 }}>
                      <Typography variant="body2" color="text.secondary" noWrap title={d.description}>
                        {d.description || "—"}
                      </Typography>
                    </TableCell>
                    <TableCell>{formatDeploymentDate(d.createdOn)}</TableCell>
                    <TableCell>{formatDeploymentDate(d.updatedOn)}</TableCell>
                    <TableCell align="right">
                      <Tooltip title="Deployment actions">
                        <IconButton
                          size="small"
                          aria-label={`Actions for ${d.name || "deployment"}`}
                          onClick={(e) => {
                            // Keep the row's open-details click from also firing.
                            e.stopPropagation();
                            openMenu(e, d);
                          }}
                        >
                          <MoreVertical size={16} />
                        </IconButton>
                      </Tooltip>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>

      {isFetching && !isLoading && (
        <Typography variant="caption" color="text.secondary">
          Updating…
        </Typography>
      )}

      <Menu anchorEl={menuAnchor} open={!!menuAnchor} onClose={closeMenu}>
        <MenuItem
          onClick={() => {
            setViewing(menuTarget);
            closeMenu();
          }}
        >
          <Eye size={16} style={{ marginRight: 8 }} />
          View details
        </MenuItem>
        <MenuItem
          onClick={() => {
            setEditing(menuTarget);
            closeMenu();
          }}
        >
          <Pencil size={16} style={{ marginRight: 8 }} />
          Edit details
        </MenuItem>
        <MenuItem
          onClick={() => {
            setDeactivating(menuTarget);
            closeMenu();
          }}
          sx={{ color: "error.main" }}
        >
          <Ban size={16} style={{ marginRight: 8 }} />
          Deactivate
        </MenuItem>
      </Menu>

      {viewing && (
        <DeploymentDetailsDialog deployment={viewing} onClose={() => setViewing(null)} />
      )}

      {editing && (
        <EditDeploymentDialog
          deployment={editing}
          isSaving={updateDeployment.isPending}
          onClose={() => setEditing(null)}
          onSave={handleSaveEdit}
        />
      )}

      {creating && (
        <CreateDeploymentDialog
          projectId={projectId}
          isSaving={createDeployment.isPending}
          onClose={() => setCreating(false)}
          onCreate={handleCreate}
        />
      )}

      <Dialog
        open={!!deactivating}
        onClose={() => setDeactivating(null)}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>Deactivate deployment</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Deactivate{" "}
            <Box component="span" sx={{ fontWeight: 600, color: "text.primary" }}>
              {deactivating?.name || "this deployment"}
            </Box>
            ? This marks the deployment inactive in ServiceNow.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeactivating(null)} disabled={updateDeployment.isPending}>
            Cancel
          </Button>
          <Button
            color="error"
            variant="contained"
            onClick={handleDeactivate}
            disabled={updateDeployment.isPending}
          >
            Deactivate
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
