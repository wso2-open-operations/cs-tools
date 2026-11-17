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
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Pencil, Plus, Search } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { useGetCatalogProducts } from "@features/csm-products/api/useGetCatalogProducts";
import { useGetAllCatalogVersions } from "@features/csm-products/api/useGetCatalogVersions";
import AddProductDialog from "@features/csm-products/components/AddProductDialog";
import AddVersionDialog from "@features/csm-products/components/AddVersionDialog";
import CatalogStatusChip from "@features/csm-products/components/CatalogStatusChip";
import VersionEditDialog from "@features/csm-products/components/VersionEditDialog";
import { formatCatalogDate } from "@features/csm-products/utils/catalogFormat";
import {
  catalogListings,
  filterCatalogListings,
  isListedProduct,
  productDisplayCode,
  type CatalogListing,
} from "@features/csm-products/utils/catalogProducts";

const SKELETON_ROWS = 8;

/**
 * Catalogue tab: one table of product versions, starting with the product
 * code (the full name on hover), searchable by code, name or version. Each row
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
  const [addingProduct, setAddingProduct] = useState(false);
  const [addingVersion, setAddingVersion] = useState(false);
  const [editing, setEditing] = useState<CatalogListing | null>(null);

  const products = useGetCatalogProducts();
  const versions = useGetAllCatalogVersions();
  const listings = useMemo(
    () => catalogListings(products.data ?? [], versions.data ?? []),
    [products.data, versions.data],
  );
  const visible = useMemo(() => filterCatalogListings(listings, search), [listings, search]);
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
          sx={{ width: 300 }}
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
                  <TableCell>Released</TableCell>
                  <TableCell>Earliest EOL</TableCell>
                  <TableCell>EOL announced</TableCell>
                  <TableCell>EOL</TableCell>
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
