import {
  Box,
  Button,
  Card,
  Chip,
  Grid,
  MenuItem,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState } from "react";
import { useNavigate } from "react-router";


import { useOrganizations, useProducts } from "@features/plg/api/hooks";
import { EmptyState, ErrorBlock, LoadingBlock, PageHeader } from "@features/plg/components/common";
import { CompanyName, MaskedEmail, RevealButton } from "@features/plg/components/CompanyIdentity";
import { SUBSCRIPTION_TIERS, type SubscriptionTier, type UserRef } from "@features/plg/api/types";
import { CSUserSelect } from "@features/plg/components/CSUserSelect";
import { formatDate, humanizeEnum } from "@features/plg/utils/format";
import { selectLabelProps } from "@features/plg/components/selectLabelProps";

const PAGE_SIZE = 25;

/**
 * The organisation list.
 *
 * Five columns: the organisation, its platforms, when it registered, who owns
 * it, and which address registered it. Filtering by product or tier matches an
 * organisation when any of its pairings match.
 */
/**
 * The two rows that are not engineers. Sentinel ids the query layer already
 * understands: "" is no owner filter at all, "UNASSIGNED" asks for pairings
 * nobody owns.
 */
const OWNER_FILTER_OPTIONS: UserRef[] = [
  { id: "", email: "", name: "Anyone" },
  { id: "UNASSIGNED", email: "", name: "Unassigned" },
];

export default function OrganizationsPage() {
  const navigate = useNavigate();
  const { data: products } = useProducts();

  const [query, setQuery] = useState("");
  const [productCode, setProductCode] = useState("");
  const [tier, setTier] = useState("");
  const [ownerId, setOwnerId] = useState("");
  const [ownerRef, setOwnerRef] = useState<UserRef | null>(null);
  // Bumped by Clear to remount the owner picker.
  //
  // The picker holds its own search term, and that term is state Clear has no
  // other way to reach: resetting ownerId alone left the dropdown still showing
  // the previous search — the field read "Anyone" and the table unfiltered,
  // while the list underneath was still the matches for whatever had been typed.
  // Remounting retires the child's state with the parent's, which an effect
  // syncing the two could always drift out of.
  const [pickerKey, setPickerKey] = useState(0);
  const [page, setPage] = useState(0);

  const request = useMemo(
    () => ({
      filters: {
        query: query.trim() || undefined,
        productCodes: productCode ? [productCode] : undefined,
        subscriptionTiers: tier ? [tier as SubscriptionTier] : undefined,
        ownerIds: ownerId && ownerId !== "UNASSIGNED" ? [ownerId] : undefined,
        unowned: ownerId === "UNASSIGNED" ? true : undefined,
      },
      pagination: { limit: PAGE_SIZE, offset: page * PAGE_SIZE },
      sortBy: "registered" as const,
      sortOrder: "desc" as const,
    }),
    [query, productCode, tier, ownerId, page],
  );

  const { data, isPending, error } = useOrganizations(request);

  const resetTo = (setter: (value: string) => void) => (value: string) => {
    setter(value);
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column" }}>
      {/* One root element, because csm-portal's AppLayout renders <Outlet /> into a
          column flex box. A fragment made each child of this page its own flex item,
          and MUI Card is overflow:hidden — so flexbox shrank the filter card and it
          clipped instead of the page scrolling. */}
      <PageHeader
        title="Organisations"
        subtitle="Every customer that has registered for a WSO2 Cloud platform"
      />

      <Card sx={{ p: 2, mb: 2 }}>
        <Grid container spacing={1.5}>
          <Grid size={{ xs: 12, md: 5 }}>
            <TextField
              fullWidth
              size="small"
              label="Search by company id, organisation or email"
              value={query}
              onChange={(e) => resetTo(setQuery)(e.target.value)}
            />
          </Grid>
          <Grid size={{ xs: 12, sm: 6, md: 2 }}>
            <TextField
              select
              {...selectLabelProps(productCode)}
              fullWidth
              size="small"
              label="Platform"
              value={productCode}
              onChange={(e) => resetTo(setProductCode)(e.target.value)}
            >
              <MenuItem value="">All platforms</MenuItem>
              {(products ?? []).map((p) => (
                <MenuItem key={p.code} value={p.code}>
                  {p.name}
                </MenuItem>
              ))}
            </TextField>
          </Grid>
          <Grid size={{ xs: 12, sm: 6, md: 2 }}>
            <TextField
              select
              {...selectLabelProps(tier)}
              fullWidth
              size="small"
              label="Subscription tier"
              value={tier}
              onChange={(e) => resetTo(setTier)(e.target.value)}
            >
              <MenuItem value="">All tiers</MenuItem>
              {SUBSCRIPTION_TIERS.map((t) => (
                <MenuItem key={t} value={t}>
                  {humanizeEnum(t)}
                </MenuItem>
              ))}
            </TextField>
          </Grid>
          <Grid size={{ xs: 12, sm: 6, md: 2 }}>
            <CSUserSelect
              key={pickerKey}
              label="CS owner"
              value={ownerId}
              extraOptions={OWNER_FILTER_OPTIONS}
              selected={ownerRef}
              onChange={(id, user) => {
                resetTo(setOwnerId)(id);
                // Kept so the name survives once the picker's page moves on —
                // the chosen engineer is often not in the default 100.
                setOwnerRef(user && user.id !== "" && user.id !== "UNASSIGNED" ? user : null);
              }}
            />
          </Grid>
          <Grid
            size={{ xs: 12, md: 1 }}
            sx={{ display: "flex", alignItems: "center", justifyContent: "center" }}
          >
            <Button
              onClick={() => {
                setQuery("");
                setProductCode("");
                setTier("");
                setOwnerId("");
                // Without this the picker would keep offering the engineer it
                // last pinned, as an option nothing had selected.
                setOwnerRef(null);
                setPickerKey((n) => n + 1);
                setPage(0);
              }}
            >
              Clear
            </Button>
          </Grid>
        </Grid>
      </Card>

      {error ? <ErrorBlock error={error} /> : null}
      {isPending && !data ? <LoadingBlock height={400} /> : null}

      {data ? (
        <Card>
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Organisation</TableCell>
                  <TableCell>Platforms</TableCell>
                  <TableCell>Registered</TableCell>
                  <TableCell>PLG CS owner</TableCell>
                  <TableCell>Registered email</TableCell>
                  <TableCell padding="checkbox" aria-label="Reveal" />
                </TableRow>
              </TableHead>
              <TableBody>
                {data.organizations.map((org) => (
                  <TableRow
                    key={org.id}
                    hover
                    sx={{ cursor: "pointer" }}
                    onClick={() => navigate(`/plg/organizations/${org.id}`)}
                  >
                    <TableCell>
                      <CompanyName organizationId={org.id} companyId={org.moesifCompanyId} fontWeight={600} />
                    </TableCell>
                    <TableCell>
                      <Stack direction="row" spacing={0.75} flexWrap="wrap" useFlexGap>
                        {org.platformStages.map((ps) => (
                          <Chip
                            key={ps.orgPlatformId}
                            size="small"
                            variant="outlined"
                            label={ps.product.name}
                            sx={{ fontWeight: 600 }}
                          />
                        ))}
                      </Stack>
                    </TableCell>
                    <TableCell>{formatDate(org.registeredOn)}</TableCell>
                    <TableCell>
                      {org.owner ? (
                        org.owner.name
                      ) : (
                        <Typography variant="body2" color="warning.main">
                          Unassigned
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <MaskedEmail organizationId={org.id} />
                    </TableCell>
                    <TableCell padding="checkbox">
                      <RevealButton organizationId={org.id} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>

          {data.organizations.length === 0 ? <EmptyState message="No organisations match these filters" /> : null}

          <TablePagination
            component="div"
            count={data.total}
            page={page}
            rowsPerPage={PAGE_SIZE}
            rowsPerPageOptions={[PAGE_SIZE]}
            onPageChange={(_, next) => setPage(next)}
          />
        </Card>
      ) : null}
    </Box>
  );
}
