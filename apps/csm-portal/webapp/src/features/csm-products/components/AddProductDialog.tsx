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
  MenuItem,
  TextField,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import { useCatalogWrite } from "@features/csm-products/api/useCatalogWrite";
import {
  PRODUCT_BUSINESS_UNIT_OPTIONS,
  PRODUCT_MANUFACTURER_OPTIONS,
  PRODUCT_UNIT_OPTIONS,
  type CatalogOption,
} from "@features/csm-products/utils/catalogFormat";
import {
  buildProductCreatePayload,
  emptyProductForm,
  validateProductForm,
  type ProductFormValues,
} from "@features/csm-products/utils/catalogProducts";

interface AddProductDialogProps {
  onClose: () => void;
  /** Called after the product was created, with a message for the success banner. */
  onSaved: (message: string) => void;
}

/** Create a product; saving applies at once. Versions are added separately. */
export default function AddProductDialog({ onClose, onSaved }: AddProductDialogProps): JSX.Element {
  const [form, setForm] = useState<ProductFormValues>(emptyProductForm);
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const save = useCatalogWrite();
  const errors = validateProductForm(form);

  const handleSave = async (): Promise<void> => {
    setSubmitted(true);
    if (Object.keys(errors).length > 0) return;
    setError(null);
    try {
      await save.mutateAsync({ kind: "createProduct", payload: buildProductCreatePayload(form) });
      onSaved(`Added ${form.name.trim()}.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to add the product.");
    }
  };

  const text = (key: keyof ProductFormValues, label: string) => ({
    label,
    value: form[key],
    onChange: (event: { target: { value: string } }) => setForm((prev) => ({ ...prev, [key]: event.target.value })),
    error: submitted && Boolean(errors[key]),
    helperText: submitted ? errors[key] : undefined,
    size: "small" as const,
    fullWidth: true,
  });
  const options = (list: readonly CatalogOption[], emptyLabel?: string) => [
    ...(emptyLabel ? [<MenuItem key="" value="">{emptyLabel}</MenuItem>] : []),
    ...list.map((o) => (
      <MenuItem key={o.value} value={o.value}>
        {o.label}
      </MenuItem>
    )),
  ];

  return (
    <Dialog open onClose={save.isPending ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Add product</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" }, pt: 1 }}>
          <TextField required {...text("name", "Name")} sx={{ gridColumn: { sm: "1 / -1" } }} />
          <TextField {...text("code", "Code")} />
          <TextField select required {...text("class", "Class")}>
            <MenuItem value="software">Software</MenuItem>
            <MenuItem value="service">Service</MenuItem>
          </TextField>
          <TextField select {...text("unit", "Unit")}>
            {options(PRODUCT_UNIT_OPTIONS, "Not set")}
          </TextField>
          <TextField select {...text("businessUnit", "Business unit")}>
            {options(PRODUCT_BUSINESS_UNIT_OPTIONS, "Not set")}
          </TextField>
          <TextField select {...text("manufacturer", "Manufacturer")}>
            {options(PRODUCT_MANUFACTURER_OPTIONS, "Not set")}
          </TextField>
          <TextField {...text("description", "Description")} multiline minRows={2} sx={{ gridColumn: { sm: "1 / -1" } }} />
          {error && (
            <Alert severity="error" sx={{ gridColumn: "1 / -1" }}>
              {error}
            </Alert>
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={save.isPending}>
          Cancel
        </Button>
        <Button variant="contained" onClick={() => void handleSave()} disabled={save.isPending}>
          {save.isPending ? "Adding…" : "Add product"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
