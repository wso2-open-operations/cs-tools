import { Grid, Stack, useTheme } from "@wso2/oxygen-ui";
import type { JSX } from "react";

import type { CountByLabel, DashboardAnalytics } from "@features/plg/api/types";
import { CountBarChart, DonutChart, MultiLineChart, StackedBarChart } from "@features/plg/components/charts";
import { LoadingBlock, SectionCard } from "@features/plg/components/common";
import { humanizeEnum } from "@features/plg/utils/format";

/** The labels the API uses for "the source never said". */
const PLACEHOLDERS = new Set(["NOT_RECORDED", "NOT_CLASSIFIED", "NOT_ASSESSED", "OTHER"]);

/** The email classes, always listed in this order. */
const EMAIL_CLASSES = ["CORPORATE", "PERSONAL", "DISPOSABLE", "INTERNAL"];

/** UNKNOWN is "could not tell" (retried by the job), not a class, so it is left out. */
function emailClasses(rows: CountByLabel[]): CountByLabel[] {
  const counts = new Map(rows.map((r) => [r.label, r.count]));
  const rest = rows.filter((r) => !EMAIL_CLASSES.includes(r.label) && r.label !== "UNKNOWN");
  return [...EMAIL_CLASSES.map((label) => ({ label, count: counts.get(label) ?? 0 })), ...rest];
}

/** Reads a placeholder (or the folded OTHER) as words; a real value (a UTM source) stays as is. */
function readable<T extends { label: string }>(rows: T[]): T[] {
  return rows.map((r) => (PLACEHOLDERS.has(r.label) ? { ...r, label: humanizeEnum(r.label) } : r));
}

/**
 * The PLG overview's charts. `data` is optional so each card shows its own
 * loading block. Qualified means the validation's eligibility outcome; a
 * registration not yet assessed shows as such so the totals add up.
 */
export function DashboardCharts({ data }: { data?: DashboardAnalytics }): JSX.Element {
  const theme = useTheme();
  const segments = [
    { key: "eligible", label: "Eligible", color: theme.palette.success.main },
    { key: "monitor", label: "Monitor", color: theme.palette.warning.main },
    { key: "excluded", label: "Excluded", color: theme.palette.error.main },
    { key: "notAssessed", label: "Not assessed", color: theme.palette.grey[500] },
  ];

  return (
    <Stack spacing={2}>
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 8 }}>
          <SectionCard fill title="Registration Trends">
            {data ? <MultiLineChart data={data.registrationsOverTime} /> : <LoadingBlock height={280} />}
          </SectionCard>
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <SectionCard fill title="Registration Qualification Overview">
            {data ? <DonutChart data={data.qualificationOverview} humanize /> : <LoadingBlock />}
          </SectionCard>
        </Grid>
      </Grid>

      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 7 }}>
          <SectionCard fill title="Registrations by Entry Product">
            {data ? <CountBarChart data={data.registrationsByEntryProduct} /> : <LoadingBlock />}
          </SectionCard>
        </Grid>
        <Grid size={{ xs: 12, md: 5 }}>
          <SectionCard fill title="Email Classification by Registration">
            {data ? <DonutChart data={emailClasses(data.emailClassification)} humanize keepEmptyLabels /> : <LoadingBlock />}
          </SectionCard>
        </Grid>
      </Grid>

      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 5 }}>
          <SectionCard fill title="UTM Sources of the Registrations">
            {data ? <DonutChart data={readable(data.utmSources)} solid /> : <LoadingBlock />}
          </SectionCard>
        </Grid>
        <Grid size={{ xs: 12, md: 7 }}>
          <SectionCard fill title="Qualified Registrations with UTM">
            {data ? (
              <StackedBarChart rows={readable(data.qualifiedByUtm)} segments={segments} />
            ) : (
              <LoadingBlock />
            )}
          </SectionCard>
        </Grid>
      </Grid>
    </Stack>
  );
}
