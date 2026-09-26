import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  FormControlLabel,
  Grid,
  LinearProgress,
  MenuItem,
  Stack,
  Switch,
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
import { useMemo, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";


import { useProducts, useWorkQueue } from "@features/plg/api/hooks";
import {
  QUEUE_REASONS,
  QUEUE_REASON_LABEL,
  type QueueReason,
  type WorkQueueItem,
} from "@features/plg/api/types";
import { EmptyState, ErrorBlock, LoadingBlock, PageHeader, SectionCard, StatusChip } from "@features/plg/components/common";
import { formatDate } from "@features/plg/utils/format";
import { selectLabelProps } from "@features/plg/components/selectLabelProps";

/**
 * The work queue.
 *
 * One row per organisation+platform pairing, not per playbook run. A pairing
 * enters when it is acknowledged, sits in the owner's queue, and leaves when it
 * reaches a stage that carries no playbooks. A pairing whose owner has chosen
 * not to run anything stays — a decision not to act is still a state worth
 * seeing.
 *
 * The tiles group by product and stage, which partitions the queue exactly, so
 * they sum to the total below them. Grouping by playbook did not: a pairing
 * running two counted twice and one running none counted nowhere.
 */
export default function WorkQueuePage() {
  const navigate = useNavigate();
  const { data: products } = useProducts();

  // "Only mine" defaults on, but the dashboard tile counts the whole team, so it
  // links with ?mine=false — otherwise clicking a 7 would land on a 4.
  const [params] = useSearchParams();
  const [mine, setMine] = useState(params.get("mine") !== "false");
  const [productCode, setProductCode] = useState("");
  const [stage, setStage] = useState("");
  const [reason, setReason] = useState<QueueReason | "">("");

  const query = useMemo(
    () => ({
      mine: mine || undefined,
      product: productCode ? [productCode] : undefined,
      stage: stage ? [stage] : undefined,
      reason: reason ? [reason] : undefined,
    }),
    [mine, productCode, stage, reason],
  );

  const { data, isPending, error } = useWorkQueue(query);

  /** Clicking a tile is a toggle: click it again to clear the filter. */
  const pickTile = (code: string, tileStage: string) => {
    const same = productCode === code && stage === tileStage;
    setProductCode(same ? "" : code);
    setStage(same ? "" : tileStage);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column" }}>
      {/* One root element, because csm-portal's AppLayout renders <Outlet /> into a
          column flex box. A fragment made each child of this page its own flex item,
          and MUI Card is overflow:hidden — so flexbox shrank the filter card and it
          clipped instead of the page scrolling. */}
      <PageHeader
        title="My work queue"
        subtitle="Acknowledged pairings that still have somewhere to go"
        actions={
          <FormControlLabel
            control={<Switch checked={mine} onChange={(e) => setMine(e.target.checked)} />}
            label="Only mine"
          />
        }
      />

      {data ? (
        <Stack direction="row" spacing={1} mb={2} flexWrap="wrap" useFlexGap>
          <Chip
            label={`${data.total} pairing${data.total === 1 ? "" : "s"}`}
            color="primary"
            onClick={() => setReason("")}
            variant={reason ? "outlined" : "filled"}
          />
          {QUEUE_REASONS.map((r) => (
            <Chip
              key={r}
              label={`${QUEUE_REASON_LABEL[r]} · ${data.byReason?.[r] ?? 0}`}
              variant={reason === r ? "filled" : "outlined"}
              color={reason === r ? "primary" : "default"}
              onClick={() => setReason(reason === r ? "" : r)}
            />
          ))}
        </Stack>
      ) : null}

      <Card sx={{ p: 2, mb: 2 }}>
        <Grid container spacing={1.5}>
          <Grid size={{ xs: 12, sm: 6, md: 4 }}>
            <TextField
              select
              {...selectLabelProps(productCode)}
              fullWidth
              size="small"
              label="Platform"
              value={productCode}
              onChange={(e) => setProductCode(e.target.value)}
            >
              <MenuItem value="">All platforms</MenuItem>
              {(products ?? []).map((p) => (
                <MenuItem key={p.code} value={p.code}>
                  {p.name}
                </MenuItem>
              ))}
            </TextField>
          </Grid>
          <Grid size={{ xs: 12, md: 3 }}>
            <Button
              onClick={() => {
                setProductCode("");
                setStage("");
                setReason("");
              }}
            >
              Clear filters
            </Button>
          </Grid>
        </Grid>
      </Card>

      {error ? <ErrorBlock error={error} /> : null}
      {isPending && !data ? <LoadingBlock height={400} /> : null}

      {data ? (
        <Stack spacing={2}>
          {data.productGroups.map((group) => (
            <SectionCard
              key={group.product.code}
              title={group.product.name}
              action={<Chip size="small" label={`${group.pairings} waiting`} color="primary" />}
            >
              <Grid container spacing={1.5}>
                {group.tiles.map((tile) => {
                  const selected =
                    productCode === group.product.code && stage === tile.lifecycleStage;
                  return (
                    <Grid key={tile.lifecycleStage} size={{ xs: 12, sm: 6, md: 4, lg: 3 }}>
                      <Card
                        onClick={() => pickTile(group.product.code, tile.lifecycleStage)}
                        sx={{
                          cursor: "pointer",
                          borderColor: selected ? "primary.main" : "divider",
                          "&:hover": { borderColor: "primary.main" },
                        }}
                      >
                        <CardContent sx={{ py: 1.5, "&:last-child": { pb: 1.5 } }}>
                          <Typography variant="h5">{tile.pairings}</Typography>
                          <Typography variant="body2" fontWeight={600} noWrap>
                            {tile.stageName}
                          </Typography>
                          {/* What kind of work, not only how much. */}
                          <Stack direction="row" spacing={0.5} mt={0.75} flexWrap="wrap" useFlexGap>
                            {tile.atRisk > 0 ? (
                              <Chip size="small" color="warning" label={`${tile.atRisk} at risk`} />
                            ) : null}
                            {tile.noPlaybook > 0 ? (
                              <Chip size="small" color="warning" label={`${tile.noPlaybook} idle`} />
                            ) : null}
                            {tile.notStarted > 0 ? (
                              <Chip size="small" variant="outlined" label={`${tile.notStarted} not started`} />
                            ) : null}
                            {tile.inProgress > 0 ? (
                              <Chip size="small" color="success" label={`${tile.inProgress} running`} />
                            ) : null}
                          </Stack>
                        </CardContent>
                      </Card>
                    </Grid>
                  );
                })}
              </Grid>
            </SectionCard>
          ))}

          <Card>
            <TableContainer>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>Organisation</TableCell>
                    <TableCell>Platform</TableCell>
                    <TableCell>Stage</TableCell>
                    <TableCell>Open playbooks</TableCell>
                    <TableCell>Progress</TableCell>
                    <TableCell>Acknowledged by</TableCell>
                    {/* Whose work it is only needs saying when the list is not
                        already filtered to one person. */}
                    {mine ? null : <TableCell>Owner</TableCell>}
                    <TableCell>Registered</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {data.items.map((item) => (
                    <TableRow
                      key={item.orgPlatformId}
                      hover
                      sx={{ cursor: "pointer" }}
                      onClick={() =>
                        navigate(`/plg/organizations/${item.organizationId}/${item.product.code}`)
                      }
                    >
                      <TableCell>
                        <Typography variant="body2" fontWeight={600}>
                          {item.organizationName}
                        </Typography>
                      </TableCell>
                      <TableCell>{item.product.name}</TableCell>
                      <TableCell>
                        <StatusChip value={item.currentStage} kind="lifecycle" />
                      </TableCell>
                      <TableCell sx={{ minWidth: 220 }}>
                        <NeedsCell item={item} />
                      </TableCell>
                      <TableCell sx={{ minWidth: 130 }}>
                        {item.taskTotal > 0 ? (
                          <>
                            <Typography variant="caption" color="text.secondary">
                              {item.taskCompleted}/{item.taskTotal}
                            </Typography>
                            <LinearProgress
                              variant="determinate"
                              value={(item.taskCompleted / item.taskTotal) * 100}
                              sx={{ height: 5, borderRadius: 3, mt: 0.5 }}
                            />
                          </>
                        ) : (
                          <Typography variant="caption" color="text.secondary">
                            —
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {item.acknowledgedBy?.name ?? "—"}
                        </Typography>
                        {item.acknowledgedOn ? (
                          <Typography variant="caption" color="text.secondary">
                            {formatDate(item.acknowledgedOn)}
                          </Typography>
                        ) : null}
                      </TableCell>
                      {mine ? null : <TableCell>{item.owner?.name ?? "—"}</TableCell>}
                      <TableCell>{formatDate(item.registeredOn)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>

            {data.items.length === 0 ? (
              <Box p={2}>
                <EmptyState
                  message={
                    mine
                      ? "Nothing waiting on your customers. Turn off “Only mine” to see the team's."
                      : "No pairings match these filters"
                  }
                />
              </Box>
            ) : null}
          </Card>
        </Stack>
      ) : null}
    </Box>
  );
}

/** What this pairing is waiting for, in the words an engineer would use. */
function NeedsCell({ item }: { item: WorkQueueItem }) {
  if (item.reason === "IN_PROGRESS") {
    return (
      <Stack spacing={0.25}>
        <Typography variant="body2">{item.nextTaskName ?? "In progress"}</Typography>
        <Typography variant="caption" color="text.secondary">
          {item.playbookName}
        </Typography>
      </Stack>
    );
  }
  if (item.reason === "NOT_STARTED") {
    return (
      <Stack direction="row" spacing={1} alignItems="center">
        <Chip size="small" variant="outlined" label="Not started" />
        <Typography variant="caption" color="text.secondary">
          {item.playbookName ?? `${item.runTotal} attached`}
        </Typography>
      </Stack>
    );
  }
  return (
    <Tooltip
      title={
        item.availablePlaybooks > 0
          ? "Playbooks are available at this stage but none is open"
          : "No playbook has been authored for this platform at this stage"
      }
    >
      <Stack direction="row" spacing={1} alignItems="center">
        <Chip size="small" color="warning" label="No playbook" />
        <Typography variant="caption" color="text.secondary">
          {item.availablePlaybooks > 0
            ? `${item.availablePlaybooks} available`
            : "none authored"}
        </Typography>
      </Stack>
    </Tooltip>
  );
}
