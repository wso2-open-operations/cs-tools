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
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormHelperText,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type { BeDeployedProductCreatePayload } from "@api/backend/types";
import { useSearchProducts } from "@features/csm-projects/api/useSearchProducts";
import { useSearchProductVersions } from "@features/csm-projects/api/useSearchProductVersions";

interface CreateDeployedProductDialogProps {
  /** The project this deployed product belongs to (required by the BE contract). */
  projectId: string;
  /** True while the POST is in flight; disables actions. */
  isSaving: boolean;
  onClose: () => void;
  /** Submit the create payload via `POST /deployments/{id}/products`. */
  onCreate: (payload: BeDeployedProductCreatePayload) => void;
}

const DESCRIPTION_MAX = 4000;

/**
 * Create a deployed product under the current deployment. Product and version
 * are required; cores, tps, and description are optional sizing fields.
 *
 * Product list comes from `POST /products/search` (full catalogue, bounded).
 * Version list is dependent on the selected product and comes from
 * `POST /products/{id}/versions/search` (lazy-loaded after product pick).
 *
 * Mount only while open.
 */
export default function CreateDeployedProductDialog({
  projectId,
  isSaving,
  onClose,
  onCreate,
}: CreateDeployedProductDialogProps): JSX.Element {
  const [productId, setProductId] = useState("");
  const [versionId, setVersionId] = useState("");
  const [cores, setCores] = useState("");
  const [tps, setTps] = useState("");
  const [description, setDescription] = useState("");

  const { data: products, isLoading: productsLoading } = useSearchProducts();
  const { data: versions, isLoading: versionsLoading } = useSearchProductVersions(
    productId || undefined,
  );

  const productError = productId === "";
  const versionError = versionId === "";
  // Validate optional numeric fields only when the user has typed something.
  const coresNum = cores.trim() === "" ? undefined : Number(cores);
  const tpsNum = tps.trim() === "" ? undefined : Number(tps);
  const coresError = cores.trim() !== "" && (!Number.isInteger(coresNum) || (coresNum as number) < 0);
  const tpsError = tps.trim() !== "" && (isNaN(tpsNum as number) || (tpsNum as number) < 0);

  const canSave =
    !isSaving &&
    !productError &&
    !versionError &&
    !coresError &&
    !tpsError;

  const handleCreate = (): void => {
    if (!canSave) return;
    const payload: BeDeployedProductCreatePayload = {
      projectId,
      productId,
      versionId,
    };
    if (coresNum !== undefined) payload.cores = coresNum;
    if (tpsNum !== undefined) payload.tps = tpsNum;
    const trimmedDescription = description.trim();
    if (trimmedDescription) payload.description = trimmedDescription;
    onCreate(payload);
  };

  // When the product changes, reset the version so the user must pick again.
  const handleProductChange = (newProductId: string): void => {
    setProductId(newProductId);
    setVersionId("");
  };

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Add deployed product</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {/* Product picker */}
          <FormControl size="small" fullWidth required error={productId === "" && tps.length > 0}>
            <InputLabel id="create-dp-product-label">Product</InputLabel>
            <Select
              labelId="create-dp-product-label"
              label="Product"
              value={productId}
              onChange={(e) => handleProductChange(e.target.value as string)}
              startAdornment={
                productsLoading ? <CircularProgress size={16} sx={{ mr: 1 }} /> : null
              }
            >
              {(products ?? []).map((p) => (
                <MenuItem key={p.id} value={p.id}>
                  {p.name ?? p.id}
                </MenuItem>
              ))}
            </Select>
            {productId === "" && tps.length > 0 && (
              <FormHelperText>Product is required.</FormHelperText>
            )}
          </FormControl>

          {/* Version picker — disabled until a product is selected */}
          <FormControl
            size="small"
            fullWidth
            required
            disabled={!productId}
            error={productId !== "" && versionId === "" && description.length > 0}
          >
            <InputLabel id="create-dp-version-label">Version</InputLabel>
            <Select
              labelId="create-dp-version-label"
              label="Version"
              value={versionId}
              onChange={(e) => setVersionId(e.target.value as string)}
              startAdornment={
                versionsLoading ? <CircularProgress size={16} sx={{ mr: 1 }} /> : null
              }
            >
              {(versions ?? []).map((v) => (
                <MenuItem key={v.id} value={v.id}>
                  {v.version ?? v.id}
                </MenuItem>
              ))}
            </Select>
            {!productId && (
              <FormHelperText>Select a product first.</FormHelperText>
            )}
            {productId !== "" && versionId === "" && description.length > 0 && (
              <FormHelperText>Version is required.</FormHelperText>
            )}
            {productId !== "" && !versionsLoading && (versions ?? []).length === 0 && (
              <FormHelperText>
                <Typography variant="inherit" component="span" color="text.secondary">
                  No versions available for this product.
                </Typography>
              </FormHelperText>
            )}
          </FormControl>

          {/* Optional sizing fields */}
          <TextField
            label="Cores"
            value={cores}
            onChange={(e) => setCores(e.target.value)}
            size="small"
            fullWidth
            type="number"
            slotProps={{ htmlInput: { min: 0, step: 1 } }}
            error={coresError}
            helperText={coresError ? "Must be a non-negative integer." : " "}
          />

          <TextField
            label="TPS"
            value={tps}
            onChange={(e) => setTps(e.target.value)}
            size="small"
            fullWidth
            type="number"
            slotProps={{ htmlInput: { min: 0, step: 0.1 } }}
            error={tpsError}
            helperText={tpsError ? "Must be a non-negative number." : " "}
          />

          <TextField
            label="Description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            size="small"
            fullWidth
            multiline
            minRows={2}
            slotProps={{ htmlInput: { maxLength: DESCRIPTION_MAX } }}
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button variant="contained" disabled={!canSave} onClick={handleCreate}>
          Add product
        </Button>
      </DialogActions>
    </Dialog>
  );
}
