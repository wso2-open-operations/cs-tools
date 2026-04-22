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

import type { BasicInformationSectionProps } from "@features/support/types/supportComponents";
import {
  Box,
  Chip,
  FormControl,
  Grid,
  MenuItem,
  Paper,
  Select,
  Skeleton,
  Typography,
  TextField,
} from "@wso2/oxygen-ui";
import { Sparkles } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type UIEvent } from "react";
import { SelectMenuLoadMoreRow } from "@components/select-menu-load-more-row/SelectMenuLoadMoreRow";
import { EMPTY_DROPDOWN_PLACEHOLDER } from "@constants/common";
import { paginatedSelectMenuListProps } from "@utils/common";
import { isCloudSupportProject } from "@utils/permission";

/**
 * Renders the Basic Information section used during case creation.
 *
 * @returns {JSX.Element} The Basic Information section.
 */
export function BasicInformationSection({
  project = "",
  product = "",
  setProduct = () => undefined,
  deployment = "",
  setDeployment = () => undefined,
  productOptionList,
  isProductAutoDetected = true,
  isDeploymentAutoDetected = true,
  metadata,
  isDeploymentLoading = false,
  isProductDropdownDisabled = false,
  isProductLoading = false,
  extraDeploymentOptions,
  extraProductOptions,
  isRelatedCaseMode = false,
  isDeploymentDisabled = false,
  hideDeploymentField = false,
  onLoadMoreDeployments,
  hasMoreDeployments = false,
  isFetchingMoreDeployments = false,
  onLoadMoreProducts,
  hasMoreProducts = false,
  isFetchingMoreProducts = false,
  projectTypeLabel,
}: BasicInformationSectionProps & { projectTypeLabel?: string | null }): JSX.Element {
  const handleDeploymentsMenuScroll = (e: UIEvent<HTMLElement>) => {
    if (
      !onLoadMoreDeployments ||
      !hasMoreDeployments ||
      isFetchingMoreDeployments
    ) {
      return;
    }
    const el = e.currentTarget;
    const threshold = 24;
    const isNearBottom =
      el.scrollHeight - el.scrollTop - el.clientHeight < threshold;
    if (isNearBottom) {
      onLoadMoreDeployments();
    }
  };

  const handleProductsMenuScroll = (e: UIEvent<HTMLElement>) => {
    if (!onLoadMoreProducts || !hasMoreProducts || isFetchingMoreProducts) {
      return;
    }
    const el = e.currentTarget;
    const threshold = 24;
    const isNearBottom =
      el.scrollHeight - el.scrollTop - el.clientHeight < threshold;
    if (isNearBottom) {
      onLoadMoreProducts();
    }
  };
  const deploymentOptions = Array.from(
    new Set(
      [
        ...(metadata?.deploymentTypes ?? []),
        ...(extraDeploymentOptions ?? []),
      ].filter((value) => value && value.trim() !== ""),
    ),
  );
  const productOptionsLegacy = Array.from(
    new Set(
      [...(metadata?.products ?? []), ...(extraProductOptions ?? [])].filter(
        (value) => value && value.trim() !== "",
      ),
    ),
  );
  const useProductOptionList = Array.isArray(productOptionList);
  const hasProductRows = (productOptionList?.length ?? 0) > 0;
  const showNoProductsHint =
    useProductOptionList &&
    !hasProductRows &&
    !isProductDropdownDisabled &&
    !isProductLoading;

  const showNoDeploymentsHint =
    !isDeploymentLoading &&
    deploymentOptions.length === 0 &&
    !isDeploymentDisabled;

  const deploymentMenuListProps = paginatedSelectMenuListProps(
    onLoadMoreDeployments && hasMoreDeployments
      ? handleDeploymentsMenuScroll
      : undefined,
  );
  const productMenuListProps = paginatedSelectMenuListProps(
    onLoadMoreProducts && hasMoreProducts
      ? handleProductsMenuScroll
      : undefined,
  );

  return (
    <Paper sx={{ p: 3 }}>
      {/* section header container */}
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          mb: 3,
        }}
      >
        <Typography variant="h6">Basic Information</Typography>
      </Box>

      {/* project card grid layout */}
      <Grid container spacing={3}>
        {/* project selection field wrapper */}
        <Grid size={{ xs: 12 }}>
          {/* project field label container */}
          <Box sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}>
            <Typography variant="caption">Project</Typography>
          </Box>
          <TextField fullWidth size="small" disabled value={project} />
        </Grid>

        {/* deployment selection field wrapper */}
        {!hideDeploymentField && (
          <Grid size={{ xs: 12 }}>
            {/* deployment field label container */}
            <Box sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}>
              <Typography variant="caption">
                Deployment Type{" "}
                {!isDeploymentDisabled && (
                  <Box component="span" sx={{ color: "warning.main" }}>
                    *
                  </Box>
                )}
              </Typography>
              {!isRelatedCaseMode && isDeploymentAutoDetected && (
                <Chip
                  label="Auto detected"
                  size="small"
                  variant="outlined"
                  icon={<Sparkles size={10} />}
                  sx={{ height: 20, fontSize: "0.65rem", p: 0.5 }}
                />
              )}
            </Box>
            {isDeploymentLoading ? (
              <Skeleton
                variant="rounded"
                height={40}
                sx={{ maxWidth: "100%" }}
              />
            ) : (
              <FormControl
                fullWidth
                size="small"
                disabled={isDeploymentDisabled}
              >
                <Select
                  value={deployment}
                  onChange={(e) => setDeployment(e.target.value)}
                  displayEmpty
                  renderValue={(value) => {
                    if (value === "") {
                      if (showNoDeploymentsHint) {
                        return EMPTY_DROPDOWN_PLACEHOLDER;
                      }
                      return "Select Deployment Type...";
                    }
                    return value;
                  }}
                  MenuProps={{
                    MenuListProps: deploymentMenuListProps,
                  }}
                >
                  <MenuItem value="" disabled>
                    {showNoDeploymentsHint
                      ? EMPTY_DROPDOWN_PLACEHOLDER
                      : "Select Deployment Type..."}
                  </MenuItem>
                  {deploymentOptions.map((d) => (
                    <MenuItem key={d} value={d}>
                      {d}
                    </MenuItem>
                  ))}
                  <SelectMenuLoadMoreRow
                    visible={Boolean(
                      onLoadMoreDeployments &&
                      hasMoreDeployments &&
                      isFetchingMoreDeployments &&
                      deploymentOptions.length > 0,
                    )}
                  />
                </Select>
              </FormControl>
            )}
          </Grid>
        )}

        {/* product selection field wrapper */}
        <Grid size={{ xs: 12 }}>
          {/* product field label container */}
          <Box sx={{ mb: 1, display: "flex", alignItems: "center", gap: 1 }}>
            <Typography variant="caption">
              {isCloudSupportProject(projectTypeLabel) ? "Product" : "Product Version"}{" "}
              <Box component="span" sx={{ color: "warning.main" }}>
                *
              </Box>
            </Typography>
            {!isRelatedCaseMode && isProductAutoDetected && (
              <Chip
                label="Auto detected"
                size="small"
                variant="outlined"
                icon={<Sparkles size={10} />}
                sx={{ height: 20, fontSize: "0.65rem", p: 0.5 }}
              />
            )}
          </Box>
          {isProductLoading ? (
            <Skeleton variant="rounded" height={40} sx={{ maxWidth: "100%" }} />
          ) : (
            <FormControl
              fullWidth
              size="small"
              disabled={isProductDropdownDisabled}
            >
              <Select
                value={product}
                onChange={(e) => setProduct(e.target.value)}
                displayEmpty
                renderValue={(value) => {
                  if (value === "") {
                    if (isProductDropdownDisabled) {
                      return "Select deployment first";
                    }
                    if (showNoProductsHint) {
                      return EMPTY_DROPDOWN_PLACEHOLDER;
                    }
                    const productLabel = isCloudSupportProject(projectTypeLabel) ? "Product" : "Product Version";
                    return `Select ${productLabel}...`;
                  }
                  if (useProductOptionList && hasProductRows) {
                    const opt = productOptionList!.find((o) => o.id === value);
                    return opt?.label ?? value;
                  }
                  if (showNoProductsHint) {
                    return EMPTY_DROPDOWN_PLACEHOLDER;
                  }
                  return value;
                }}
                MenuProps={{
                  MenuListProps: productMenuListProps,
                }}
              >
                <MenuItem value="" disabled>
                  {isProductDropdownDisabled
                    ? "Select deployment first"
                    : showNoProductsHint
                      ? EMPTY_DROPDOWN_PLACEHOLDER
                      : `Select ${isCloudSupportProject(projectTypeLabel) ? "Product" : "Product Version"}...`}
                </MenuItem>
                {useProductOptionList
                  ? productOptionList!.map((p) => (
                      <MenuItem key={p.id} value={p.id}>
                        {p.label}
                      </MenuItem>
                    ))
                  : productOptionsLegacy.map((p) => (
                      <MenuItem key={p} value={p}>
                        {p}
                      </MenuItem>
                    ))}
                <SelectMenuLoadMoreRow
                  visible={Boolean(
                    onLoadMoreProducts &&
                    hasMoreProducts &&
                    isFetchingMoreProducts &&
                    (useProductOptionList
                      ? hasProductRows
                      : productOptionsLegacy.length > 0),
                  )}
                />
              </Select>
            </FormControl>
          )}
        </Grid>
      </Grid>
    </Paper>
  );
}
