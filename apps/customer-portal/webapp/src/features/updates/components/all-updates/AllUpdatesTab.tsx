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
  Select,
  Skeleton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { FileText } from "@wso2/oxygen-ui-icons-react";
import searchingSvg from "@assets/search/searching.svg";
import { useCallback, useEffect, useMemo, useState, type JSX } from "react";
import { useNavigate, useParams } from "react-router";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import { useGetProductUpdateLevels } from "@features/updates/api/useGetProductUpdateLevels";
import { usePostUpdateLevelsSearch } from "@features/updates/api/usePostUpdateLevelsSearch";
import { PendingUpdatesList } from "@features/updates/components/pending-updates/PendingUpdatesList";
import PendingUpdatesListSkeleton from "@features/updates/components/pending-updates/PendingUpdatesListSkeleton";
import EmptyState from "@components/empty-state/EmptyState";
import error500Svg from "@assets/error/error-500.svg";
import UpdateLevelsReportModal from "@features/updates/components/all-updates/UpdateLevelsReportModal";
import type {
  AllUpdatesTabFilterState,
  AllUpdatesTabSearchParams,
} from "@features/updates/types/updates";
import {
  ALL_UPDATES_CLEAR_FILTERS_BUTTON_LABEL,
  ALL_UPDATES_END_LEVEL_LABEL,
  ALL_UPDATES_FILTER_OPTIONS_ERROR_MESSAGE,
  ALL_UPDATES_IDLE_HINT,
  ALL_UPDATES_PRODUCT_LABEL,
  ALL_UPDATES_SEARCH_BUTTON_LABEL,
  ALL_UPDATES_SEARCH_ERROR_MESSAGE,
  ALL_UPDATES_SECTION_TITLE,
  ALL_UPDATES_SELECT_LEVEL_PLACEHOLDER,
  ALL_UPDATES_SELECT_PRODUCT_PLACEHOLDER,
  ALL_UPDATES_SELECT_VERSION_PLACEHOLDER,
  ALL_UPDATES_START_LEVEL_LABEL,
  ALL_UPDATES_TAB_INITIAL_FILTER,
  ALL_UPDATES_VERSION_LABEL,
  ALL_UPDATES_VIEW_REPORT_BUTTON_LABEL,
  ALL_UPDATES_EMPTY_SEARCH_MESSAGE,
} from "@features/updates/constants/updatesConstants";
import { EMPTY_DROPDOWN_PLACEHOLDER } from "@constants/common";
import { getUpdateLevelsReportData } from "@features/updates/utils/updateLevelsReportPdf";
import {
  getNextAllUpdatesFilterAfterChange,
  getProductNamesFromProductLevels,
  getUpdateLevelsForProductVersion,
  getVersionEntriesForProduct,
  validateAllUpdatesFilter,
} from "@features/updates/utils/allUpdatesTab";

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

  const [filter, setFilter] = useState<AllUpdatesTabFilterState>(
    ALL_UPDATES_TAB_INITIAL_FILTER,
  );
  const [reportModalOpen, setReportModalOpen] = useState(false);
  const [searchParams, setSearchParams] =
    useState<AllUpdatesTabSearchParams | null>(null);

  const {
    data: productLevelsData,
    isLoading: isProductLevelsLoading,
    isError: isProductLevelsError,
  } = useGetProductUpdateLevels();

  const {
    data: searchData,
    isLoading: isSearchLoading,
    isError: isSearchError,
  } = usePostUpdateLevelsSearch(searchParams);

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
    return startLevelOptions.filter((level) => level > start);
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
    (field: keyof AllUpdatesTabFilterState) =>
      (e: SelectChangeEvent<string>) => {
        const value = e.target.value;
        setFilter((prev) =>
          getNextAllUpdatesFilterAfterChange(prev, field, value),
        );
      },
    [],
  );

  const handleSearch = useCallback(() => {
    const result = validateAllUpdatesFilter(filter);
    if (!result.valid) return;
    setSearchParams({
      productName: filter.productName,
      productVersion: filter.productVersion,
      startingUpdateLevel: result.start,
      endingUpdateLevel: result.end,
    });
  }, [filter]);

  const handleClearFilters = useCallback(() => {
    setFilter(ALL_UPDATES_TAB_INITIAL_FILTER);
    setSearchParams(null);
  }, []);

  const handleView = useCallback(
    (levelKey: string) => {
      if (!searchParams || !projectId) return;
      const params = new URLSearchParams({
        productName: searchParams.productName,
        productBaseVersion: searchParams.productVersion,
        startingUpdateLevel: String(searchParams.startingUpdateLevel),
        endingUpdateLevel: String(searchParams.endingUpdateLevel),
      });
      navigate(
        `/projects/${projectId}/updates/pending/level/${levelKey}?${params}`,
      );
    },
    [navigate, projectId, searchParams],
  );

  const reportData = useMemo(() => {
    if (!searchData || !searchParams || Object.keys(searchData).length === 0)
      return null;
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

  const canSearch = validateAllUpdatesFilter(filter).valid;

  const canViewReport = !!reportData;

  const hasActiveFilter =
    filter.productName !== "" ||
    filter.productVersion !== "" ||
    filter.startLevel !== "" ||
    filter.endLevel !== "";
  const canClear = hasActiveFilter || searchParams !== null;

  if (isProductLevelsLoading) {
    return (
      <Stack spacing={3} sx={{ width: "100%" }}>
        <Card variant="outlined">
          <CardContent sx={{ p: 3 }}>
            <Typography variant="subtitle1" sx={{ mb: 2 }}>
              {ALL_UPDATES_SECTION_TITLE}
            </Typography>
            <Grid container spacing={2}>
              {[1, 2, 3, 4].map((i) => (
                <Grid key={i} size={{ xs: 12, sm: 6, md: 3 }}>
                  <Skeleton variant="rounded" height={40} />
                </Grid>
              ))}
            </Grid>
            <Box sx={{ mt: 3, display: "flex", gap: 2 }}>
              <Skeleton variant="rounded" width={120} height={36} />
              <Skeleton variant="rounded" width={160} height={36} />
            </Box>
          </CardContent>
        </Card>
        <Skeleton variant="rounded" height={240} />
      </Stack>
    );
  }

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
        <img
          src={error500Svg}
          alt=""
          aria-hidden="true"
          style={{ width: 200, height: "auto" }}
        />
        <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
          {ALL_UPDATES_FILTER_OPTIONS_ERROR_MESSAGE}
        </Typography>
      </Box>
    );
  }

  return (
    <Stack spacing={3} sx={{ width: "100%" }}>
      <Card variant="outlined">
        <CardContent sx={{ p: 3 }}>
          <Typography variant="subtitle1" sx={{ mb: 2 }}>
            {ALL_UPDATES_SECTION_TITLE}
          </Typography>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <FormControl
                fullWidth
                size="small"
                disabled={isProductLevelsLoading}
              >
                <InputLabel id="all-updates-product-label">
                  {ALL_UPDATES_PRODUCT_LABEL}
                </InputLabel>
                <Select
                  labelId="all-updates-product-label"
                  id="all-updates-product"
                  value={filter.productName}
                  label={ALL_UPDATES_PRODUCT_LABEL}
                  onChange={handleFilterChange("productName")}
                >
                  <MenuItem value="" disabled>
                    <Typography variant="body2">
                      {isProductLevelsLoading
                        ? ALL_UPDATES_SELECT_PRODUCT_PLACEHOLDER
                        : productNames.length === 0
                          ? EMPTY_DROPDOWN_PLACEHOLDER
                          : ALL_UPDATES_SELECT_PRODUCT_PLACEHOLDER}
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
              <FormControl
                fullWidth
                size="small"
                disabled={isProductLevelsLoading || !filter.productName}
              >
                <InputLabel id="all-updates-version-label">
                  {ALL_UPDATES_VERSION_LABEL}
                </InputLabel>
                <Select
                  labelId="all-updates-version-label"
                  id="all-updates-version"
                  value={filter.productVersion}
                  label={ALL_UPDATES_VERSION_LABEL}
                  onChange={handleFilterChange("productVersion")}
                >
                  <MenuItem value="" disabled>
                    <Typography variant="body2">
                      {!filter.productName
                        ? ALL_UPDATES_SELECT_VERSION_PLACEHOLDER
                        : isProductLevelsLoading
                          ? ALL_UPDATES_SELECT_VERSION_PLACEHOLDER
                          : versionEntries.length === 0
                            ? EMPTY_DROPDOWN_PLACEHOLDER
                            : ALL_UPDATES_SELECT_VERSION_PLACEHOLDER}
                    </Typography>
                  </MenuItem>
                  {versionEntries.map((v) => (
                    <MenuItem
                      key={v.productBaseVersion}
                      value={v.productBaseVersion}
                    >
                      <Typography variant="body2">
                        {v.productBaseVersion}
                      </Typography>
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <FormControl
                fullWidth
                size="small"
                disabled={!filter.productVersion}
              >
                <InputLabel id="all-updates-start-label">
                  {ALL_UPDATES_START_LEVEL_LABEL}
                </InputLabel>
                <Select
                  labelId="all-updates-start-label"
                  id="all-updates-start"
                  value={filter.startLevel}
                  label={ALL_UPDATES_START_LEVEL_LABEL}
                  onChange={handleFilterChange("startLevel")}
                >
                  <MenuItem value="" disabled>
                    <Typography variant="body2">
                      {!filter.productVersion
                        ? ALL_UPDATES_SELECT_LEVEL_PLACEHOLDER
                        : startLevelOptions.length === 0
                          ? EMPTY_DROPDOWN_PLACEHOLDER
                          : ALL_UPDATES_SELECT_LEVEL_PLACEHOLDER}
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
                <InputLabel id="all-updates-end-label">
                  {ALL_UPDATES_END_LEVEL_LABEL}
                </InputLabel>
                <Select
                  labelId="all-updates-end-label"
                  id="all-updates-end"
                  value={filter.endLevel}
                  label={ALL_UPDATES_END_LEVEL_LABEL}
                  onChange={handleFilterChange("endLevel")}
                >
                  <MenuItem value="" disabled>
                    <Typography variant="body2">
                      {!filter.startLevel
                        ? ALL_UPDATES_SELECT_LEVEL_PLACEHOLDER
                        : endLevelOptions.length === 0
                          ? EMPTY_DROPDOWN_PLACEHOLDER
                          : ALL_UPDATES_SELECT_LEVEL_PLACEHOLDER}
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
          <Stack direction="row" spacing={2} sx={{ mt: 3 }} alignItems="center">
            <Button
              variant="contained"
              color="warning"
              onClick={handleSearch}
              disabled={!canSearch || isSearchLoading}
            >
              {ALL_UPDATES_SEARCH_BUTTON_LABEL}
            </Button>
            <Button
              variant="text"
              onClick={handleClearFilters}
              disabled={!canClear}
            >
              {ALL_UPDATES_CLEAR_FILTERS_BUTTON_LABEL}
            </Button>
            <Button
              variant="outlined"
              color="warning"
              startIcon={<FileText size={18} />}
              onClick={handleViewReport}
              disabled={!canViewReport}
            >
              {ALL_UPDATES_VIEW_REPORT_BUTTON_LABEL}
            </Button>
          </Stack>
        </CardContent>
      </Card>

      {!searchParams ? (
        <Box sx={{ textAlign: "center", py: 8 }}>
          <img
            src={searchingSvg}
            alt=""
            aria-hidden="true"
            style={{ width: 180, height: "auto", opacity: 0.85 }}
          />
          <Typography
            variant="body1"
            sx={{ mt: 2.5, fontWeight: 500, color: "text.secondary" }}
          >
            Search for updates
          </Typography>
          <Typography variant="body2" sx={{ mt: 0.5, whiteSpace: "nowrap" }}>
            {ALL_UPDATES_IDLE_HINT}
          </Typography>
        </Box>
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
          <img
            src={error500Svg}
            alt=""
            aria-hidden="true"
            style={{ width: 200, height: "auto" }}
          />
          <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
            {ALL_UPDATES_SEARCH_ERROR_MESSAGE}
          </Typography>
        </Box>
      ) : searchData && Object.keys(searchData).length === 0 ? (
        <EmptyState description={ALL_UPDATES_EMPTY_SEARCH_MESSAGE} />
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
