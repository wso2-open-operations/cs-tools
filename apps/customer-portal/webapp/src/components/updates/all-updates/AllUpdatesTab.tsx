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
  Card,
  CardContent,
  FormControl,
  Grid,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { FileText } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useEffect, useMemo, useState, type JSX } from "react";
import { useNavigate, useParams } from "react-router";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import { useGetProductUpdateLevels } from "@api/useGetProductUpdateLevels";
import { usePostUpdateLevelsSearch } from "@api/usePostUpdateLevelsSearch";
import { PendingUpdatesList } from "@components/updates/pending-updates/PendingUpdatesList";
import PendingUpdatesListSkeleton from "@components/updates/pending-updates/PendingUpdatesListSkeleton";
import EmptyState from "@components/common/empty-state/EmptyState";
import Error500Page from "@components/common/error/Error500Page";
import UpdateLevelsReportModal from "@components/updates/all-updates/UpdateLevelsReportModal";
import type { ProductUpdateLevelEntry, ProductUpdateLevelsItem } from "@/types/updates";
import { EMPTY_DROPDOWN_PLACEHOLDER } from "@constants/dropdownConstants";
import { getUpdateLevelsReportData } from "@utils/updateLevelsReportPdf";

export interface AllUpdatesTabFilterState {
  productName: string;
  productVersion: string;
  startLevel: string;
  endLevel: string;
}

const INITIAL_FILTER: AllUpdatesTabFilterState = {
  productName: "",
  productVersion: "",
  startLevel: "",
  endLevel: "",
};

function isValidFilter(
  f: AllUpdatesTabFilterState,
): { valid: true; start: number; end: number } | { valid: false } {
  const start = Number(f.startLevel);
  const end = Number(f.endLevel);
  if (
    !f.productName ||
    !f.productVersion ||
    f.startLevel === "" ||
    f.endLevel === "" ||
    !Number.isFinite(start) ||
    !Number.isFinite(end) ||
    start < 0 ||
    end < 0 ||
    start > end
  ) {
    return { valid: false };
  }
  return { valid: true, start, end };
}

/**
 * Derives unique product names from product update levels (GET /updates/product-update-levels).
 *
 * @param {ProductUpdateLevelsItem[] | undefined} data - Product update levels response.
 * @returns {string[]} Sorted unique product names.
 */
function getProductNamesFromProductLevels(
  data: ProductUpdateLevelsItem[] | undefined,
): string[] {
  if (!data?.length) return [];
  const names = [...new Set(data.map((d) => d.productName))];
  return names.sort();
}

/**
 * Derives version entries for a selected product.
 *
 * @param {ProductUpdateLevelsItem[] | undefined} data - Product update levels response.
 * @param {string} productName - Selected product.
 * @returns {ProductUpdateLevelEntry[]} Matching version entries.
 */
function getVersionEntriesForProduct(
  data: ProductUpdateLevelsItem[] | undefined,
  productName: string,
): ProductUpdateLevelEntry[] {
  if (!data?.length || !productName) return [];
  const item = data.find((d) => d.productName === productName);
  return item?.productUpdateLevels ?? [];
}

/**
 * Returns sorted update levels for the selected product and version.
 *
 * @param {ProductUpdateLevelsItem[] | undefined} data - Product update levels response.
 * @param {string} productName - Selected product.
 * @param {string} productVersion - Selected version (productBaseVersion).
 * @returns {number[]} Sorted update levels.
 */
function getUpdateLevelsForProductVersion(
  data: ProductUpdateLevelsItem[] | undefined,
  productName: string,
  productVersion: string,
): number[] {
  const entries = getVersionEntriesForProduct(data, productName);
  const entry = entries.find((e) => e.productBaseVersion === productVersion);
  if (!entry?.updateLevels?.length) return [];
  return [...entry.updateLevels].sort((a, b) => a - b);
}

/**
 * AllUpdatesTab displays a filter section and search results for update levels.
 * Uses GET /updates/product-update-levels for filter options and
 * POST /updates/levels/search for results.
 *
 * @returns {JSX.Element} The rendered All Updates tab content.
 */
export default function AllUpdatesTab(): JSX.Element {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();

  const [filter, setFilter] = useState<AllUpdatesTabFilterState>(INITIAL_FILTER);
  const [reportModalOpen, setReportModalOpen] = useState(false);
  const [searchParams, setSearchParams] = useState<{
    productName: string;
    productVersion: string;
    startingUpdateLevel: number;
    endingUpdateLevel: number;
  } | null>(null);

  const {
    data: productLevelsData,
    isLoading: isProductLevelsLoading,
    isError: isProductLevelsError,
  } = useGetProductUpdateLevels();

  const { data: searchData, isLoading: isSearchLoading, isError: isSearchError } =
    usePostUpdateLevelsSearch(searchParams);

  const productNames = useMemo(
    () => getProductNamesFromProductLevels(productLevelsData),
    [productLevelsData],
  );

  const versionEntries = useMemo(
    () => getVersionEntriesForProduct(productLevelsData, filter.productName),
    [productLevelsData, filter.productName],
  );

  const startLevelOptions = useMemo(
    () =>
      getUpdateLevelsForProductVersion(
        productLevelsData,
        filter.productName,
        filter.productVersion,
      ),
    [productLevelsData, filter.productName, filter.productVersion],
  );

  const endLevelOptions = useMemo(() => {
    if (!filter.startLevel || startLevelOptions.length === 0) return [];
    const start = Number(filter.startLevel);
    if (!Number.isFinite(start)) return [];
    return startLevelOptions.filter((level) => level >= start);
  }, [startLevelOptions, filter.startLevel]);

  useEffect(() => {
    if (!filter.endLevel || endLevelOptions.length === 0) return;
    const endNum = Number(filter.endLevel);
    const valid = endLevelOptions.includes(endNum);
    if (!valid) {
      setFilter((prev) => ({ ...prev, endLevel: "" }));
    }
  }, [endLevelOptions, filter.endLevel]);

  const handleFilterChange = useCallback(
    (field: keyof AllUpdatesTabFilterState) => (e: SelectChangeEvent<string>) => {
      const value = e.target.value;
      setFilter((prev) => {
        const next = { ...prev, [field]: value };
        if (field === "productName") {
          next.productVersion = "";
          next.startLevel = "";
          next.endLevel = "";
        } else if (field === "productVersion") {
          next.startLevel = "";
          next.endLevel = "";
        } else if (field === "startLevel") {
          next.endLevel = "";
        }
        return next;
      });
    },
    [],
  );

  const handleSearch = useCallback(() => {
    const result = isValidFilter(filter);
    if (!result.valid) return;
    setSearchParams({
      productName: filter.productName,
      productVersion: filter.productVersion,
      startingUpdateLevel: result.start,
      endingUpdateLevel: result.end,
    });
  }, [filter]);

  const handleView = useCallback(
    (levelKey: string) => {
      if (!searchParams || !projectId) return;
      const params = new URLSearchParams({
        productName: searchParams.productName,
        productBaseVersion: searchParams.productVersion,
        startingUpdateLevel: String(searchParams.startingUpdateLevel),
        endingUpdateLevel: String(searchParams.endingUpdateLevel),
      });
      navigate(`/projects/${projectId}/updates/pending/level/${levelKey}?${params}`);
    },
    [navigate, projectId, searchParams],
  );

  const reportData = useMemo(() => {
    if (!searchData || !searchParams || Object.keys(searchData).length === 0) return null;
    try {
      return getUpdateLevelsReportData({
        productName: searchParams.productName,
        productVersion: searchParams.productVersion,
        startLevel: searchParams.startingUpdateLevel,
        endLevel: searchParams.endingUpdateLevel,
        data: searchData,
      });
    } catch {
      return null;
    }
  }, [searchData, searchParams]);

  const handleViewReport = useCallback(() => {
    if (!reportData) return;
    setReportModalOpen(true);
  }, [reportData]);

  const canSearch = isValidFilter(filter).valid;

  const canViewReport = !!reportData;

  if (isProductLevelsError) {
    return (
      <Box
        sx={{
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
          flexDirection: "column",
          py: 5,
        }}
      >
        <Error500Page style={{ width: 200, height: "auto" }} />
        <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
          Could not load filter options. Please try again later.
        </Typography>
      </Box>
    );
  }

  return (
    <Stack spacing={3} sx={{ width: "100%" }}>
      <Card variant="outlined" sx={{ borderRadius: 0 }}>
        <CardContent sx={{ p: 3 }}>
          <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 2 }}>
            Search Update Levels
          </Typography>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <FormControl fullWidth size="small" disabled={isProductLevelsLoading}>
                <InputLabel id="all-updates-product-label">Product Name *</InputLabel>
                <Select
                  labelId="all-updates-product-label"
                  id="all-updates-product"
                  value={filter.productName}
                  label="Product Name *"
                  onChange={handleFilterChange("productName")}
                >
                  <MenuItem value="" disabled>
                    <Typography variant="body2">
                      {isProductLevelsLoading
                        ? "Select Product"
                        : productNames.length === 0
                          ? EMPTY_DROPDOWN_PLACEHOLDER
                          : "Select Product"}
                    </Typography>
                  </MenuItem>
                  {productNames.map((name) => (
                    <MenuItem key={name} value={name}>
                      <Typography variant="body2">{name}</Typography>
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <FormControl fullWidth size="small" disabled={isProductLevelsLoading || !filter.productName}>
                <InputLabel id="all-updates-version-label">Product Version *</InputLabel>
                <Select
                  labelId="all-updates-version-label"
                  id="all-updates-version"
                  value={filter.productVersion}
                  label="Product Version *"
                  onChange={handleFilterChange("productVersion")}
                >
                  <MenuItem value="" disabled>
                    <Typography variant="body2">
                      {!filter.productName
                        ? "Select Version"
                        : isProductLevelsLoading
                          ? "Select Version"
                          : versionEntries.length === 0
                            ? EMPTY_DROPDOWN_PLACEHOLDER
                            : "Select Version"}
                    </Typography>
                  </MenuItem>
                  {versionEntries.map((v) => (
                    <MenuItem key={v.productBaseVersion} value={v.productBaseVersion}>
                      <Typography variant="body2">{v.productBaseVersion}</Typography>
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <FormControl fullWidth size="small" disabled={!filter.productVersion}>
                <InputLabel id="all-updates-start-label">Starting Update Level *</InputLabel>
                <Select
                  labelId="all-updates-start-label"
                  id="all-updates-start"
                  value={filter.startLevel}
                  label="Starting Update Level *"
                  onChange={handleFilterChange("startLevel")}
                >
                  <MenuItem value="" disabled>
                    <Typography variant="body2">
                      {!filter.productVersion
                        ? "Select Level"
                        : startLevelOptions.length === 0
                          ? EMPTY_DROPDOWN_PLACEHOLDER
                          : "Select Level"}
                    </Typography>
                  </MenuItem>
                  {startLevelOptions.map((level) => (
                    <MenuItem key={level} value={String(level)}>
                      <Typography variant="body2">{level}</Typography>
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <FormControl fullWidth size="small" disabled={!filter.startLevel}>
                <InputLabel id="all-updates-end-label">Ending Update Level *</InputLabel>
                <Select
                  labelId="all-updates-end-label"
                  id="all-updates-end"
                  value={filter.endLevel}
                  label="Ending Update Level *"
                  onChange={handleFilterChange("endLevel")}
                >
                  <MenuItem value="" disabled>
                    <Typography variant="body2">
                      {!filter.startLevel
                        ? "Select Level"
                        : endLevelOptions.length === 0
                          ? EMPTY_DROPDOWN_PLACEHOLDER
                          : "Select Level"}
                    </Typography>
                  </MenuItem>
                  {endLevelOptions.map((level) => (
                    <MenuItem key={level} value={String(level)}>
                      <Typography variant="body2">{level}</Typography>
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
          </Grid>
          <Stack direction="row" spacing={2} sx={{ mt: 3 }}>
            <Button
              variant="contained"
              color="warning"
              onClick={handleSearch}
              disabled={!canSearch || isSearchLoading}
            >
              Search
            </Button>
            <Button
              variant="outlined"
              color="warning"
              startIcon={<FileText size={18} />}
              onClick={handleViewReport}
              disabled={!canViewReport}
            >
              View Report
            </Button>
          </Stack>
        </CardContent>
      </Card>

      {!searchParams ? (
        <Paper variant="outlined" sx={{ p: 4, textAlign: "center" }}>
          <Typography variant="body2" color="text.secondary">
            Select product, version, and update level range, then click Search to view updates.
          </Typography>
        </Paper>
      ) : isSearchLoading ? (
        <PendingUpdatesListSkeleton />
      ) : isSearchError ? (
        <Box
          sx={{
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
            flexDirection: "column",
            py: 5,
          }}
        >
          <Error500Page style={{ width: 200, height: "auto" }} />
          <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
            Failed to load update levels. Please try again.
          </Typography>
        </Box>
      ) : searchData && Object.keys(searchData).length === 0 ? (
        <EmptyState description="No update levels found for the selected criteria." />
      ) : (
        <PendingUpdatesList
          data={searchData ?? null}
          isError={false}
          onView={handleView}
        />
      )}

      <UpdateLevelsReportModal
        open={reportModalOpen}
        reportData={reportData}
        onClose={() => setReportModalOpen(false)}
      />
    </Stack>
  );
}
