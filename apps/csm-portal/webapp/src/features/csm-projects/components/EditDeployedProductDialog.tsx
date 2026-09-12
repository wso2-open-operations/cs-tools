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
  Tab,
  Tabs,
  TextField,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type {
  BeDeployedProduct,
  BeDeployedProductDetailUpdatePayload,
  BeProductUpdate,
} from "@api/backend/types";
import UpdateHistoryPanel, {
  type UpdateHistoryFormState,
} from "@features/csm-projects/components/UpdateHistoryPanel";

interface EditDeployedProductDialogProps {
  deployedProduct: BeDeployedProduct;
  /** True while the Details-tab PATCH is in flight; disables Details fields/Save. */
  isSaving: boolean;
  onClose: () => void;
  /** Persist the changed detail fields only (only changed fields are sent). */
  onSaveDetails: (payload: BeDeployedProductDetailUpdatePayload) => void;
  /**
   * Persist the whole update-history array immediately. Independent of
   * {@link onSaveDetails} — does not close the dialog either way; the caller
   * resolves/rejects based on the PATCH result so the history panel can show
   * inline feedback.
   */
  onSaveHistory: (updates: BeProductUpdate[]) => Promise<void>;
}

const DESCRIPTION_MAX = 4000;

function updateHistoryFooterLabel(action: UpdateHistoryFormState["saveAction"]): string {
  switch (action) {
    case "delete":
      return "Deleting Update...";
    case "edit":
      return "Saving Update...";
    default:
      return "Adding...";
  }
}

/**
 * Edit a deployed product: its cores/tps/description (Details tab) and its
 * update-level history (Update History tab), via
 * `PATCH /deployments/{deploymentId}/products/{productId}` (detail variant).
 *
 * The two tabs are independent saves, matching customer-portal's
 * `ManageProductModal`/`UpdateHistoryTab` interaction:
 *  - Details tab: only changed fields are sent (BE requires minProperties 1);
 *    Save is disabled until at least one field differs. On success the
 *    dialog closes.
 *  - Update History tab: add/edit/delete each PATCH the whole resulting
 *    array immediately (there is no per-entry endpoint) via
 *    {@link UpdateHistoryPanel}; the dialog stays open either way, showing
 *    inline feedback in the panel itself.
 *
 * The footer's action button switches with the active tab: "Save changes"
 * on Details, "Add update" (label reflecting the in-flight action) on Update
 * History — the latter is bound to the history panel's own add-form submit
 * via state the panel lifts up through {@link onFormStateChange}.
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
  onSaveDetails,
  onSaveHistory,
}: EditDeployedProductDialogProps): JSX.Element {
  const [tab, setTab] = useState(0);

  // --- Details tab state -----------------------------------------------
  // `description` is not in the read schema (DeployedProduct in openapi.yaml
  // does not carry it back) — initialize as empty so we can only set/clear it.
  const originalCores = deployedProduct.cores ?? null;
  const originalTps = deployedProduct.tps ?? null;
  const originalDescription = "";

  const [cores, setCores] = useState(originalCores === null ? "" : String(originalCores));
  const [tps, setTps] = useState(originalTps === null ? "" : String(originalTps));
  const [description, setDescription] = useState(originalDescription);

  const coresNum = cores.trim() === "" ? null : Number(cores);
  const tpsNum = tps.trim() === "" ? null : Number(tps);
  const coresError =
    cores.trim() !== "" &&
    (!Number.isInteger(coresNum) || (coresNum as number) < 0);
  const tpsError =
    tps.trim() !== "" &&
    (isNaN(tpsNum as number) || (tpsNum as number) < 0);

  const coresChanged = coresNum !== originalCores;
  const tpsChanged = tpsNum !== originalTps;
  const descriptionChanged = description.trim() !== originalDescription;

  const detailsPayload: BeDeployedProductDetailUpdatePayload = {};
  if (coresChanged) detailsPayload.cores = coresNum;
  if (tpsChanged) detailsPayload.tps = tpsNum;
  if (descriptionChanged) {
    detailsPayload.description = description.trim().length > 0 ? description.trim() : null;
  }

  const canSaveDetails =
    !isSaving &&
    !coresError &&
    !tpsError &&
    (coresChanged || tpsChanged || descriptionChanged);

  // --- Update History tab state -----------------------------------------
  const updates = deployedProduct.updates ?? [];
  const [historyFormState, setHistoryFormState] = useState<UpdateHistoryFormState | null>(null);

  const anySaving = isSaving || (historyFormState?.isSaving ?? false);

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Edit deployed product</DialogTitle>
      <Tabs value={tab} onChange={(_e, v: number) => setTab(v)} sx={{ px: 3, minHeight: 36 }}>
        <Tab label="Details" id="edit-deployed-product-tab-details" />
        <Tab label="Update History" id="edit-deployed-product-tab-history" />
      </Tabs>
      <DialogContent dividers>
        {tab === 0 && (
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
              disabled={isSaving}
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
              disabled={isSaving}
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
              disabled={isSaving}
            />
          </Box>
        )}

        {tab === 1 && (
          <UpdateHistoryPanel
            updates={updates}
            onSaveUpdates={onSaveHistory}
            onFormStateChange={setHistoryFormState}
          />
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={anySaving}>
          Cancel
        </Button>
        {tab === 0 && (
          <Button
            variant="contained"
            disabled={!canSaveDetails}
            onClick={() => onSaveDetails(detailsPayload)}
          >
            Save changes
          </Button>
        )}
        {tab === 1 && historyFormState && (
          <Button
            variant="contained"
            disabled={!historyFormState.canAdd}
            onClick={historyFormState.handleAdd}
          >
            {historyFormState.isSaving
              ? updateHistoryFooterLabel(historyFormState.saveAction)
              : "Add update"}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
