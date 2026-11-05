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
  IconButton,
  InputAdornment,
  MenuItem,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TableSortLabel,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Pencil, Plus, Search } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import MultiSelectField from "@components/MultiSelectField";
import QueryErrorState from "@components/QueryErrorState";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { useGetCatalogProducts } from "@features/csm-products/api/useGetCatalogProducts";
import { useGetAllCatalogVersions } from "@features/csm-products/api/useGetCatalogVersions";
import AddProductDialog from "@features/csm-products/components/AddProductDialog";
import AddVersionDialog from "@features/csm-products/components/AddVersionDialog";
import CatalogStatusChip from "@features/csm-products/components/CatalogStatusChip";
import VersionEditDialog from "@features/csm-products/components/VersionEditDialog";
import { nextSort, type ListSort } from "@features/csm-operations/utils/listSort";
import type { CatalogSupportStatus } from "@features/csm-products/types/productCatalog";
import { formatCatalogDate, SUPPORT_STATUS_LABEL } from "@features/csm-products/utils/catalogFormat";
import {
  applyListingFilters,
  catalogListings,
  filterCatalogListings,
  hasListingFilters,
  isListedProduct,
  NO_LISTING_FILTERS,
  productDisplayCode,
  sortListings,
  type CatalogDateField,
  type CatalogEolFilter,
  type CatalogListing,
  type CatalogListingFilters,
} from "@features/csm-products/utils/catalogProducts";

const SKELETON_ROWS = 8;

/** The statuses a listed version can have (discontinued versions are not listed). */
const STATUS_OPTIONS: { value: CatalogSupportStatus; label: string }[] = (
  ["available", "extended", "deprecated"] as const
).map((value) => ({ value, label: SUPPORT_STATUS_LABEL[value] }));

const DATE_COLUMNS: { field: CatalogDateField; label: string }[] = [
  { field: "releaseDate", label: "Released" },
  { field: "earliestPossibleSupportEolDate", label: "Earliest EOL" },
  { field: "supportEolAnnouncementDate", label: "EOL announced" },
  { field: "supportEolDate", label: "EOL" },
];

const EOL_OPTIONS: { value: CatalogEolFilter; label: string }[] = [
  { value: "all", label: "Any" },
  { value: "past", label: "Past EOL" },
  { value: "within1Month", label: "EOL within 1 month" },
  { value: "within3Months", label: "EOL within 3 months" },
  { value: "within6Months", label: "EOL within 6 months" },
  { value: "none", label: "No EOL date" },
];

/** Today as "YYYY-MM-DD" in the viewer's own time zone. */
function todayLocal(): string {
  const now = new Date();
  const pad = (n: number): string => String(n).padStart(2, "0");
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}

/**
 * Products tab: one table of product versions, starting with the product
 * code (the full name on hover), searchable by code, name or version and
 * filterable by product, status and EOL; the date columns sort on a click. Each row
 * is one version, its deployment profiles folded together since they share
 * their status and dates. Cloud (SaaS) products, end-of-life products and
 * retired or discontinued versions are never listed. A catalogue manager can
 * add a product, add a version and edit a version's support status and dates,
 * retiring it from the same dialog; every save applies at once.
 */
export default function ProductCatalogPage(): JSX.Element {
  const canManage = usePortalAccess().canManageProductCatalog;
  const { showSuccess } = useSuccessBanner();
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<CatalogListingFilters>(NO_LISTING_FILTERS);
  // Null keeps the default order: product code, newest release first.
  const [sort, setSort] = useState<ListSort<CatalogDateField> | null>(null);
  const [addingProduct, setAddingProduct] = useState(false);
  const [addingVersion, setAddingVersion] = useState(false);
  const [editing, setEditing] = useState<CatalogListing | null>(null);

  const products = useGetCatalogProducts();
  const versions = useGetAllCatalogVersions();
  const listings = useMemo(
    () => catalogListings(products.data ?? [], versions.data ?? []),
    [products.data, versions.data],
  );
  const visible = useMemo(
    () => sortListings(applyListingFilters(filterCatalogListings(listings, search), filters, todayLocal()), sort),
    [listings, search, filters, sort],
  );
  // The products that have a listed version, by code, for the Product filter.
  const productOptions = useMemo(() => {
    const seen = new Map<string, string>();
    for (const { product } of listings) seen.set(product.id, productDisplayCode(product));
    return [...seen].map(([value, label]) => ({ value, label })).sort((a, b) => a.label.localeCompare(b.label));
  }, [listings]);
  const filtered = hasListingFilters(filters) || search.trim() !== "";
  // A version can only be added to a listed (on-premise, not end-of-life) product.
  const versionTargets = useMemo(() => (products.data ?? []).filter(isListedProduct), [products.data]);
  const loading = products.isLoading || versions.isLoading;
  const failed = products.isError ? products : versions.isError ? versions : null;
  const columnCount = canManage ? 8 : 7;

  const saved = (message: string): void => {
    setAddingProduct(false);
    setAddingVersion(false);
    setEditing(null);
    showSuccess(message);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", gap: 2, alignItems: "center", flexWrap: "wrap" }}>
        <TextField
          size="small"
          placeholder="Search by code, name or version"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          sx={{ width: 260 }}
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <Search size={16} />
                </InputAdornment>
              ),
            },
          }}
        />
        <Box sx={{ width: 200 }}>
          <MultiSelectField
            id="catalog-filter-product"
            label="Product"
            values={filters.productIds}
            options={productOptions}
            onChange={(productIds) => setFilters((f) => ({ ...f, productIds }))}
          />
        </Box>
        <Box sx={{ width: 180 }}>
          <MultiSelectField
            id="catalog-filter-status"
            label="Status"
            values={filters.statuses}
            options={STATUS_OPTIONS}
            onChange={(statuses) => setFilters((f) => ({ ...f, statuses }))}
          />
        </Box>
        <TextField
          select
          size="small"
          id="catalog-filter-eol"
          label="EOL"
          value={filters.eol}
          onChange={(event) => setFilters((f) => ({ ...f, eol: event.target.value as CatalogEolFilter }))}
          sx={{ width: 190 }}
        >
          {EOL_OPTIONS.map((option) => (
            <MenuItem key={option.value} value={option.value}>
              {option.label}
            </MenuItem>
          ))}
        </TextField>
        {filtered && (
          <Button
            size="small"
            onClick={() => {
              setSearch("");
              setFilters(NO_LISTING_FILTERS);
            }}
          >
            Clear
          </Button>
        )}
        {!loading && !failed && (
          <Typography variant="body2" color="text.secondary">
            {visible.length} of {listings.length} versions
          </Typography>
        )}
        {canManage && (
          <Box sx={{ display: "flex", gap: 1, ml: "auto" }}>
            <Button variant="outlined" size="small" startIcon={<Plus size={16} />} onClick={() => setAddingProduct(true)}>
              Add product
            </Button>
            <Button variant="contained" size="small" startIcon={<Plus size={16} />} onClick={() => setAddingVersion(true)}>
              Add product version
            </Button>
          </Box>
        )}
      </Box>

      {failed ? (
        <QueryErrorState
          message="Could not load the product catalogue."
          error={failed.error}
          onRetry={() => {
            void products.refetch();
            void versions.refetch();
          }}
        />
      ) : (
        <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
          <TableContainer sx={{ maxHeight: "calc(100vh - 280px)" }}>
            <Table size="small" stickyHeader sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
              <TableHead>
                <TableRow>
                  <TableCell>Product code</TableCell>
                  <TableCell>Version</TableCell>
                  <TableCell>Status</TableCell>
                  {DATE_COLUMNS.map((column) => {
                    const active = sort?.field === column.field;
                    return (
                      <TableCell key={column.field} sortDirection={active ? sort.order : false}>
                        <TableSortLabel
                          active={active}
                          direction={active ? sort.order : "desc"}
                          onClick={() =>
                            setSort(sort ? nextSort(sort, column.field) : { field: column.field, order: "desc" })
                          }
                        >
                          {column.label}
                        </TableSortLabel>
                      </TableCell>
                    );
                  })}
                  {canManage && <TableCell align="right">Actions</TableCell>}
                </TableRow>
              </TableHead>
              <TableBody>
                {loading &&
                  Array.from({ length: SKELETON_ROWS }).map((_, i) => (
                    <TableRow key={`s-${i}`}>
                      {Array.from({ length: columnCount }).map((__, c) => (
                        <TableCell key={c}>
                          <Skeleton />
                        </TableCell>
                      ))}
                    </TableRow>
                  ))}
                {!loading && visible.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={columnCount}>
                      <Typography variant="body2" color="text.secondary">
                        No versions match.
                      </Typography>
                    </TableCell>
                  </TableRow>
                )}
                {!loading &&
                  visible.map((listing) => {
                    const { product, version } = listing;
                    const label = `${productDisplayCode(product)} ${version.version}${listing.named && version.publicName ? ` ${version.publicName}` : ""}`;
                    return (
                      <TableRow key={listing.key} hover>
                        <TableCell sx={{ maxWidth: 240 }}>
                          <Tooltip title={product.name}>
                            <Typography variant="body2" component="span" sx={{ fontWeight: 600 }}>
                              {productDisplayCode(product)}
                            </Typography>
                          </Tooltip>
                        </TableCell>
                        <TableCell sx={{ whiteSpace: "nowrap" }}>
                          {version.version}
                          {listing.named && version.publicName && (
                            <Typography variant="caption" color="text.secondary" sx={{ display: "block" }}>
                              {version.publicName}
                            </Typography>
                          )}
                        </TableCell>
                        <TableCell>
                          <CatalogStatusChip status={version.currentSupportStatus} />
                        </TableCell>
                        <TableCell sx={{ whiteSpace: "nowrap" }}>{formatCatalogDate(version.releaseDate)}</TableCell>
                        <TableCell sx={{ whiteSpace: "nowrap" }}>
                          {formatCatalogDate(version.earliestPossibleSupportEolDate)}
                        </TableCell>
                        <TableCell sx={{ whiteSpace: "nowrap" }}>
                          {formatCatalogDate(version.supportEolAnnouncementDate)}
                        </TableCell>
                        <TableCell sx={{ whiteSpace: "nowrap" }}>{formatCatalogDate(version.supportEolDate)}</TableCell>
                        {canManage && (
                          <TableCell align="right">
                            <Tooltip title="Edit">
                              <IconButton size="small" aria-label={`Edit ${label}`} onClick={() => setEditing(listing)}>
                                <Pencil size={16} />
                              </IconButton>
                            </Tooltip>
                          </TableCell>
                        )}
                      </TableRow>
                    );
                  })}
              </TableBody>
            </Table>
          </TableContainer>
        </Box>
      )}

      {addingProduct && <AddProductDialog onClose={() => setAddingProduct(false)} onSaved={saved} />}
      {addingVersion && (
        <AddVersionDialog products={versionTargets} onClose={() => setAddingVersion(false)} onSaved={saved} />
      )}
      {editing && (
        <VersionEditDialog
          product={editing.product}
          rows={editing.rows}
          listingName={editing.named ? editing.version.publicName : null}
          onClose={() => setEditing(null)}
          onSaved={saved}
        />
      )}
    </Box>
  );
}
