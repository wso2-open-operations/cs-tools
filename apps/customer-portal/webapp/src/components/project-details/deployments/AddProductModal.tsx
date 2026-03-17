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
  IconButton,
  MenuItem,
  Skeleton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ChangeEvent,
  type JSX,
} from "react";
import { useGetProducts } from "@api/useGetProducts";
import { useSearchProductVersions } from "@api/useSearchProductVersions";
import { usePostDeploymentProduct } from "@api/usePostDeploymentProduct";
import type { ProductItem, ProductVersionItem } from "@models/responses";

export interface AddProductModalProps {
  open: boolean;
  deploymentId: string;
  projectId: string;
  onClose: () => void;
  onSuccess?: () => void;
  onError?: (message: string) => void;
}

const INITIAL_FORM = {
  productId: "",
  versionId: "",
  cores: "",
  tps: "",
  description: "",
};

/**
 * Helper to parse and validate a numeric string value.
 * Returns a valid number or undefined if invalid.
 */
function parseValidNumber(value: string): number | undefined {
  if (!value || !value.trim()) return undefined;
  const num = Number(value);
  return Number.isFinite(num) ? num : undefined;
}

/**
 * Modal for adding a WSO2 product to a deployment environment.
 * Product Name and Version come from APIs; Description and Initial Update Info are disabled.
 *
 * @param {AddProductModalProps} props - open, deploymentId, projectId, onClose, optional onSuccess/onError.
 * @returns {JSX.Element} The add product modal.
 */
export default function AddProductModal({
  open,
  deploymentId,
  projectId,
  onClose,
  onSuccess,
  onError,
}: AddProductModalProps): JSX.Element {
  const [form, setForm] = useState(INITIAL_FORM);
  const [productOffset, setProductOffset] = useState(0);
  const [versionOffset, setVersionOffset] = useState(0);
  const [products, setProducts] = useState<ProductItem[]>([]);
  const [versions, setVersions] = useState<ProductVersionItem[]>([]);

  const {
    data: productsPage,
    isLoading: isLoadingProducts,
    isFetching: isFetchingProducts,
  } = useGetProducts({
    offset: productOffset,
    limit: 10,
  });

  const {
    data: versionsPage,
    isLoading: isLoadingVersions,
    isFetching: isFetchingVersions,
  } = useSearchProductVersions(form.productId, {
    limit: 10,
    offset: versionOffset,
  });

  useEffect(() => {
    if (!productsPage) return;
    const pageItems = productsPage.products ?? [];
    const offset = productsPage.offset ?? 0;

    Promise.resolve().then(() => {
      if (offset === 0) {
        setProducts(pageItems);
        return;
      }
      setProducts((prev) => {
        const existingIds = new Set(prev.map((p) => p.id));
        const next = [...prev];
        pageItems.forEach((item) => {
          if (!existingIds.has(item.id)) {
            next.push(item);
            existingIds.add(item.id);
          }
        });
        return next;
      });
    });
  }, [productsPage]);

  const productsTotalRecords = productsPage?.totalRecords ?? products.length;
  const canLoadMoreProducts = products.length < productsTotalRecords;

  const handleProductsScroll = useCallback(
    (event: React.UIEvent<HTMLDivElement>) => {
      const target = event.currentTarget;
      if (
        !canLoadMoreProducts ||
        isLoadingProducts ||
        isFetchingProducts ||
        products.length === 0
      ) {
        return;
      }

      const threshold = 24; // px from bottom to trigger load
      if (
        target.scrollHeight - target.scrollTop - target.clientHeight <
        threshold
      ) {
        setProductOffset((prev) => prev + 10);
      }
    },
    [
      canLoadMoreProducts,
      isLoadingProducts,
      isFetchingProducts,
      products.length,
    ],
  );

  useEffect(() => {
    if (!versionsPage) return;
    const pageItems = versionsPage.versions ?? [];
    const offset = versionsPage.offset ?? 0;

    if (pageItems.length === 0) {
      return;
    }

    Promise.resolve().then(() => {
      if (offset === 0) {
        setVersions(pageItems);
        return;
      }
      setVersions((prev) => {
        const existingIds = new Set(prev.map((v) => v.id));
        const next = [...prev];
        pageItems.forEach((item) => {
          if (!existingIds.has(item.id)) {
            next.push(item);
            existingIds.add(item.id);
          }
        });
        return next;
      });
    });
  }, [versionsPage]);

  // Reset versions when product changes.
  useEffect(() => {
    Promise.resolve().then(() => {
      setVersions([]);
      setVersionOffset(0);
    });
  }, [form.productId]);

  const versionsTotalRecords = versionsPage?.totalRecords ?? versions.length;
  const canLoadMoreVersions = versions.length < versionsTotalRecords;

  const handleVersionsScroll = useCallback(
    (event: React.UIEvent<HTMLDivElement>) => {
      const target = event.currentTarget;
      if (
        !canLoadMoreVersions ||
        isLoadingVersions ||
        isFetchingVersions ||
        versions.length === 0
      ) {
        return;
      }

      const threshold = 24;
      if (
        target.scrollHeight - target.scrollTop - target.clientHeight <
        threshold
      ) {
        setVersionOffset((prev) => prev + 10);
      }
    },
    [
      canLoadMoreVersions,
      isLoadingVersions,
      isFetchingVersions,
      versions.length,
    ],
  );

  // Sort versions in ascending order
  const sortedVersions = useMemo(() => {
    return [...versions].sort((a, b) => {
      return a.version.localeCompare(b.version, undefined, {
        numeric: true,
        sensitivity: "base",
      });
    });
  }, [versions]);

  const postProduct = usePostDeploymentProduct();

  const isSubmitting = postProduct.isPending;
  const isValid =
    !!form.productId &&
    !!form.versionId &&
    !!projectId &&
    deploymentId.length > 0;

  const handleClose = useCallback(() => {
    setForm(INITIAL_FORM);
    onClose();
  }, [onClose]);

  const handleProductChange = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      const productId = event.target.value;
      setForm((prev) => ({
        ...prev,
        productId,
        versionId: "",
      }));
    },
    [],
  );

  const handleVersionChange = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      setForm((prev) => ({ ...prev, versionId: event.target.value }));
    },
    [],
  );

  const handleTextChange =
    (field: "cores" | "tps" | "description") =>
    (event: ChangeEvent<HTMLInputElement>) => {
      setForm((prev) => ({ ...prev, [field]: event.target.value }));
    };

  const handleSubmit = useCallback(async () => {
    if (!isValid) return;

    try {
      await postProduct.mutateAsync({
        deploymentId,
        body: {
          productId: form.productId,
          versionId: form.versionId,
          projectId,
          cores: parseValidNumber(form.cores),
          tps: parseValidNumber(form.tps),
          description: form.description || undefined,
        },
      });
      handleClose();
      onSuccess?.();
    } catch (error) {
      onError?.(
        error instanceof Error ? error.message : "Failed to add product",
      );
    }
  }, [
    isValid,
    form.productId,
    form.versionId,
    form.cores,
    form.tps,
    form.description,
    deploymentId,
    projectId,
    postProduct,
    handleClose,
    onSuccess,
    onError,
  ]);

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="add-product-dialog-title"
      aria-describedby="add-product-dialog-description"
    >
      <DialogTitle
        id="add-product-dialog-title"
        sx={{ pr: 6, position: "relative", pb: 0.5 }}
      >
        Add WSO2 Product
        <Typography
          id="add-product-dialog-description"
          variant="body2"
          color="text.secondary"
          sx={{ mt: 0.5, fontWeight: "normal", fontSize: "0.875rem" }}
        >
          Add a WSO2 product to this deployment environment.
        </Typography>
        <IconButton
          aria-label="close"
          onClick={handleClose}
          sx={{ position: "absolute", right: 12, top: 12 }}
          size="small"
        >
          <X size={20} aria-hidden />
        </IconButton>
      </DialogTitle>

      <DialogContent
        sx={{
          pt: 1,
          "& .MuiInputBase-input::placeholder": {
            color: "text.secondary",
            opacity: 1,
          },
        }}
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: "1fr 1fr",
            gap: 2,
            mt: 2,
            mb: 2,
          }}
        >
          <TextField
            select
            fullWidth
            size="small"
            id="product-name"
            label="Product Name *"
            value={form.productId}
            onChange={handleProductChange}
            disabled={isSubmitting || isLoadingProducts}
            sx={{
              "& .MuiSelect-select": {
                color: !form.productId ? "text.secondary" : undefined,
              },
            }}
            SelectProps={{
              MenuProps: {
                PaperProps: {
                  onScroll: handleProductsScroll,
                },
              },
            }}
          >
            <MenuItem value="">Select</MenuItem>
            {products.map((p) => (
              <MenuItem key={p.id} value={p.id}>
                {p.label ?? p.name ?? p.id}
              </MenuItem>
            ))}
            {(isLoadingProducts || isFetchingProducts) &&
              products.length === 0 && (
                <MenuItem disabled>
                  <Skeleton variant="text" width="100%" />
                </MenuItem>
              )}
          </TextField>
          <TextField
            select
            fullWidth
            size="small"
            id="product-version"
            label="Version *"
            value={form.versionId}
            onChange={handleVersionChange}
            disabled={isSubmitting || !form.productId || isLoadingVersions}
            sx={{
              "& .MuiSelect-select": {
                color: !form.versionId ? "text.secondary" : undefined,
              },
            }}
            SelectProps={{
              MenuProps: {
                PaperProps: {
                  onScroll: handleVersionsScroll,
                },
              },
            }}
          >
            <MenuItem value="">Select</MenuItem>
            {sortedVersions.map((v) => (
              <MenuItem key={v.id} value={v.id}>
                {v.version}
              </MenuItem>
            ))}
            {(isLoadingVersions || isFetchingVersions) &&
              versions.length === 0 && (
                <MenuItem disabled>
                  <Skeleton variant="text" width="100%" />
                </MenuItem>
              )}
          </TextField>
        </Box>

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: "1fr 1fr",
            gap: 2,
            mb: 2,
          }}
        >
          <TextField
            id="product-cores"
            label="Core Count"
            placeholder="e.g., 8"
            type="number"
            value={form.cores}
            onChange={handleTextChange("cores")}
            fullWidth
            size="small"
            disabled={isSubmitting}
            inputProps={{ min: 0 }}
          />
          <TextField
            id="product-tps"
            label="TPS (Transactions Per Second)"
            placeholder="e.g., 5000"
            type="number"
            value={form.tps}
            onChange={handleTextChange("tps")}
            fullWidth
            size="small"
            disabled={isSubmitting}
            inputProps={{ min: 0 }}
          />
        </Box>

        <TextField
          id="product-description"
          label="Description"
          placeholder="Enter Description..."
          fullWidth
          size="small"
          multiline
          rows={2}
          sx={{ mb: 2 }}
          value={form.description}
          onChange={handleTextChange("description")}
          disabled={isSubmitting}
        />

        <Box sx={{ borderTop: 1, borderColor: "divider", pt: 2, mt: 2 }}>
          <Typography variant="subtitle2" sx={{ mb: 2 }}>
            Initial Update Information
          </Typography>
          <Box
            sx={{
              bgcolor: "action.hover",
              p: 2,
              borderRadius: 1,
              border: 1,
              borderColor: "divider",
            }}
          >
            <TextField
              select
              fullWidth
              size="small"
              id="product-update-level"
              label="Update Level"
              value=""
              disabled
              sx={{
                mb: 2,
                "& .MuiSelect-select": {
                  color: "text.secondary",
                },
              }}
            >
              <MenuItem value="">Select</MenuItem>
            </TextField>
            <TextField
              id="product-applied-on"
              label="Applied On"
              type="date"
              fullWidth
              size="small"
              slotProps={{ inputLabel: { shrink: true } }}
              disabled
            />
          </Box>
        </Box>
      </DialogContent>

      <DialogActions
        sx={{ px: 3, pb: 3, pt: 1, justifyContent: "flex-end", gap: 1 }}
      >
        <Button
          variant="outlined"
          onClick={handleClose}
          disabled={isSubmitting}
        >
          Cancel
        </Button>
        {isSubmitting ? (
          <Button
            variant="contained"
            color="primary"
            startIcon={<CircularProgress color="inherit" size={16} />}
            disabled
          >
            Adding...
          </Button>
        ) : (
          <Button
            type="button"
            variant="contained"
            color="primary"
            onClick={handleSubmit}
            disabled={!isValid}
          >
            Add Product
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
