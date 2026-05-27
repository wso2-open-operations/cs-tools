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
import type { ReactNode } from "react";

import { Link } from "react-router-dom";

import { Card, pxToRem, Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import { Calendar, ChevronRight, Clock4 } from "@wso2/oxygen-ui-icons-react";

import { useDateTime } from "@shared/hooks/useDateTime";

import { Dot } from "@shared/components/ui";

/* eslint-disable react-refresh/only-export-components */

function Root({ to, children }: { to: string; children: ReactNode }) {
  return (
    <Card component={Link} to={to} sx={{ textDecoration: "none", p: 1 }}>
      <Stack gap={0.8}>{children}</Stack>
    </Card>
  );
}

function Header({
  icon: Icon,
  iconColor,
  number,
  internalId,
}: {
  icon: React.ElementType;
  iconColor: string;
  number: string;
  internalId?: string;
}) {
  const theme = useTheme();

  return (
    <Stack direction="row" justifyContent="space-between" alignItems="center">
      <Stack direction="row" alignItems="center" gap={1}>
        <Icon size={pxToRem(18)} color={iconColor} />
        <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
          {internalId && (
            <>
              {internalId}
              <span style={{ opacity: 0.5, margin: "0 4px" }}>|</span>
            </>
          )}
          {number}
        </Typography>
      </Stack>
      <ChevronRight size={pxToRem(18)} color={theme.palette.text.secondary} />
    </Stack>
  );
}

function Title({ children }: { children: ReactNode }) {
  return (
    <Typography variant="body1" color="text.primary" mr={1} noWrap>
      {children}
    </Typography>
  );
}

function Meta({ label, suffix, children }: { label: string; suffix?: string; children?: ReactNode }) {
  return (
    <Stack direction="row" alignItems="center" gap={1}>
      {children && (
        <>
          {children}
          <Dot />
        </>
      )}
      <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
        {label}
      </Typography>
      {suffix && (
        <>
          <Dot />
          <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
            {suffix}
          </Typography>
        </>
      )}
    </Stack>
  );
}

function ScheduledDate({ date }: { date: Date | undefined }) {
  const theme = useTheme();
  const formatted =
    date?.toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
    }) ?? "N/A";

  return (
    <Stack direction="row" alignItems="center" gap={1}>
      <Calendar size={pxToRem(16)} color={theme.palette.text.secondary} />
      <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
        Scheduled: {formatted}
      </Typography>
    </Stack>
  );
}

function Footer({ timestamp, children }: { timestamp: Date; children?: ReactNode }) {
  const theme = useTheme();
  const { fromNow } = useDateTime();
  console.log("timestamp:", timestamp);
  console.log("timestamp from now:", fromNow(timestamp));

  return (
    <Stack gap={0.5} mt={1} direction="row" justifyContent="space-between">
      <Stack direction="row" alignItems="center" gap={1}>
        <Clock4 size={pxToRem(13)} color={theme.palette.text.secondary} />
        <Typography
          fontWeight="regular"
          color="text.tertiary"
          sx={(theme) => ({ fontSize: theme.typography.pxToRem(13) })}
        >
          {fromNow(timestamp)}
        </Typography>
      </Stack>
      {children}
    </Stack>
  );
}

export const ItemCard = { Root, Header, Title, Meta, ScheduledDate, Footer };
