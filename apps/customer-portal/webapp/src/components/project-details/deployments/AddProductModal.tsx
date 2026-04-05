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
  useLayoutEffect,
  useRef,
  useState,
  type ChangeEvent,
  type JSX,
  type UIEvent,
} from "react";
import { SelectMenuLoadMoreRow } from "@components/common/select-menu-load-more-row/SelectMenuLoadMoreRow";
import { paginatedSelectMenuListProps } from "@constants/dropdownConstants";
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
 * Product Name and Version come from paginated APIs; Description and optional metrics are user-entered.
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
  const previousProductIdRef = useRef<string>("");
  const [cachedProductsTotalRecords, setCachedProductsTotalRecords] = useState<
    number | null
  >(null);
  const [cachedVersionsTotalRecords, setCachedVersionsTotalRecords] = useState<
    number | null
  >(null);

  const [productsLoadMorePending, setProductsLoadMorePending] = useState(false);
  const [versionsLoadMorePending, setVersionsLoadMorePending] = useState(false);
  const accumulatedProductsLengthRef = useRef(0);
  const accumulatedVersionsLengthRef = useRef(0);

  useLayoutEffect(() => {
    accumulatedProductsLengthRef.current = products.length;
    accumulatedVersionsLengthRef.current = versions.length;
  }, [products, versions]);

  const resetModalState = useCallback(() => {
    setForm(INITIAL_FORM);
    setProductOffset(0);
    setProducts([]);
    setVersionOffset(0);
    setVersions([]);
    previousProductIdRef.current = "";
    setProductsLoadMorePending(false);
    setVersionsLoadMorePending(false);
    setCachedProductsTotalRecords(null);
    setCachedVersionsTotalRecords(null);
  }, []);

  const {
    data: productsPage,
    isLoading: isLoadingProducts,
    isFetching: isFetchingProducts,
  } = useGetProducts({
    offset: productOffset,
    limit: 10,
  });

  /* eslint-disable react-hooks/set-state-in-effect -- paginated Select: load-more flags + merged list rows */
  useEffect(() => {
    if (!isFetchingProducts) {
      setProductsLoadMorePending(false);
    }
  }, [isFetchingProducts]);

  const {
    data: versionsPage,
    isLoading: isLoadingVersions,
    isFetching: isFetchingVersions,
  } = useSearchProductVersions(form.productId, {
    limit: 10,
    offset: versionOffset,
  });

  useEffect(() => {
    if (!isFetchingVersions) {
      setVersionsLoadMorePending(false);
    }
  }, [isFetchingVersions]);

  useEffect(() => {
    if (!productsPage) return;

    const pageItems = productsPage.products ?? [];
    const offset = productsPage.offset ?? 0;
    const pageLimit =
      typeof productsPage.limit === "number" && productsPage.limit > 0
        ? productsPage.limit
        : 10;

    const applyServerProductsTotal = (): void => {
      if (
        typeof productsPage.totalRecords === "number" &&
        !Number.isNaN(productsPage.totalRecords)
      ) {
        setCachedProductsTotalRecords(productsPage.totalRecords);
      }
    };

    if (offset > 0 && pageItems.length === 0) {
      setCachedProductsTotalRecords(accumulatedProductsLengthRef.current);
      return;
    }

    if (offset === 0) {
      setProducts(pageItems);
      if (pageItems.length < pageLimit) {
        setCachedProductsTotalRecords(pageItems.length);
      } else {
        applyServerProductsTotal();
      }
      return;
    }

    const prevProducts = products;
    const prevProductIds = new Set(
      prevProducts.map((p) => p.id).filter((id): id is string => Boolean(id)),
    );
    const newProductItems = pageItems.filter(
      (p) => typeof p.id === "string" && p.id.length > 0 && !prevProductIds.has(p.id),
    );
    const mergedProductsLen = prevProducts.length + newProductItems.length;

    if (newProductItems.length === 0) {
      setCachedProductsTotalRecords(prevProducts.length);
      return;
    }

    if (pageItems.length < pageLimit) {
      setCachedProductsTotalRecords(mergedProductsLen);
    } else {
      applyServerProductsTotal();
    }
    setProducts([...prevProducts, ...newProductItems]);
  }, [productsPage, products]);

  const productsTotalRecords =
    cachedProductsTotalRecords ??
    productsPage?.totalRecords ??
    products.length;
  const canLoadMoreProducts = products.length < productsTotalRecords;

  const handleProductsScroll = useCallback(
    (event: UIEvent<HTMLElement>) => {
      const target = event.currentTarget;
      if (
        productsLoadMorePending ||
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
        setProductsLoadMorePending(true);
        setProductOffset((prev) => prev + 10);
      }
    },
    [
      productsLoadMorePending,
      canLoadMoreProducts,
      isLoadingProducts,
      isFetchingProducts,
      products.length,
    ],
  );

  useEffect(() => {
    if (previousProductIdRef.current !== form.productId) {
      previousProductIdRef.current = form.productId;
      setVersionOffset(0);
      setVersions([]);
      setCachedVersionsTotalRecords(null);
    }

    if (!form.productId) {
      return;
    }

    if (!versionsPage) {
      return;
    }

    const pageItems = versionsPage.versions ?? [];
    const offset = versionsPage.offset ?? 0;
    const pageLimit =
      typeof versionsPage.limit === "number" && versionsPage.limit > 0
        ? versionsPage.limit
        : 10;

    const applyServerVersionsTotal = (): void => {
      if (
        typeof versionsPage.totalRecords === "number" &&
        !Number.isNaN(versionsPage.totalRecords)
      ) {
        setCachedVersionsTotalRecords(versionsPage.totalRecords);
      }
    };

    if (offset > 0 && pageItems.length === 0) {
      setCachedVersionsTotalRecords(accumulatedVersionsLengthRef.current);
      return;
    }

    if (offset === 0) {
      setVersions(pageItems);
      if (pageItems.length < pageLimit) {
        setCachedVersionsTotalRecords(pageItems.length);
      } else {
        applyServerVersionsTotal();
      }
      return;
    }

    const prevVersions = versions;
    const prevVersionIds = new Set(
      prevVersions.map((v) => v.id).filter((id): id is string => Boolean(id)),
    );
    const newVersionItems = pageItems.filter(
      (v) => typeof v.id === "string" && v.id.length > 0 && !prevVersionIds.has(v.id),
    );
    const mergedVersionsLen = prevVersions.length + newVersionItems.length;

    if (newVersionItems.length === 0) {
      setCachedVersionsTotalRecords(prevVersions.length);
      return;
    }

    if (pageItems.length < pageLimit) {
      setCachedVersionsTotalRecords(mergedVersionsLen);
    } else {
      applyServerVersionsTotal();
    }
    setVersions([...prevVersions, ...newVersionItems]);
  }, [form.productId, versionsPage, versions]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const versionsTotalRecords =
    cachedVersionsTotalRecords ??
    versionsPage?.totalRecords ??
    versions.length;
  const canLoadMoreVersions = versions.length < versionsTotalRecords;

  /**
   * Footer spinner only after the user scrolls to load more (pending flag), not for
   * background refetches or inflated API totalRecords.
   */
  const isFetchingMoreProducts =
    productsLoadMorePending &&
    isFetchingProducts &&
    productOffset > 0 &&
    products.length > 0;
  const isFetchingMoreVersions =
    versionsLoadMorePending &&
    isFetchingVersions &&
    versionOffset > 0 &&
    versions.length > 0;

  const handleVersionsScroll = useCallback(
    (event: UIEvent<HTMLElement>) => {
      const target = event.currentTarget;
      if (
        versionsLoadMorePending ||
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
        setVersionsLoadMorePending(true);
        setVersionOffset((prev) => prev + 10);
      }
    },
    [
      versionsLoadMorePending,
      canLoadMoreVersions,
      isLoadingVersions,
      isFetchingVersions,
      versions.length,
    ],
  );

  const postProduct = usePostDeploymentProduct();

  const isSubmitting = postProduct.isPending;
  const isValid =
    !!form.productId &&
    !!form.versionId &&
    !!projectId &&
    deploymentId.length > 0;

  const handleClose = useCallback(() => {
    resetModalState();
    onClose();
  }, [onClose, resetModalState]);

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
                MenuListProps: paginatedSelectMenuListProps(handleProductsScroll),
                PaperProps: {
                  sx: { zIndex: 1400 },
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
            <SelectMenuLoadMoreRow
              visible={Boolean(
                canLoadMoreProducts && isFetchingMoreProducts,
              )}
            />
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
                MenuListProps: paginatedSelectMenuListProps(handleVersionsScroll),
                PaperProps: {
                  sx: { zIndex: 1400 },
                },
              },
            }}
          >
            <MenuItem value="">Select</MenuItem>
            {versions.map((v) => (
              <MenuItem key={v.id} value={v.id}>
                {v.version}
              </MenuItem>
            ))}
            <SelectMenuLoadMoreRow
              visible={Boolean(
                canLoadMoreVersions && isFetchingMoreVersions,
              )}
            />
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
          value={form.description}
          onChange={handleTextChange("description")}
          disabled={isSubmitting}
        />
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
