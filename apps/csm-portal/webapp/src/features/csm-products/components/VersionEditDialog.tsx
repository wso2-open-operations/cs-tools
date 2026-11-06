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
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import { useCatalogWrite } from "@features/csm-products/api/useCatalogWrite";
import VersionSupportFields from "@features/csm-products/components/VersionSupportFields";
import type { CatalogProduct, CatalogVersion } from "@features/csm-products/types/productCatalog";
import {
  buildVersionSupportPayload,
  validateVersionForm,
  versionToForm,
  type VersionFormValues,
} from "@features/csm-products/utils/catalogChange";
import { productDisplayCode } from "@features/csm-products/utils/catalogProducts";

interface VersionEditDialogProps {
  product: CatalogProduct;
  /** Every live deployment-profile row of the listing, the one shown first. */
  rows: CatalogVersion[];
  /** The listing's public name, when the version has several listings to tell apart. */
  listingName?: string | null;
  onClose: () => void;
  /** Called after a change was applied, with a message for the success banner. */
  onSaved: (message: string) => void;
}

/**
 * Edit a version's support status and end-of-life dates. Saving applies the
 * change at once: one PATCH to the row shown, with only the support fields
 * that differ from it, which entity-service applies to every live deployment
 * profile of the version in one transaction. Retiring is here too, behind an
 * in-dialog confirmation step: one call that retires every profile. Retired
 * rows are not listed, so there is no Restore.
 */
export default function VersionEditDialog({
  product,
  rows,
  listingName,
  onClose,
  onSaved,
}: VersionEditDialogProps): JSX.Element {
  const version = rows[0];
  const [form, setForm] = useState<VersionFormValues>(() => versionToForm(version));
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmingRetire, setConfirmingRetire] = useState(false);
  const save = useCatalogWrite();
  const pending = save.isPending;
  const errors = validateVersionForm(form, "update");
  const name = `${productDisplayCode(product)} ${version.version}${listingName ? ` (${listingName})` : ""}`;

  const handleSave = async (): Promise<void> => {
    setSubmitted(true);
    if (Object.keys(errors).length > 0) return;
    setError(null);
    const payload = buildVersionSupportPayload(version, form);
    if (Object.keys(payload).length === 0) {
      setError("Nothing has changed.");
      return;
    }
    try {
      await save.mutateAsync({ kind: "updateVersion", id: version.id, payload });
      onSaved(`Saved ${name}.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save the change.");
    }
  };

  const handleRetire = async (): Promise<void> => {
    setError(null);
    try {
      // One call: entity-service retires every live profile of the version.
      await save.mutateAsync({ kind: "retireVersion", id: version.id });
      onSaved(`Retired ${name}.`);
    } catch (err) {
      setConfirmingRetire(false);
      setError(err instanceof Error ? err.message : "Failed to save the change.");
    }
  };

  return (
    <Dialog open onClose={pending ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Edit {name}</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
          {rows.length > 1 && (
            <Alert severity="info">
              Changes apply to all {rows.length} deployment profiles of this version (
              {rows.map((row) => row.deploymentProfile ?? "no profile").join(", ")}).
            </Alert>
          )}
          <VersionSupportFields
            form={form}
            onChange={(key, value) => setForm((prev) => ({ ...prev, [key]: value }))}
            errors={errors}
            showErrors={submitted}
            showReleaseDate={false}
          />
          {error && <Alert severity="error">{error}</Alert>}
        </Box>
      </DialogContent>
      {confirmingRetire ? (
        <DialogActions sx={{ flexWrap: "wrap", gap: 1 }}>
          <Typography variant="body2" sx={{ mr: "auto" }}>
            Retire this version{rows.length > 1 ? ` and its ${rows.length} deployment profiles` : ""}? It drops off the
            catalogue and the public feeds now; deployments that use it keep referencing it.
          </Typography>
          <Button onClick={() => setConfirmingRetire(false)} disabled={pending}>
            Back
          </Button>
          <Button
            variant="contained"
            color="error"
            onClick={() => void handleRetire()}
            disabled={pending}
          >
            Retire
          </Button>
        </DialogActions>
      ) : (
        <DialogActions>
          <Button
            color="error"
            onClick={() => {
              setError(null);
              setConfirmingRetire(true);
            }}
            disabled={pending}
            sx={{ mr: "auto" }}
          >
            Retire this version
          </Button>
          <Button onClick={onClose} disabled={pending}>
            Cancel
          </Button>
          <Button variant="contained" onClick={() => void handleSave()} disabled={pending}>
            {pending ? "Saving…" : "Save"}
          </Button>
        </DialogActions>
      )}
    </Dialog>
  );
}
