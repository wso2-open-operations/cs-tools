import {
  Box,
  Grid,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { useState } from "react";
import { useLocation, useNavigate } from "react-router";

import { useDashboard } from "@features/plg/api/hooks";
import { DonutChart, MultiLineChart } from "@features/plg/components/charts";
import { ErrorBlock, LoadingBlock, PageHeader, SectionCard, StatTile } from "@features/plg/components/common";
import { PeriodSelect } from "@features/plg/components/PeriodSelect";
import { DEFAULT_RANGE_DAYS, isoDaysAgo } from "@features/plg/components/period";

/**
 * The dashboard, in two variants from one component.
 *
 * Workspace's answers "what should I do next": it leads with the two queue
 * counts, and every tile is a link into the work.
 *
 * Overview's is the LEADERSHIP view. It drops those same two tiles, because
 * they are a to-do list and a leadership dashboard is not one — "12 awaiting
 * acknowledgement" is an instruction to whoever owns the queue and noise to
 * anyone reading the shape of the funnel. What remains is the standing
 * picture: how many customers there are, how many registrations, and the
 * cohort charts underneath.
 *
 * One component rather than two files because the charts, the period control
 * and the loading and error states are identical, and the day the funnel
 * charts change they must change in both. The variant is read from the route
 * rather than passed as a prop so the routing stays the single place that
 * decides which page is which.
 *
 * The period control scopes the cohort charts only. The queue counts — where
 * they are shown at all — ignore it on purpose: a backlog that shrinks because
 * someone changed a dropdown is a backlog nobody clears.
 */
export default function DashboardPage() {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const [days, setDays] = useState(DEFAULT_RANGE_DAYS);
  const { data, isPending, error } = useDashboard(isoDaysAgo(days));

  // endsWith, not startsWith. Standalone this page lived at "/overview"; merged
  // it is "/plg/overview", and a startsWith check silently stopped matching —
  // so the Leadership Dashboard rendered the Workspace variant, queue tiles and
  // all. Matching the last segment survives any future prefix.
  const leadership = pathname.endsWith("/overview");

  if (error) return <ErrorBlock error={error} />;

  return (
    <Box sx={{ display: "flex", flexDirection: "column" }}>
      {/* One root element, because csm-portal's AppLayout renders <Outlet /> into a
          column flex box. A fragment made each child of this page its own flex item,
          and MUI Card is overflow:hidden — so flexbox shrank the filter card and it
          clipped instead of the page scrolling. */}
      <PageHeader
        title={leadership ? "Leadership Dashboard" : "Dashboard"}
        subtitle={
          leadership
            ? "Registrations and lifecycle spread across the whole portfolio"
            : "Registrations, lifecycle spread and the work in flight"
        }
        actions={<PeriodSelect value={days} onChange={setDays} />}
      />

      {isPending || !data ? (
        <LoadingBlock height={120} />
      ) : (
        <Grid container spacing={2} mb={3}>
          <Grid size={{ xs: 12, sm: 6, md: leadership ? 6 : 3 }}>
            <StatTile
              label="Organisations"
              value={data.summary.totalOrganizations}
              hint="Customers registered through PLG"
              onClick={() => navigate("/plg/organizations")}
            />
          </Grid>
          <Grid size={{ xs: 12, sm: 6, md: leadership ? 6 : 3 }}>
            <StatTile
              label="Registrations"
              value={data.summary.totalRegistrations}
              hint="One per organisation and platform"
              onClick={() => navigate("/plg/organizations")}
            />
          </Grid>
          {/* Queue counts — Workspace only. See the component comment: these
              two are a to-do list, and the leadership view is not one. */}
          {leadership ? null : (
            <>
              <Grid size={{ xs: 12, sm: 6, md: 3 }}>
                <StatTile
                  label="Awaiting acknowledgement"
                  value={data.summary.newRegistrations}
                  hint="Nobody has picked these up yet"
                  color={data.summary.newRegistrations > 0 ? "warning.main" : "text.primary"}
                  onClick={() => navigate("/plg/new-registrations")}
                />
              </Grid>
              <Grid size={{ xs: 12, sm: 6, md: 3 }}>
                <StatTile
                  label="Pairings needing attention"
                  value={data.summary.pairingsNeedingAttention}
                  hint="Acknowledged, and not yet at an ending stage"
                  // mine=false, because this counts the whole team's queue. A
                  // tile should land you on a page showing the number it just
                  // showed you.
                  onClick={() => navigate("/plg/work-queue?mine=false")}
                />
              </Grid>
            </>
          )}
        </Grid>
      )}

      <Stack spacing={2}>
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 4 }}>
            <SectionCard fill title="Registrations by platform">
              {data ? <DonutChart data={data.registrationsByProduct} /> : <LoadingBlock />}
            </SectionCard>
          </Grid>
          <Grid size={{ xs: 12, md: 4 }}>
            <SectionCard fill title="Pairings by lifecycle stage">
              {data ? <DonutChart data={data.registrationsByLifecycleStage} humanize /> : <LoadingBlock />}
            </SectionCard>
          </Grid>
          <Grid size={{ xs: 12, md: 4 }}>
            <SectionCard fill
              title="Subscription mix"
              action={
                data && data.summary.trialsEndingSoon > 0 ? (
                  <Typography variant="caption" color="warning.main">
                    {data.summary.trialsEndingSoon} trials ending within 30 days
                  </Typography>
                ) : undefined
              }
            >
              {data ? <DonutChart data={data.subscriptionMix} humanize /> : <LoadingBlock />}
            </SectionCard>
          </Grid>
        </Grid>

        <SectionCard title="Registrations over time">
          {data ? <MultiLineChart data={data.registrationsOverTime} /> : <LoadingBlock height={300} />}
        </SectionCard>
      </Stack>
    </Box>
  );
}
