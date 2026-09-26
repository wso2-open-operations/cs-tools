import {
  Box,
  Button,
  Chip,
  Grid,
  MenuItem,
  Stack,
  Tab,
  Tabs,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  ArrowLeft as ArrowBackIcon,
} from "@wso2/oxygen-ui-icons-react";
import { useState } from "react";
import { useNavigate, useParams } from "react-router";



import { useCSUsers, useOrganization, useSetOrganizationOwner } from "@features/plg/api/hooks";
import type { UserRef } from "@features/plg/api/types";
import {
  EmptyState,
  ErrorBlock,
  Field,
  LoadingSpinner,
  PageHeader,
  SectionCard,
  StatusChip,
} from "@features/plg/components/common";
import { formatDate } from "@features/plg/utils/format";
import { ProductTab } from "./ProductTab";
import { selectLabelProps } from "@features/plg/components/selectLabelProps";
import { AXIS_BUTTON_SX } from "@features/plg/components/controls";

/**
 * One organisation.
 *
 * The tab strip is the shape of the model made visible: Overview holds the few
 * facts that belong to the customer, and every other tab is a pairing — one
 * platform, with its own lifecycle stage, its own playbooks and its own history.
 */
/**
 * The CS owner picker, staged behind a Save.
 *
 * Reassigning moves every pairing under the customer into someone else's work
 * queue, so it is not something to do on the way past a dropdown — picking a
 * name used to write immediately, which made a mis-click a silent handover that
 * the new owner discovered in their queue. The Save button turns a wrong
 * selection into nothing at all.
 */
function OwnerEditor({
  current,
  owners,
  pending,
  onSave,
}: {
  current: string;
  owners: UserRef[];
  pending: boolean;
  onSave: (id: string) => void;
}) {
  const [draft, setDraft] = useState(current);
  const changed = draft !== current;

  return (
    <Stack direction={{ xs: "column", sm: "row" }} spacing={1} alignItems={{ sm: "flex-start" }}>
      <TextField
        select
        {...selectLabelProps(draft)}
        size="small"
        label="PLG CS owner"
        sx={{ minWidth: 280 }}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        disabled={pending}
      >
        {owners.map((u) => (
          <MenuItem key={u.id} value={u.id}>
            {u.name}
          </MenuItem>
        ))}
      </TextField>
      <Button
        variant="contained"
        size="small"
        sx={AXIS_BUTTON_SX}
        disabled={!changed || pending}
        onClick={() => onSave(draft)}
      >
        {pending ? "Saving…" : "Save owner"}
      </Button>
      {changed ? (
        <Button
          variant="text"
          color="inherit"
          size="small"
          sx={AXIS_BUTTON_SX}
          onClick={() => setDraft(current)}
        >
          Cancel
        </Button>
      ) : null}
    </Stack>
  );
}

export default function OrganizationDetailPage() {
  const { organizationId, productCode } = useParams();
  const navigate = useNavigate();
  const { data: org, isPending, error } = useOrganization(organizationId);

  if (error) return <ErrorBlock error={error} />;
  if (isPending || !org) return <LoadingSpinner />;

  const activeTab = productCode ?? "overview";
  const known = org.platformCards.some((c) => c.product.code === activeTab);
  const selected = activeTab === "overview" || known ? activeTab : "overview";

  return (
    <Box sx={{ display: "flex", flexDirection: "column" }}>
      {/* One root element, because csm-portal's AppLayout renders <Outlet /> into a
          column flex box. A fragment made each child of this page its own flex item,
          and MUI Card is overflow:hidden — so flexbox shrank the filter card and it
          clipped instead of the page scrolling. */}
      <Button startIcon={<ArrowBackIcon />} onClick={() => navigate("/plg/organizations")} sx={{ mb: 1 }}>
        All organisations
      </Button>

      <PageHeader
        title={org.organizationName}
        subtitle={`Registered ${formatDate(org.createdOn)} by ${org.registeredName ?? org.registeredEmail}`}
      />

      <Box sx={{ borderBottom: 1, borderColor: "divider", mb: 2 }}>
        <Tabs
          value={selected}
          onChange={(_, value: string) =>
            navigate(value === "overview" ? `/plg/organizations/${org.id}` : `/plg/organizations/${org.id}/${value}`)
          }
          variant="scrollable"
          scrollButtons="auto"
        >
          <Tab value="overview" label="Overview" />
          {org.platformCards.map((card) => (
            <Tab
              key={card.product.code}
              value={card.product.code}
              label={
                <Stack direction="row" spacing={0.75} alignItems="center">
                  <span>{card.product.name}</span>
                  {card.isNew ? <Chip size="small" color="warning" label="New" /> : null}
                </Stack>
              }
            />
          ))}
        </Tabs>
      </Box>

      {selected === "overview" ? (
        <OverviewTab organizationId={org.id} />
      ) : (
        // Keyed on the product so switching tabs remounts the editors below it:
        // the stage picker, the use-case box and the note draft all hold local
        // state that belongs to one pairing, not to the tab strip.
        <ProductTab key={selected} organizationId={org.id} productCode={selected} />
      )}
    </Box>
  );
}

/**
 * The overview: two cards and nothing else.
 *
 * The customer-level record is deliberately thin. Everything an engineer works
 * on — the stage, the playbooks, the notes — belongs to a pairing, so it lives
 * on a product tab rather than here.
 */
function OverviewTab({ organizationId }: { organizationId: string }) {
  const navigate = useNavigate();
  const { data: org } = useOrganization(organizationId);
  const { data: owners } = useCSUsers();
  const setOwner = useSetOrganizationOwner(organizationId);

  if (!org) return <LoadingSpinner />;

  return (
    <Grid container spacing={2}>
      <Grid size={{ xs: 12, md: 5 }}>
        <SectionCard fill title="Registration details">
          <Field label="Registered email" value={org.registeredEmail} />
          <Field label="Registered by" value={org.registeredName ?? "—"} />
          <Field label="Registered on" value={formatDate(org.createdOn)} />
          <Field label="Company name from domain" value={org.companyNameFromDomain ?? "—"} />
          <Field label="Country" value={org.countryName ?? "—"} />
          <Field label="Company id" value={org.moesifCompanyId ?? "—"} />

          {org.fieldsInherited ? (
            <Tooltip title="The source sent only the mandatory fields for this organisation. The rest are shown from the same person's earlier registration and are not stored here.">
              <Chip
                size="small"
                color="info"
                variant="outlined"
                label="Some details resolved from an earlier registration"
              />
            </Tooltip>
          ) : null}
        </SectionCard>
      </Grid>

      <Grid size={{ xs: 12, md: 7 }}>
        <Stack spacing={2}>
          <SectionCard title="Ownership">
            <Typography variant="body2" color="text.secondary" mb={1.5}>
              One PLG CS engineer owns the customer and every platform under it. Reassigning moves
              all of its pairings to the new owner's work queue at once — and ownership always names
              a person, so there is no way back to unassigned.
            </Typography>
            {/* Keyed on the saved owner so the draft resets once a save lands,
                and so switching organisation cannot carry a pending choice over. */}
            <OwnerEditor
              key={org.owner?.id ?? ""}
              current={org.owner?.id ?? ""}
              owners={owners ?? []}
              pending={setOwner.isPending}
              onSave={(id) => setOwner.mutate(id || null)}
            />
            {setOwner.error ? <ErrorBlock error={setOwner.error} /> : null}
          </SectionCard>

          <SectionCard title="Registered platforms">
            {org.platformCards.length === 0 ? (
              <EmptyState message="No platforms registered yet" />
            ) : (
              <Stack spacing={1}>
                {org.platformCards.map((card) => (
                  <Box
                    key={card.orgPlatformId}
                    onClick={() => navigate(`/plg/organizations/${org.id}/${card.product.code}`)}
                    sx={{
                      p: 1.5,
                      border: "1px solid",
                      borderColor: "divider",
                      borderRadius: 2,
                      cursor: "pointer",
                      "&:hover": { borderColor: "primary.main" },
                    }}
                  >
                    <Stack direction="row" justifyContent="space-between" alignItems="center" spacing={1}>
                      <Box>
                        <Stack direction="row" spacing={1} alignItems="center">
                          <Typography variant="body2" fontWeight={600}>
                            {card.product.name}
                          </Typography>
                          {card.isNew ? <Chip size="small" color="warning" label="New" /> : null}
                        </Stack>
                        <Typography variant="caption" color="text.secondary">
                          Registered {formatDate(card.registeredOn)} · {card.runActive} of {card.runTotal} playbooks
                          open · {card.taskCompleted}/{card.taskTotal} tasks done
                        </Typography>
                      </Box>
                      <Stack direction="row" spacing={0.75} alignItems="center">
                        <StatusChip value={card.subscriptionTier} kind="tier" />
                        <StatusChip value={card.lifecycleStage} kind="lifecycle" />
                      </Stack>
                    </Stack>
                  </Box>
                ))}
              </Stack>
            )}
          </SectionCard>
        </Stack>
      </Grid>
    </Grid>
  );
}
