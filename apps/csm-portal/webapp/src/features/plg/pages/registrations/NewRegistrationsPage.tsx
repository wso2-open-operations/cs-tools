import {
  Box,
  Button,
  Card,
  Chip,
  FormControlLabel,
  Grid,
  MenuItem,
  Stack,
  Switch,
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


import { useAcknowledge, useProducts, useRegistrations } from "@features/plg/api/hooks";
import { EmptyState, ErrorBlock, LoadingBlock, PageHeader, StatusChip } from "@features/plg/components/common";
import { formatDate, formatRelative } from "@features/plg/utils/format";
import { selectLabelProps } from "@features/plg/components/selectLabelProps";

const PAGE_SIZE = 25;

/**
 * New registrations.
 *
 * "New" is the absence of an acknowledgement, not a status somebody has to
 * remember to change: a pairing leaves this list the moment an engineer claims
 * it. Acknowledging also assigns the organisation's owner if it has none, since
 * ownership is per customer.
 */
export default function NewRegistrationsPage() {
  const navigate = useNavigate();
  const { data: products } = useProducts();
  const acknowledge = useAcknowledge();

  const [unacknowledgedOnly, setUnacknowledgedOnly] = useState(true);
  const [productCode, setProductCode] = useState("");
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(0);

  const request = useMemo(
    () => ({
      unacknowledgedOnly,
      filters: {
        query: query.trim() || undefined,
        productCodes: productCode ? [productCode] : undefined,
      },
      pagination: { limit: PAGE_SIZE, offset: page * PAGE_SIZE },
    }),
    [unacknowledgedOnly, query, productCode, page],
  );

  const { data, isPending, error } = useRegistrations(request);

  return (
    <Box sx={{ display: "flex", flexDirection: "column" }}>
      {/* One root element, because csm-portal's AppLayout renders <Outlet /> into a
          column flex box. A fragment made each child of this page its own flex item,
          and MUI Card is overflow:hidden — so flexbox shrank the filter card and it
          clipped instead of the page scrolling. */}
      <PageHeader
        title="New registrations"
        subtitle="Registrations nobody has picked up yet, newest first"
      />

      <Card sx={{ p: 2, mb: 2 }}>
        <Grid container spacing={1.5} alignItems="center">
          <Grid size={{ xs: 12, md: 4 }}>
            <TextField
              fullWidth
              size="small"
              label="Search by organisation or email"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                setPage(0);
              }}
            />
          </Grid>
          <Grid size={{ xs: 12, sm: 6, md: 3 }}>
            <TextField
              select
              {...selectLabelProps(productCode)}
              fullWidth
              size="small"
              label="Platform"
              value={productCode}
              onChange={(e) => {
                setProductCode(e.target.value);
                setPage(0);
              }}
            >
              <MenuItem value="">All platforms</MenuItem>
              {(products ?? []).map((p) => (
                <MenuItem key={p.code} value={p.code}>
                  {p.name}
                </MenuItem>
              ))}
            </TextField>
          </Grid>
          <Grid size={{ xs: 12, sm: 6, md: 5 }}>
            <FormControlLabel
              control={
                <Switch
                  checked={unacknowledgedOnly}
                  onChange={(e) => {
                    setUnacknowledgedOnly(e.target.checked);
                    setPage(0);
                  }}
                />
              }
              label="Unacknowledged only"
            />
          </Grid>
        </Grid>
      </Card>

      {error ? <ErrorBlock error={error} /> : null}
      {acknowledge.error ? <ErrorBlock error={acknowledge.error} /> : null}
      {isPending && !data ? <LoadingBlock height={360} /> : null}

      {data ? (
        <Card>
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Organisation</TableCell>
                  <TableCell>Platform</TableCell>
                  <TableCell>Registered</TableCell>
                  <TableCell>Stage</TableCell>
                  <TableCell align="right">Action</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {data.registrations.map((item) => (
                  <TableRow key={item.orgPlatformId} hover>
                    <TableCell
                      sx={{ cursor: "pointer" }}
                      onClick={() => navigate(`/plg/organizations/${item.organizationId}/${item.product.code}`)}
                    >
                      <Typography variant="body2" fontWeight={600}>
                        {item.organizationName}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {item.registeredEmail}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Chip size="small" variant="outlined" label={item.product.name} />
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">{formatDate(item.registeredOn)}</Typography>
                      <Typography variant="caption" color="text.secondary">
                        {formatRelative(item.registeredOn)}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <StatusChip value={item.lifecycleStage} kind="lifecycle" />
                    </TableCell>
                    <TableCell align="right">
                      <Stack direction="row" spacing={1} justifyContent="flex-end">
                        <Button
                          size="small"
                          variant="contained"
                          disabled={acknowledge.isPending}
                          onClick={() => acknowledge.mutate({ orgPlatformId: item.orgPlatformId })}
                        >
                          Acknowledge
                        </Button>
                        <Button
                          size="small"
                          onClick={() =>
                            navigate(`/plg/organizations/${item.organizationId}/${item.product.code}`)
                          }
                        >
                          Open
                        </Button>
                      </Stack>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>

          {data.registrations.length === 0 ? (
            <EmptyState message="Nothing waiting — every registration has been picked up" />
          ) : null}

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
