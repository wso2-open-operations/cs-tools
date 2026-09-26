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


import { useCSUsers, useOrganizations, useProducts } from "@features/plg/api/hooks";
import { EmptyState, ErrorBlock, LoadingBlock, PageHeader, StatusChip } from "@features/plg/components/common";
import { LIFECYCLE_STAGES, type LifecycleStage } from "@features/plg/api/types";
import { formatDate, humanizeEnum } from "@features/plg/utils/format";
import { selectLabelProps } from "@features/plg/components/selectLabelProps";

const PAGE_SIZE = 25;

/**
 * The organisation list.
 *
 * Five columns, which is what the requirement asks for: the organisation, its
 * platforms and their stages, when it registered, who owns it, and which address
 * registered it. Filtering by product, stage or tier matches an organisation when
 * any of its pairings match — the only coherent reading once the stage belongs to
 * the pairing rather than the customer.
 */
export default function OrganizationsPage() {
  const navigate = useNavigate();
  const { data: products } = useProducts();
  const { data: owners } = useCSUsers();

  const [query, setQuery] = useState("");
  const [productCode, setProductCode] = useState("");
  const [stage, setStage] = useState("");
  const [ownerId, setOwnerId] = useState("");
  const [page, setPage] = useState(0);

  const request = useMemo(
    () => ({
      filters: {
        query: query.trim() || undefined,
        productCodes: productCode ? [productCode] : undefined,
        lifecycleStages: stage ? [stage as LifecycleStage] : undefined,
        ownerIds: ownerId && ownerId !== "UNASSIGNED" ? [ownerId] : undefined,
        unowned: ownerId === "UNASSIGNED" ? true : undefined,
      },
      pagination: { limit: PAGE_SIZE, offset: page * PAGE_SIZE },
      sortBy: "registered" as const,
      sortOrder: "desc" as const,
    }),
    [query, productCode, stage, ownerId, page],
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
          <Grid size={{ xs: 12, md: 4 }}>
            <TextField
              fullWidth
              size="small"
              label="Search by organisation or email"
              value={query}
              onChange={(e) => resetTo(setQuery)(e.target.value)}
            />
          </Grid>
          <Grid size={{ xs: 12, sm: 4, md: 2.5 }}>
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
          <Grid size={{ xs: 12, sm: 4, md: 2.5 }}>
            <TextField
              select
              {...selectLabelProps(stage)}
              fullWidth
              size="small"
              label="Lifecycle stage"
              value={stage}
              onChange={(e) => resetTo(setStage)(e.target.value)}
            >
              <MenuItem value="">All stages</MenuItem>
              {LIFECYCLE_STAGES.map((s) => (
                <MenuItem key={s} value={s}>
                  {humanizeEnum(s)}
                </MenuItem>
              ))}
            </TextField>
          </Grid>
          <Grid size={{ xs: 12, sm: 4, md: 2.5 }}>
            <TextField
              select
              {...selectLabelProps(ownerId)}
              fullWidth
              size="small"
              label="CS owner"
              value={ownerId}
              onChange={(e) => resetTo(setOwnerId)(e.target.value)}
            >
              <MenuItem value="">Anyone</MenuItem>
              <MenuItem value="UNASSIGNED">Unassigned</MenuItem>
              {(owners ?? []).map((u) => (
                <MenuItem key={u.id} value={u.id}>
                  {u.name}
                </MenuItem>
              ))}
            </TextField>
          </Grid>
          <Grid size={{ xs: 12, md: 0.5 }}>
            <Button
              fullWidth
              onClick={() => {
                setQuery("");
                setProductCode("");
                setStage("");
                setOwnerId("");
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
                  <TableCell>Platforms and lifecycle stage</TableCell>
                  <TableCell>Registered</TableCell>
                  <TableCell>PLG CS owner</TableCell>
                  <TableCell>Registered email</TableCell>
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
                      <Typography variant="body2" fontWeight={600}>
                        {org.organizationName}
                      </Typography>
                      {org.registeredName ? (
                        <Typography variant="caption" color="text.secondary">
                          {org.registeredName}
                        </Typography>
                      ) : null}
                    </TableCell>
                    <TableCell>
                      <Stack direction="row" spacing={0.75} flexWrap="wrap" useFlexGap>
                        {org.platformStages.map((ps) => (
                          <Box key={ps.orgPlatformId} display="flex" alignItems="center" gap={0.5}>
                            <Chip
                              size="small"
                              variant="outlined"
                              label={ps.product.name}
                              sx={{ fontWeight: 600 }}
                            />
                            <StatusChip value={ps.lifecycleStage} kind="lifecycle" />
                            {ps.isNew ? <Chip size="small" color="warning" label="New" /> : null}
                          </Box>
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
                      <Typography variant="body2" color="text.secondary">
                        {org.registeredEmail}
                      </Typography>
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
