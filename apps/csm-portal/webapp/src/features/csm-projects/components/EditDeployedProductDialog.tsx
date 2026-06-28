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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import type {
  BeDeployedProduct,
  BeDeployedProductDetailUpdatePayload,
} from "@api/backend/types";

interface EditDeployedProductDialogProps {
  deployedProduct: BeDeployedProduct;
  /** True while the PATCH is in flight; disables actions. */
  isSaving: boolean;
  onClose: () => void;
  /** Persist the changed detail fields (only changed fields are sent). */
  onSave: (payload: BeDeployedProductDetailUpdatePayload) => void;
}

const DESCRIPTION_MAX = 4000;

/**
 * Edit a deployed product's cores, tps, and description
 * (`PATCH /deployments/{deploymentId}/products/{productId}` detail variant).
 *
 * Only changed fields are sent — the BE requires minProperties 1 for this
 * variant, so Save is disabled until at least one field differs. Clearing a
 * field sends `null` to explicitly remove the value.
 *
 * Deactivation is a separate concern handled via a confirm dialog in
 * {@link DeployedProductsPanel}, not here — keeping the two BE shapes distinct
 * avoids any chance of accidentally mixing `active` with detail fields.
 *
 * Mount only while open.
 */
export default function EditDeployedProductDialog({
  deployedProduct,
  isSaving,
  onClose,
  onSave,
}: EditDeployedProductDialogProps): JSX.Element {
  // Existing SN cores/tps arrive as strings (or null). Parse to number for the
  // input; the payload sends numbers per the openapi spec.
  // `description` is not in the read schema (DeployedProduct in openapi.yaml
  // does not carry it back) — initialize as empty so we can only set/clear it.
  const originalCoresStr = deployedProduct.cores?.trim() ?? "";
  const originalTpsStr = deployedProduct.tps?.trim() ?? "";
  const originalDescription = "";

  const [cores, setCores] = useState(originalCoresStr);
  const [tps, setTps] = useState(originalTpsStr);
  const [description, setDescription] = useState(originalDescription);

  const coresNum = cores.trim() === "" ? null : Number(cores);
  const tpsNum = tps.trim() === "" ? null : Number(tps);
  const coresError =
    cores.trim() !== "" &&
    (!Number.isInteger(coresNum) || (coresNum as number) < 0);
  const tpsError =
    tps.trim() !== "" &&
    (isNaN(tpsNum as number) || (tpsNum as number) < 0);

  const coresChanged = cores.trim() !== originalCoresStr;
  const tpsChanged = tps.trim() !== originalTpsStr;
  const descriptionChanged = description.trim() !== originalDescription;

  const payload = useMemo<BeDeployedProductDetailUpdatePayload>(() => {
    const next: Record<string, unknown> = {};
    if (coresChanged) next.cores = coresNum;
    if (tpsChanged) next.tps = tpsNum;
    if (descriptionChanged) {
      next.description = description.trim().length > 0 ? description.trim() : null;
    }
    return next as BeDeployedProductDetailUpdatePayload;
  }, [coresChanged, tpsChanged, descriptionChanged, coresNum, tpsNum, description]);

  const canSave =
    !isSaving &&
    !coresError &&
    !tpsError &&
    (coresChanged || tpsChanged || descriptionChanged);

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Edit deployed product</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          <TextField
            label="Cores"
            value={cores}
            onChange={(e) => setCores(e.target.value)}
            size="small"
            fullWidth
            type="number"
            autoFocus
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
        <Button variant="contained" disabled={!canSave} onClick={() => onSave(payload)}>
          Save changes
        </Button>
      </DialogActions>
    </Dialog>
  );
}
