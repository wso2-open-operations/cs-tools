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
  Autocomplete,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  MenuItem,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useAddCatalogVersion } from "@features/csm-products/api/useAddCatalogVersion";
import { useGetCatalogVersions } from "@features/csm-products/api/useGetCatalogVersions";
import { useGetReleaseLines } from "@features/csm-products/api/useGetReleaseLines";
import VersionListingFields from "@features/csm-products/components/VersionListingFields";
import VersionSupportFields from "@features/csm-products/components/VersionSupportFields";
import type { CatalogProduct } from "@features/csm-products/types/productCatalog";
import {
  CARBON_KEYS,
  buildVersionCreatePayload,
  emptyVersionForm,
  validateVersionForm,
  type VersionFormValues,
} from "@features/csm-products/utils/catalogChange";
import {
  DEFAULT_PROFILE,
  latestLiveVersion,
  listingFormFrom,
  normalizeProfiles,
  productDisplayCode,
  type ListingFormValues,
} from "@features/csm-products/utils/catalogProducts";

interface AddVersionDialogProps {
  /** The products a version can be added to (cloud and retired products already left out). */
  products: CatalogProduct[];
  /** Preselected when opened from a product row. */
  initialProductId?: string;
  onClose: () => void;
  /** Called after the version was added, with a message for the success banner. */
  onSaved: (message: string) => void;
}

/** The note under Publishing: where the settings came from. */
function sourceSummary(source: { version: string } | null, edited: boolean): string {
  if (edited) return "Edited for this version.";
  return source ? `Copied from ${source.version}; change anything that differs.` : "No earlier version to copy from.";
}

/**
 * Add a version of a product: one row per deployment profile, sharing one
 * public listing. The dialog asks for the product, version, profiles and the
 * support fields, and shows the publishing settings below them, pre-filled
 * from the product's most recent live version so the new version publishes
 * like its siblings. Until the user edits them they follow the selected
 * product; once edited, the user's values are kept.
 */
export default function AddVersionDialog({ products, initialProductId, onClose, onSaved }: AddVersionDialogProps): JSX.Element {
  const [productId, setProductId] = useState(initialProductId ?? "");
  const [form, setForm] = useState<VersionFormValues>(emptyVersionForm);
  const [profiles, setProfiles] = useState<string[]>([DEFAULT_PROFILE]);
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Null while the publishing fields follow the selected product's latest version.
  const [advancedEdits, setAdvancedEdits] = useState<ListingFormValues | null>(null);
  const versions = useGetCatalogVersions(productId);
  const releaseLines = useGetReleaseLines();
  const add = useAddCatalogVersion();

  const product = products.find((p) => p.id === productId) ?? null;
  // No product picked, or its versions failed to load: nothing to copy the
  // advanced settings from (the dialog says so rather than prefilling blanks).
  const sourceFailed = Boolean(productId) && versions.isError;
  const latest = useMemo(
    () => (productId && !sourceFailed ? latestLiveVersion(versions.data ?? []) : null),
    [productId, sourceFailed, versions.data],
  );
  const prefilled = useMemo(() => listingFormFrom(latest), [latest]);
  const advanced = advancedEdits ?? prefilled;
  const showCarbon = Boolean(latest?.carbon) || CARBON_KEYS.some((key) => advanced.carbon[key].trim() !== "");
  const editAdvanced = (patch: Partial<ListingFormValues>): void => setAdvancedEdits({ ...advanced, ...patch });
  const profileOptions = useMemo(
    () =>
      normalizeProfiles([
        DEFAULT_PROFILE,
        ...(versions.data ?? []).map((v) => v.deploymentProfile ?? ""),
      ]),
    [versions.data],
  );
  const cleanProfiles = normalizeProfiles(profiles);
  const errors: Record<string, string> = {
    ...validateVersionForm({ ...form, ...advanced }, "create", { productCode: product?.code ?? null }),
  };
  if (!productId) errors.product = "Pick a product.";
  if (cleanProfiles.length === 0) errors.profiles = "Add at least one deployment profile.";
  const loadingSource = Boolean(productId) && versions.isLoading;

  const handleSave = async (): Promise<void> => {
    setSubmitted(true);
    if (Object.keys(errors).length > 0 || !product) return;
    setError(null);
    const payload = buildVersionCreatePayload({ ...form, ...advanced, deploymentProfile: "" });
    try {
      const result = await add.mutateAsync({ productId: product.id, payload, profiles: cleanProfiles });
      const what = `${productDisplayCode(product)} ${form.version.trim()}`;
      onSaved(result.created > 1 ? `Added ${what} (${result.created} profiles).` : `Added ${what}.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to add the version.");
    }
  };

  return (
    <Dialog open onClose={add.isPending ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Add product version</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
          <TextField
            select
            required
            size="small"
            fullWidth
            label="Product"
            value={productId}
            onChange={(event) => setProductId(event.target.value)}
            error={submitted && Boolean(errors.product)}
            helperText={submitted ? errors.product : undefined}
          >
            {products.map((p) => (
              <MenuItem key={p.id} value={p.id}>
                {productDisplayCode(p)} — {p.name}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            required
            size="small"
            fullWidth
            label="Version"
            value={form.version}
            onChange={(event) => setForm((prev) => ({ ...prev, version: event.target.value }))}
            error={submitted && Boolean(errors.version)}
            helperText={submitted ? errors.version : undefined}
          />
          <Autocomplete
            multiple
            freeSolo
            size="small"
            options={profileOptions}
            value={profiles}
            onChange={(_event, next) => setProfiles(next.map(String))}
            renderTags={(value, getTagProps) =>
              value.map((option, index) => {
                const { key, ...tagProps } = getTagProps({ index });
                return <Chip key={key} size="small" label={option} {...tagProps} />;
              })
            }
            renderInput={(params) => (
              <TextField
                {...params}
                label="Deployment profiles"
                error={submitted && Boolean(errors.profiles)}
                helperText={
                  (submitted && errors.profiles) || "One row per profile, all in one public listing. Type to add a new one."
                }
              />
            )}
          />
          <VersionSupportFields
            form={form}
            onChange={(key, value) => setForm((prev) => ({ ...prev, [key]: value }))}
            errors={errors}
            showErrors={submitted}
          />
          {sourceFailed && (
            <Alert severity="warning">
              Could not load this product&apos;s versions to copy settings from; check the publishing settings.
            </Alert>
          )}
          <Box>
            <Typography variant="subtitle2">Publishing</Typography>
            <Typography variant="caption" color="text.secondary">
              {!productId && advancedEdits === null
                ? "Pick a product to copy its latest version's settings."
                : loadingSource
                  ? "Checking how this product publishes…"
                  : sourceSummary(latest, advancedEdits !== null)}
            </Typography>
          </Box>
          <VersionListingFields
            values={advanced}
            onChange={editAdvanced}
            releaseLines={releaseLines.data ?? []}
            errors={errors}
            showErrors={submitted}
            showCarbon={showCarbon}
          />
          {error && <Alert severity="error">{error}</Alert>}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={add.isPending}>
          Cancel
        </Button>
        <Button variant="contained" onClick={() => void handleSave()} disabled={add.isPending || loadingSource}>
          {add.isPending ? "Adding…" : "Add version"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
