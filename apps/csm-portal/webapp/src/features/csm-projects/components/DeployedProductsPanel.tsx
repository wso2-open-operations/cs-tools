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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Ban, MoreVertical, Pencil, Plus } from "@wso2/oxygen-ui-icons-react";
import QueryErrorState from "@components/QueryErrorState";
import { useState, type JSX } from "react";
import { useSearchDeployedProducts } from "@features/csm-projects/api/useSearchDeployedProducts";
import { useCreateDeployedProduct } from "@features/csm-projects/api/useCreateDeployedProduct";
import { useUpdateDeployedProduct } from "@features/csm-projects/api/useUpdateDeployedProduct";
import { formatDeploymentDate } from "@features/csm-projects/utils/deployments";
import type {
  BeDeployedProduct,
  BeDeployedProductCreatePayload,
  BeDeployedProductDetailUpdatePayload,
} from "@api/backend/types";
import CreateDeployedProductDialog from "@features/csm-projects/components/CreateDeployedProductDialog";
import EditDeployedProductDialog from "@features/csm-projects/components/EditDeployedProductDialog";

interface DeployedProductsPanelProps {
  deploymentId: string;
  /**
   * The project this deployment belongs to. Required for the create payload
   * (POST /deployments/{id}/products needs projectId). Derived from
   * BeDeployment.projectId in the parent dialog. When absent (e.g. opened
   * from a case context where projectId is not populated), the Add button is
   * hidden.
   */
  projectId?: string;
}

interface Feedback {
  message: string;
  severity: "success" | "error";
}

/** SN sizing fields arrive as strings; treat null/blank uniformly as "—". */
function sizingValue(value?: string | null): string {
  return value?.trim() ? value : "—";
}

/**
 * List of the products deployed in a single deployment with write actions.
 * Loads via POST /deployments/{id}/products/search when mounted (lazy on
 * row expand). Engineers can add a new deployed product, edit sizing fields,
 * or deactivate an existing entry.
 */
export default function DeployedProductsPanel({
  deploymentId,
  projectId,
}: DeployedProductsPanelProps): JSX.Element {
  const { data, isLoading, isError, error } = useSearchDeployedProducts(deploymentId);
  const createDeployedProduct = useCreateDeployedProduct(deploymentId);
  const updateDeployedProduct = useUpdateDeployedProduct(deploymentId);

  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [menuTarget, setMenuTarget] = useState<BeDeployedProduct | null>(null);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<BeDeployedProduct | null>(null);
  const [deactivating, setDeactivating] = useState<BeDeployedProduct | null>(null);
  const [feedback, setFeedback] = useState<Feedback | null>(null);

  const products = data ?? [];

  const openMenu = (e: React.MouseEvent<HTMLElement>, p: BeDeployedProduct): void => {
    e.stopPropagation();
    setMenuAnchor(e.currentTarget);
    setMenuTarget(p);
  };
  const closeMenu = (): void => {
    setMenuAnchor(null);
    setMenuTarget(null);
  };

  const handleCreate = (payload: BeDeployedProductCreatePayload): void => {
    createDeployedProduct.mutate(payload, {
      onSuccess: () => {
        setCreating(false);
        setFeedback({ message: "Deployed product added.", severity: "success" });
      },
      onError: (err) => {
        setCreating(false);
        setFeedback({
          message: "Could not add deployed product: " + err.message,
          severity: "error",
        });
      },
    });
  };

  const handleSaveEdit = (payload: BeDeployedProductDetailUpdatePayload): void => {
    if (!editing) return;
    const productName = editing.product?.name ?? "this product";
    updateDeployedProduct.mutate(
      { deployedProductId: editing.id, payload },
      {
        onSuccess: () => {
          setEditing(null);
          setFeedback({ message: "Updated " + productName + ".", severity: "success" });
        },
        onError: (err) => {
          setEditing(null);
          setFeedback({
            message: "Could not update " + productName + ": " + err.message,
            severity: "error",
          });
        },
      },
    );
  };

  const handleDeactivate = (): void => {
    if (!deactivating) return;
    const target = deactivating;
    const productName = target.product?.name ?? "this product";
    updateDeployedProduct.mutate(
      { deployedProductId: target.id, payload: { active: false } },
      {
        onSuccess: () => {
          setDeactivating(null);
          setFeedback({ message: "Deactivated " + productName + ".", severity: "success" });
        },
        onError: (err) => {
          setDeactivating(null);
          setFeedback({
            message: "Could not deactivate " + productName + ": " + err.message,
            severity: "error",
          });
        },
      },
    );
  };

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", py: 3 }}>
        <CircularProgress size={20} />
      </Box>
    );
  }

  if (isError) {
    return (
      <QueryErrorState
        message={`Failed to load deployed products: ${error instanceof Error ? error.message : "unknown error"}`}
        error={error}
      />
    );
  }

  return (
    <Box sx={{ py: 1 }}>
      {feedback && (
        <Alert severity={feedback.severity} onClose={() => setFeedback(null)} sx={{ mb: 1 }}>
          {feedback.message}
        </Alert>
      )}

      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          px: 1,
          mb: 0.5,
        }}
      >
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
        >
          Deployed products
        </Typography>
        {projectId && (
          <Button
            size="small"
            variant="outlined"
            startIcon={<Plus size={14} />}
            onClick={() => setCreating(true)}
            sx={{ py: 0.25 }}
          >
            Add product
          </Button>
        )}
      </Box>

      {products.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ py: 1, px: 1 }}>
          No products deployed in this deployment.
        </Typography>
      ) : (
        <Table size="small" sx={{ mt: 0.5 }}>
          <TableHead>
            <TableRow>
              <TableCell>Product</TableCell>
              <TableCell>Version</TableCell>
              <TableCell>Support EOL</TableCell>
              <TableCell align="right">Cores</TableCell>
              <TableCell align="right">TPS</TableCell>
              <TableCell>Category</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {products.map((p) => (
              <TableRow key={p.id}>
                <TableCell>{p.product?.name || p.product?.id || "—"}</TableCell>
                <TableCell>
                  {p.version?.name ? (
                    <Chip size="small" variant="outlined" label={p.version.name} />
                  ) : (
                    "—"
                  )}
                </TableCell>
                <TableCell>{formatDeploymentDate(p.version?.supportEoLDate)}</TableCell>
                <TableCell align="right">{sizingValue(p.cores)}</TableCell>
                <TableCell align="right">{sizingValue(p.tps)}</TableCell>
                <TableCell>{p.category ?? "—"}</TableCell>
                <TableCell align="right">
                  <Tooltip title="Product actions">
                    <IconButton
                      size="small"
                      aria-label={"Actions for " + (p.product?.name ?? "deployed product")}
                      onClick={(e) => openMenu(e, p)}
                    >
                      <MoreVertical size={14} />
                    </IconButton>
                  </Tooltip>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      <Menu anchorEl={menuAnchor} open={!!menuAnchor} onClose={closeMenu}>
        <MenuItem
          onClick={() => {
            setEditing(menuTarget);
            closeMenu();
          }}
        >
          <Pencil size={14} style={{ marginRight: 8 }} />
          Edit details
        </MenuItem>
        <MenuItem
          onClick={() => {
            setDeactivating(menuTarget);
            closeMenu();
          }}
          sx={{ color: "error.main" }}
        >
          <Ban size={14} style={{ marginRight: 8 }} />
          Deactivate
        </MenuItem>
      </Menu>

      {creating && projectId && (
        <CreateDeployedProductDialog
          projectId={projectId}
          isSaving={createDeployedProduct.isPending}
          onClose={() => setCreating(false)}
          onCreate={handleCreate}
        />
      )}

      {editing && (
        <EditDeployedProductDialog
          deployedProduct={editing}
          isSaving={updateDeployedProduct.isPending}
          onClose={() => setEditing(null)}
          onSave={handleSaveEdit}
        />
      )}

      <Dialog
        open={!!deactivating}
        onClose={() => setDeactivating(null)}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>Deactivate deployed product</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Deactivate{" "}
            <Box component="span" sx={{ fontWeight: 600, color: "text.primary" }}>
              {deactivating?.product?.name ?? "this product"}
            </Box>
            ? This marks it inactive in ServiceNow.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => setDeactivating(null)}
            disabled={updateDeployedProduct.isPending}
          >
            Cancel
          </Button>
          <Button
            color="error"
            variant="contained"
            onClick={handleDeactivate}
            disabled={updateDeployedProduct.isPending}
          >
            Deactivate
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
