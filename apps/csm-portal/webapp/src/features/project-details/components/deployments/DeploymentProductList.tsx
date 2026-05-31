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

import type { DeploymentProductItem } from "@features/project-details/types/deployments";
import {
  displayValue,
  formatProjectDate,
} from "@features/project-details/utils/projectDetails";
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  Skeleton,
  Typography,
  alpha,
} from "@wso2/oxygen-ui";
import {
  Calendar,
  Check,
  CircleAlert,
  Cpu,
  Package,
  PencilLine,
  Plus,
  Trash2,
  Zap,
} from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { usePostDeploymentProductsSearchAll } from "@features/project-details/api/usePostDeploymentProductsSearch";
import { usePatchDeploymentProduct } from "@features/project-details/api/usePatchDeploymentProduct";
import { ApiQueryKeys } from "@constants/apiConstants";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import ErrorBanner from "@components/error-banner/ErrorBanner";
import AddProductModal from "@features/project-details/components/deployments/AddProductModal";
import ManageProductModal from "@features/project-details/components/deployments/ManageProductModal";
import DeleteProductModal from "@features/project-details/components/deployments/DeleteProductModal";
import type {
  DeploymentProductItemRowProps,
  DeploymentProductListProps,
} from "@features/project-details/types/projectDetailsComponents";

function ProductsSkeleton(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {[1, 2].map((i) => (
        <Box
          key={i}
          sx={{
            p: 2,
            bgcolor: (theme) => alpha(theme.palette.grey[500], 0.05),
            borderRadius: "8px",
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "flex-start",
              justifyContent: "space-between",
            }}
          >
            <Box
              sx={{
                display: "flex",
                alignItems: "flex-start",
                gap: 1.5,
                flex: 1,
                minWidth: 0,
              }}
            >
              <Skeleton
                variant="rounded"
                width={16}
                height={16}
                sx={{ mt: 0.25, flexShrink: 0 }}
              />
              <Box sx={{ flex: 1, minWidth: 0 }}>
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 1,
                    mb: 1,
                    flexWrap: "wrap",
                  }}
                >
                  <Skeleton variant="text" width="42%" height={22} />
                </Box>
                <Skeleton
                  variant="text"
                  width="88%"
                  height={14}
                  sx={{ mb: 1.5 }}
                />
                <Box
                  sx={{
                    display: "grid",
                    gridTemplateColumns: "1fr 1fr",
                    columnGap: 1,
                    rowGap: 1.5,
                    mb: 1.5,
                  }}
                >
                  <Skeleton variant="text" width="72%" height={14} />
                  <Skeleton variant="text" width="68%" height={14} />
                  <Skeleton variant="text" width="78%" height={14} />
                  <Skeleton variant="text" width="64%" height={14} />
                </Box>
                <Skeleton variant="rounded" width={132} height={20} />
              </Box>
            </Box>
            <Box sx={{ display: "flex", gap: 0.25, alignItems: "center" }}>
              <Skeleton variant="rounded" width={32} height={32} />
              <Skeleton variant="rounded" width={32} height={32} />
            </Box>
          </Box>
        </Box>
      ))}
    </Box>
  );
}

/**
 * Renders the list of products for a deployment.
 *
 * @param {DeploymentProductListProps} props - Props containing deploymentId.
 * @returns {JSX.Element} The product list component.
 */
export default function DeploymentProductList({
  deploymentId,
  projectId,
  selectedProduct,
  onToggleProductSelect,
}: DeploymentProductListProps): JSX.Element {
  const queryClient = useQueryClient();
  const [isAddProductModalOpen, setIsAddProductModalOpen] = useState(false);
  const [addProductModalKey, setAddProductModalKey] = useState(0);
  const [editingProduct, setEditingProduct] =
    useState<DeploymentProductItem | null>(null);
  const [deletingProductId, setDeletingProductId] = useState<string | null>(
    null,
  );
  const [deleteModalOpen, setDeleteModalOpen] = useState(false);
  const [productToDelete, setProductToDelete] =
    useState<DeploymentProductItem | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const productsQuery = usePostDeploymentProductsSearchAll(deploymentId, {
    enabled: !!deploymentId,
  });
  const products = productsQuery.data ?? [];
  const manageModalProduct = useMemo((): DeploymentProductItem | null => {
    if (!editingProduct) return null;
    return products.find((p) => p.id === editingProduct.id) ?? editingProduct;
  }, [products, editingProduct]);
  const isLoading = productsQuery.isLoading;
  const isError = productsQuery.isError;
  const patchProduct = usePatchDeploymentProduct();

  const handleDeleteClick = (item: DeploymentProductItem) => {
    setProductToDelete(item);
    setDeleteModalOpen(true);
  };

  const handleConfirmDelete = async () => {
    if (!productToDelete) return;
    setDeletingProductId(productToDelete.id);
    setDeleteError(null);
    try {
      await patchProduct.mutateAsync({
        deploymentId,
        productId: productToDelete.id,
        body: { active: false },
      });
      setDeleteModalOpen(false);
      setProductToDelete(null);
    } catch (error) {
      console.error("Failed to delete product:", error);
      const productName = productToDelete.product?.label || "product";
      setDeleteError(`Failed to delete ${productName}. Please try again.`);
    } finally {
      setDeletingProductId(null);
    }
  };

  return (
    <>
      {deleteError && (
        <ErrorBanner
          message={deleteError}
          onClose={() => setDeleteError(null)}
        />
      )}
      <Box>
        <Box sx={{ display: "flex", justifyContent: "space-between", mb: 2 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Package size={16} />
            <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
              WSO2 Products
            </Typography>
            {isLoading ? (
              <Skeleton
                variant="rounded"
                width={32}
                height={20}
                sx={{ flexShrink: 0 }}
              />
            ) : (
              <Typography variant="body2" color="text.secondary">
                ({products.length})
              </Typography>
            )}
          </Box>
          <Button
            variant="outlined"
            size="small"
            startIcon={<Plus />}
            sx={{ height: 32, fontSize: "0.75rem" }}
            onClick={() => {
              setAddProductModalKey((k) => k + 1);
              setIsAddProductModalOpen(true);
            }}
          >
            Add Product
          </Button>
        </Box>
        {isLoading ? (
          <ProductsSkeleton />
        ) : isError ? (
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, py: 2 }}>
            <ErrorIndicator entityName="products" size="small" />
            <Typography variant="body2" color="text.secondary">
              Failed to load products
            </Typography>
          </Box>
        ) : products.length === 0 ? (
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ py: 2, textAlign: "center" }}
          >
            No products added yet
          </Typography>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            {products.map((item) => (
              <ProductItemRow
                key={item.id}
                item={item}
                deploymentId={deploymentId}
                isSelected={
                  selectedProduct?.deploymentId === deploymentId &&
                  selectedProduct.productItemId === item.id
                }
                onToggleSelect={() =>
                  onToggleProductSelect(deploymentId, item.id)
                }
                onEdit={() => setEditingProduct(item)}
                onDelete={() => handleDeleteClick(item)}
                isDeleting={deletingProductId === item.id}
              />
            ))}
          </Box>
        )}

        <AddProductModal
          key={addProductModalKey}
          open={isAddProductModalOpen}
          deploymentId={deploymentId}
          projectId={projectId}
          onClose={() => setIsAddProductModalOpen(false)}
          onSuccess={() => {
            setIsAddProductModalOpen(false);
            queryClient.invalidateQueries({
              queryKey: [ApiQueryKeys.DEPLOYMENT_PRODUCTS, deploymentId],
            });
          }}
        />
        <ManageProductModal
          open={!!editingProduct}
          deploymentId={deploymentId}
          product={manageModalProduct}
          onClose={() => setEditingProduct(null)}
          onSuccess={() => {
            queryClient.invalidateQueries({
              queryKey: [ApiQueryKeys.DEPLOYMENT_PRODUCTS, deploymentId],
            });
          }}
        />
        <DeleteProductModal
          open={deleteModalOpen}
          product={productToDelete}
          onClose={() => {
            setDeleteModalOpen(false);
            setProductToDelete(null);
          }}
          onConfirm={handleConfirmDelete}
          isDeleting={
            !!deletingProductId && deletingProductId === productToDelete?.id
          }
        />
      </Box>
    </>
  );
}

function ProductItemRow({
  item,
  isSelected,
  onToggleSelect,
  onEdit,
  onDelete,
  isDeleting,
}: DeploymentProductItemRowProps): JSX.Element {
  const emptyVal = "Not Available";
  const name = displayValue(item.product?.label, emptyVal);
  const description = displayValue(item.description, emptyVal);
  const coresStr =
    typeof item.cores === "number"
      ? String(item.cores)
      : displayValue(null, emptyVal);
  const tpsStr =
    typeof item.tps === "number"
      ? item.tps.toLocaleString()
      : displayValue(null, emptyVal);
  const versionRef =
    typeof item.version === "object" && item.version !== null
      ? item.version
      : null;
  const versionLabel =
    item.version == null
      ? null
      : typeof item.version === "string"
        ? item.version || null
        : (item.version as { label?: string })?.label || null;
  const releasedStr = versionRef?.releasedOn
    ? formatProjectDate(versionRef.releasedOn)
    : displayValue(null, emptyVal);
  const eolStr = versionRef?.endOfLifeOn
    ? formatProjectDate(versionRef.endOfLifeOn)
    : displayValue(null, emptyVal);
  const updateLevelStr = (() => {
    const levels =
      item.updates
        ?.map((u) => u.updateLevel)
        .filter((l): l is number => typeof l === "number") ?? [];
    if (levels.length === 0) return emptyVal;
    return String(Math.max(...levels));
  })();

  return (
    <Box
      sx={{
        p: 2,
        bgcolor: (theme) => alpha(theme.palette.grey[500], 0.05),
        borderRadius: "8px",
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
        }}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "flex-start",
            gap: 1.5,
            flex: 1,
            minWidth: 0,
          }}
        >
          <Box
            component="button"
            type="button"
            role="checkbox"
            aria-checked={isSelected}
            aria-label={`Select ${name} for service request`}
            onClick={(e) => {
              e.preventDefault();
              onToggleSelect();
            }}
            sx={{
              width: 16,
              height: 16,
              flexShrink: 0,
              mt: 0.25,
              border: "1px solid",
              borderColor: "divider",
              bgcolor: isSelected ? "primary.main" : "background.paper",
              color: isSelected ? "primary.contrastText" : "transparent",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              cursor: "pointer",
              p: 0,
            }}
          >
            {isSelected ? (
              <Check size={12} strokeWidth={3} aria-hidden />
            ) : null}
          </Box>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1,
                mb: 1,
                flexWrap: "wrap",
              }}
            >
              <Typography
                variant="body2"
                sx={{ fontWeight: 600, color: "text.primary" }}
              >
                {name}
              </Typography>
              {versionLabel && (
                <Typography
                  variant="body2"
                  sx={{ fontWeight: 600, color: "text.primary" }}
                >
                  {versionLabel}
                </Typography>
              )}
            </Box>
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ display: "block", mb: 1.5 }}
            >
              {description}
            </Typography>
            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: "1fr 1fr",
                columnGap: 1,
                rowGap: 1.5,
                mb: 1.5,
              }}
            >
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 0.5,
                  flexShrink: 0,
                }}
              >
                <Cpu size={12} style={{ display: "block", flexShrink: 0 }} />
                <Typography
                  variant="caption"
                  sx={{ whiteSpace: "nowrap", lineHeight: 1 }}
                >
                  Cores: {coresStr}
                </Typography>
              </Box>
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 0.5,
                  color: "text.secondary",
                  flexShrink: 0,
                }}
              >
                <Zap size={12} style={{ display: "block", flexShrink: 0 }} />
                <Typography
                  variant="caption"
                  sx={{ whiteSpace: "nowrap", lineHeight: 1 }}
                >
                  TPS: {tpsStr}
                </Typography>
              </Box>
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 0.5,
                  color: "text.secondary",
                  flexShrink: 0,
                }}
              >
                <Calendar
                  size={12}
                  style={{ display: "block", flexShrink: 0 }}
                />
                <Typography
                  variant="caption"
                  sx={{ whiteSpace: "nowrap", lineHeight: 1 }}
                >
                  Released: {releasedStr}
                </Typography>
              </Box>
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 0.5,
                  color: "text.secondary",
                  flexShrink: 0,
                }}
              >
                <CircleAlert
                  size={12}
                  style={{ display: "block", flexShrink: 0 }}
                />
                <Typography
                  variant="caption"
                  sx={{ whiteSpace: "nowrap", lineHeight: 1 }}
                >
                  EOL: {eolStr}
                </Typography>
              </Box>
            </Box>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1,
                mt: 1.5,
              }}
            >
              <Chip
                label={`Update Level: ${updateLevelStr}`}
                size="small"
                variant="outlined"
                sx={{
                  height: 20,
                  fontSize: "0.75rem",
                  color: "text.primary",
                }}
              />
            </Box>
          </Box>
        </Box>
        <Box sx={{ display: "flex", gap: 0.25, alignItems: "center" }}>
          <IconButton
            size="small"
            aria-label={`Edit ${name}`}
            onClick={onEdit}
            sx={{
              color: "text.secondary",
              "&:hover": { color: "primary.main" },
              "&.Mui-focusVisible": { color: "primary.main" },
            }}
          >
            <PencilLine size={16} aria-hidden />
          </IconButton>
          <IconButton
            size="small"
            aria-label={`Delete ${name}`}
            onClick={() => onDelete(item)}
            disabled={isDeleting}
            sx={{
              color: "text.secondary",
              "&:hover": { color: "error.main" },
              "&.Mui-focusVisible": { color: "error.main" },
            }}
          >
            {isDeleting ? (
              <CircularProgress size={16} color="inherit" />
            ) : (
              <Trash2 size={16} aria-hidden />
            )}
          </IconButton>
        </Box>
      </Box>
    </Box>
  );
}
