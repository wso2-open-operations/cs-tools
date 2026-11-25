import { Box } from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";

import { useDashboard } from "@features/plg/api/hooks";
import { DashboardCharts } from "@features/plg/components/DashboardCharts";
import { ErrorBlock, PageHeader } from "@features/plg/components/common";
import { PeriodSelect } from "@features/plg/components/PeriodSelect";
import { DEFAULT_RANGE_DAYS, isoDaysAgo } from "@features/plg/components/period";

/** The PLG overview: the portfolio's charts (DashboardCharts), no tiles or queue counts. */
export default function LeadershipDashboardPage(): JSX.Element {
  const [days, setDays] = useState(DEFAULT_RANGE_DAYS);
  const { data, error } = useDashboard(isoDaysAgo(days));

  if (error) return <ErrorBlock error={error} />;

  return (
    // One root element, because csm-portal's AppLayout renders <Outlet /> into
    // a column flex box: a fragment would make each child its own flex item,
    // and an overflow:hidden card then clips instead of the page scrolling.
    <Box sx={{ display: "flex", flexDirection: "column" }}>
      <PageHeader
        title="PLG Overview"
        subtitle="Registrations, where they come from and which of them qualify"
        // Scopes the cohort charts only.
        actions={<PeriodSelect value={days} onChange={setDays} />}
      />

      <DashboardCharts data={data} />
    </Box>
  );
}
