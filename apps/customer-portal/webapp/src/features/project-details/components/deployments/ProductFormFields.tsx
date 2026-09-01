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

import { Box, MenuItem, Skeleton, TextField } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { SelectMenuLoadMoreRow } from "@components/select-menu-load-more-row/SelectMenuLoadMoreRow";
import { paginatedSelectMenuListProps } from "@utils/common";
import type { UseProductFormResult } from "@features/project-details/components/deployments/useProductForm";

export type ProductFormFieldsProps = Pick<
  UseProductFormResult,
  | "form"
  | "products"
  | "versions"
  | "isLoadingProducts"
  | "isFetchingProducts"
  | "isLoadingVersions"
  | "isFetchingVersions"
  | "canLoadMoreProducts"
  | "canLoadMoreVersions"
  | "isFetchingMoreProducts"
  | "isFetchingMoreVersions"
  | "handleProductsScroll"
  | "handleVersionsScroll"
  | "handleProductChange"
  | "handleVersionChange"
  | "handleTextChange"
> & {
  isSubmitting: boolean;
};

/**
 * Pure presentational product/version + metrics form fields, shared by the
 * standalone AddProductModal and the add-deployment wizard's product step.
 * All state and data-fetching live in useProductForm; this component only
 * renders and wires up the handlers it is given.
 *
 * @param {ProductFormFieldsProps} props - Form state, paginated options, and change handlers from useProductForm.
 * @returns {JSX.Element} The product form fields.
 */
export function ProductFormFields({
  form,
  products,
  versions,
  isLoadingProducts,
  isFetchingProducts,
  isLoadingVersions,
  isFetchingVersions,
  canLoadMoreProducts,
  canLoadMoreVersions,
  isFetchingMoreProducts,
  isFetchingMoreVersions,
  handleProductsScroll,
  handleVersionsScroll,
  handleProductChange,
  handleVersionChange,
  handleTextChange,
  isSubmitting,
}: ProductFormFieldsProps): JSX.Element {
  return (
    <>
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
            visible={Boolean(canLoadMoreProducts && isFetchingMoreProducts)}
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
            visible={Boolean(canLoadMoreVersions && isFetchingMoreVersions)}
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
    </>
  );
}
