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
  Chip,
  Link,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { Link as RouterLink } from "react-router";
import {
  MATRIX_SEVERITIES,
  MATRIX_STATES,
  useCaseCountsMatrix,
} from "@features/csm-dashboard/api/useCaseCountsMatrix";
import {
  SEVERITY_COLOR,
  STATE_LABEL,
} from "@features/csm-dashboard/utils/abtDashboard";
import type {
  CaseState,
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";
import SectionCard from "@features/csm-dashboard/components/SectionCard";

/** Build a `/cases?...` href for the given severity and/or state filter. */
function casesHref(severity?: Severity, state?: CaseState): string {
  const params = new URLSearchParams();
  if (severity) params.set("severities", severity);
  if (state) params.set("states", state);
  const qs = params.toString();
  return qs ? `/cases?${qs}` : "/cases";
}

/** A count rendered as a link when > 0, muted "0" otherwise. */
function CountCell({
  value,
  href,
  bold = false,
}: {
  value: number;
  href: string;
  bold?: boolean;
}): JSX.Element {
  return (
    <TableCell align="center" sx={{ fontWeight: bold ? 700 : 400 }}>
      {value > 0 ? (
        <Link
          component={RouterLink}
          to={href}
          underline="hover"
          color="inherit"
          sx={{ fontWeight: "inherit" }}
        >
          {value}
        </Link>
      ) : (
        <Typography component="span" variant="body2" color="text.disabled">
          0
        </Typography>
      )}
    </TableCell>
  );
}

export default function CaseCountsMatrix(): JSX.Element {
  const { data, isLoading, isError } = useCaseCountsMatrix();

  return (
    <SectionCard title="Cases by severity and state">
      {isLoading ? (
        <Skeleton variant="rounded" height={220} />
      ) : isError || !data ? (
        <Typography variant="body2" color="text.secondary">
          Could not load case counts.
        </Typography>
      ) : (
        <TableContainer>
          <Table size="small" aria-label="Case counts by severity and state">
            <TableHead>
              <TableRow>
                <TableCell sx={{ fontWeight: 600 }}>Severity \ State</TableCell>
                {MATRIX_STATES.map((st) => (
                  <TableCell key={st} align="center">
                    <Link
                      component={RouterLink}
                      to={casesHref(undefined, st)}
                      underline="hover"
                      color="text.secondary"
                      sx={{ fontWeight: 600, fontSize: "0.78rem" }}
                    >
                      {STATE_LABEL[st]}
                    </Link>
                  </TableCell>
                ))}
                <TableCell align="center" sx={{ fontWeight: 700 }}>
                  Total
                </TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {MATRIX_SEVERITIES.map((sev) => (
                <TableRow key={sev} hover>
                  <TableCell>
                    <Link
                      component={RouterLink}
                      to={casesHref(sev)}
                      underline="none"
                    >
                      <Chip
                        size="small"
                        label={sev}
                        color={SEVERITY_COLOR[sev]}
                        sx={{ cursor: "pointer" }}
                      />
                    </Link>
                  </TableCell>
                  {MATRIX_STATES.map((st) => (
                    <CountCell
                      key={st}
                      value={data.counts[sev][st]}
                      href={casesHref(sev, st)}
                    />
                  ))}
                  <CountCell
                    value={data.severityTotals[sev]}
                    href={casesHref(sev)}
                    bold
                  />
                </TableRow>
              ))}
              <TableRow>
                <TableCell sx={{ fontWeight: 700 }}>Total</TableCell>
                {MATRIX_STATES.map((st) => (
                  <CountCell
                    key={st}
                    value={data.stateTotals[st]}
                    href={casesHref(undefined, st)}
                    bold
                  />
                ))}
                <CountCell value={data.total} href="/cases" bold />
              </TableRow>
            </TableBody>
          </Table>
          {data.truncated && (
            <Box sx={{ mt: 1 }}>
              <Typography variant="caption" color="text.secondary">
                Sampled the first {data.total} cases; totals may be higher.
              </Typography>
            </Box>
          )}
        </TableContainer>
      )}
    </SectionCard>
  );
}
