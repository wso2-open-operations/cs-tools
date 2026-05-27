/* eslint-disable react-refresh/only-export-components */
import type { ReactNode } from "react";

import { Link } from "react-router-dom";

import { Box, Card, CardActionArea, Divider, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
import { Calendar, ChevronRight } from "@wso2/oxygen-ui-icons-react";

import { stripHtmlTags } from "@shared/utils";

function Root({ to, children }: { to: string; children: ReactNode }) {
  return (
    <Card sx={{ textDecoration: "none", mb: 2 }}>
      <CardActionArea component={Link} to={to}>
        <Stack bgcolor="background.paper" p={2} gap={1}>
          {children}
        </Stack>
      </CardActionArea>
    </Card>
  );
}

function Header({
  icon: Icon,
  iconColor,
  number,
  internalId,
  chips,
  status,
}: {
  icon: React.ElementType;
  iconColor: string;
  number: string;
  internalId?: string;
  chips?: ReactNode;
  status?: ReactNode;
}) {
  return (
    <Stack direction="row" justifyContent="space-between" gap={3}>
      <Stack direction="row" alignItems="center" gap={1} sx={{ flex: 1, minWidth: 0 }}>
        <Icon size={20} color={iconColor} style={{ flexShrink: 0 }} />
        <Typography noWrap variant="subtitle2" color="text.secondary">
          {internalId && (
            <>
              {internalId}
              <span style={{ opacity: 0.5, margin: "0 4px" }}>|</span>
            </>
          )}
          {number}
        </Typography>
        {chips}
      </Stack>
      <Stack direction="row" gap={2} alignItems="center">
        {status}
        <Box color="text.secondary">
          <ChevronRight size={pxToRem(18)} />
        </Box>
      </Stack>
    </Stack>
  );
}

function Body({ title, description }: { title: string; description?: string }) {
  return (
    <Stack gap={0.2}>
      <Typography variant="body1" color="text.primary" noWrap>
        {title}
      </Typography>
      {description && (
        <Typography
          variant="subtitle2"
          color="text.secondary"
          sx={{
            display: "-webkit-box",
            WebkitLineClamp: 2,
            WebkitBoxOrient: "vertical",
            overflow: "hidden",
          }}
        >
          {stripHtmlTags(description)}
        </Typography>
      )}
    </Stack>
  );
}

function ScheduledDate({ date }: { date: Date | undefined }) {
  const formatted =
    date?.toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
    }) ?? "N/A";

  return (
    <Stack direction="row" alignItems="center" gap={1}>
      <Box sx={{ color: "text.secondary" }}>
        <Calendar size={pxToRem(16)} />
      </Box>
      <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
        Scheduled: {formatted}
      </Typography>
    </Stack>
  );
}

function MetaField({ label, children }: { label?: string; children?: ReactNode }) {
  return (
    <Stack>
      {label && (
        <Typography variant="caption" color="text.secondary">
          {label}
        </Typography>
      )}
      <Typography variant="caption">{children}</Typography>
    </Stack>
  );
}

function Footer({ timestamp, fields = [] }: { timestamp: string; fields?: { label?: string; value: ReactNode }[] }) {
  return (
    <>
      <Divider />
      <Stack direction="row" justifyContent="space-between" alignItems="center" gap={5}>
        <Stack direction="row" alignItems="center" gap={3}>
          {fields?.map(({ label, value }) => (
            <MetaField key={label} label={label}>
              {value}
            </MetaField>
          ))}
        </Stack>
        <Typography variant="caption" color="text.secondary">
          {timestamp}
        </Typography>
      </Stack>
    </>
  );
}

export const ItemCard = { Root, Header, Body, ScheduledDate, MetaField, Footer };
