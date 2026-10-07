import {
  Alert,
  Box,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Skeleton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import type { ReactNode } from "react";

import { humanizeEnum } from "@features/plg/utils/format";

/** Page title block with an optional right-hand action area. */
export function PageHeader({
  title,
  subtitle,
  actions,
}: {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
}) {
  return (
    <Stack direction={{ xs: "column", sm: "row" }} justifyContent="space-between" alignItems={{ sm: "center" }} spacing={2} mb={3}>
      <Box>
        <Typography variant="h4">{title}</Typography>
        {subtitle ? (
          <Typography variant="body2" color="text.secondary" mt={0.5}>
            {subtitle}
          </Typography>
        ) : null}
      </Box>
      {actions ? <Box>{actions}</Box> : null}
    </Stack>
  );
}

/** Bordered card with a heading — the standard section container. */
/**
 * A titled card.
 *
 * `fill` makes the card take its parent's full height, and it is OPT-IN because
 * the unconditional `height: "100%"` it replaces clipped content once PLG moved
 * inside csm-portal.
 *
 * Standalone, PLG's pages scrolled the document, so nothing above a card had a
 * definite height and `height: 100%` quietly resolved to `auto`. csm-portal's
 * AppLayout pins the shell to `100dvh` and scrolls an inner column flex box
 * instead, which gives the page a definite height — so the percentage started
 * resolving, and a SectionCard in a vertical `<Stack>` claimed the whole stack
 * as its flex basis. Flexbox then took the shortfall out of its sibling, and
 * because MUI's `Card` is `overflow: hidden`, that sibling clipped rather than
 * scrolled: the work queue's table rendered 240px of rows inside a 150px card
 * with no scrollbar anywhere, measured at a 1440x900 viewport.
 *
 * Pass `fill` only where cards sit side by side in a `<Grid>` row and should
 * share a height. In a vertical stack it is never what you want.
 */
export function SectionCard({
  title,
  action,
  children,
  dense,
  fill,
}: {
  title: string;
  action?: ReactNode;
  children: ReactNode;
  dense?: boolean;
  fill?: boolean;
}) {
  return (
    <Card sx={fill ? { height: "100%" } : undefined}>
      <CardContent sx={{ p: dense ? 2 : 2.5 }}>
        <Stack direction="row" justifyContent="space-between" alignItems="center" mb={1.5}>
          <Typography variant="h6">{title}</Typography>
          {action}
        </Stack>
        {children}
      </CardContent>
    </Card>
  );
}

/** Single headline number. */
export function StatTile({
  label,
  value,
  hint,
  color = "text.primary",
  onClick,
}: {
  label: string;
  value: number | string;
  hint?: string;
  color?: string;
  onClick?: () => void;
}) {
  return (
    <Card
      onClick={onClick}
      sx={{
        cursor: onClick ? "pointer" : "default",
        transition: "border-color 120ms",
        "&:hover": onClick ? { borderColor: "primary.main" } : undefined,
      }}
    >
      <CardContent sx={{ py: 2, px: 2.5, "&:last-child": { pb: 2 } }}>
        <Typography variant="caption" color="text.secondary" sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}>
          {label}
        </Typography>
        <Typography variant="h4" sx={{ mt: 0.5, color }}>
          {value}
        </Typography>
        {hint ? (
          <Typography variant="caption" color="text.secondary">
            {hint}
          </Typography>
        ) : null}
      </CardContent>
    </Card>
  );
}

type ChipColor = "default" | "primary" | "secondary" | "success" | "error" | "info" | "warning";

// A lifecycle stage and a subscription tier are IDENTITY, not judgement, so
// both are drawn in one static accent: the theme's own primary, which follows
// whichever palette the user has chosen rather than pinning a literal hue.
//
// They used to be colour scales — COMMERCIAL green, PLG_CS_ELIGIBLE blue,
// ABANDONED red — which read as a verdict the data does not carry. A customer at
// REGISTRATION is not doing badly and one at COMMERCIAL is not doing well; they
// are at different points. Worse, the same green that said "commercial" also
// said "healthy" two chips away, so the one axis where colour genuinely IS the
// meaning had to compete with two where it was decoration.
//
// Spending one flat accent here leaves green and red available for health,
// below. `primary` and not an sx override of a specific palette path, because
// the named slot is what tracks the active theme — a run status chip reading
// ACTIVE already uses it, and these now match it under every theme.
const STATIC_KINDS = ["lifecycle", "tier"] as const;
const STATIC_COLOR: ChipColor = "primary";

// Health is the one axis where the colour IS the meaning, so it is the only one
// that varies — and now the only green and red on the page.
//
// The same two named slots the health ToggleButtonGroup in ProductTab already
// uses, so the chip reporting a pairing's health and the control that sets it
// are the same green and the same red rather than two approximations of them.
const healthColor: Record<string, ChipColor> = {
  HEALTHY: "success",
  AT_RISK: "error",
};

// Deliberately not green/red. A playbook kind is not good or bad news; only
// the health that selects it is, and using the same two colours here would
// make a Recovery playbook look like a problem rather than the response to one.
const playbookTypeColor: Record<string, ChipColor> = {
  PROGRESSIVE: "primary",
  RECOVERY: "warning",
  SUSTAINING: "info",
};

const runStatusColor: Record<string, ChipColor> = {
  NOT_STARTED: "default",
  ACTIVE: "primary",
  CLOSED: "success",
};

const valueTypeColor: Record<string, ChipColor> = {
  BOOLEAN: "default",
  STRING: "info",
  NUMBER: "secondary",
  CHECKLIST: "warning",
  SINGLE_SELECT: "primary",
};

// Every kind drawn from a named chip colour, which is now all of them but the
// two static ones above.
const palettes: Record<string, Record<string, ChipColor>> = {
  health: healthColor,
  playbookType: playbookTypeColor,
  runStatus: runStatusColor,
  valueType: valueTypeColor,
};

/** Every kind a chip can be drawn for. */
export type ChipKind = (typeof STATIC_KINDS)[number] | keyof typeof palettes;

/** The named chip colour for one kind and value. */
function colorFor(kind: ChipKind, value: string): ChipColor {
  if ((STATIC_KINDS as readonly string[]).includes(kind)) return STATIC_COLOR;
  return palettes[kind]?.[value] ?? "default";
}

/** Enum chip whose colour comes from the named palette. */
export function StatusChip({
  value,
  kind,
  size = "small",
}: {
  value: string | null | undefined;
  kind: ChipKind;
  size?: "small" | "medium";
}) {
  if (!value) return <Chip label="—" size={size} variant="outlined" />;
  const color = colorFor(kind, value);
  return <Chip label={humanizeEnum(value)} size={size} color={color} variant={color === "default" ? "outlined" : "filled"} />;
}

/**
 * A status chip with a word saying what it is a chip of.
 *
 * Three bare chips reading "Activated", "PayG", "Healthy" tell a reader who
 * already knows the model what they mean, and tell everyone else nothing — the
 * values are distinct enough that nothing identifies which axis each one
 * belongs to. Worse, two of them can hold overlapping-sounding values:
 * COMMERCIAL is a stage and PAYG is a tier, and both name money changing
 * hands.
 *
 * The label is deliberately subordinate — small, uppercase, muted — using the
 * same treatment as Field's label, so the app has one way of saying "this is
 * what the thing beside me is". The value keeps the colour and the weight.
 */
export function LabelledChip({
  label,
  value,
  kind,
  size = "small",
}: {
  label: string;
  value: string | null | undefined;
  kind: ChipKind;
  size?: "small" | "medium";
}) {
  return (
    <Stack direction="row" spacing={0.625} alignItems="center">
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4, whiteSpace: "nowrap" }}
      >
        {label}
      </Typography>
      <StatusChip value={value} kind={kind} size={size} />
    </Stack>
  );
}

export function LoadingBlock({ height = 160 }: { height?: number }) {
  return <Skeleton variant="rounded" height={height} />;
}

export function LoadingSpinner() {
  return (
    <Box display="flex" justifyContent="center" py={6}>
      <CircularProgress />
    </Box>
  );
}

export function ErrorBlock({ error }: { error: unknown }) {
  const message = error instanceof Error ? error.message : "Something went wrong";
  return <Alert severity="error">{message}</Alert>;
}

export function EmptyState({ message }: { message: string }) {
  return (
    <Box py={4} textAlign="center">
      <Typography variant="body2" color="text.secondary">
        {message}
      </Typography>
    </Box>
  );
}

/** Label/value row used across every detail panel. */
export function Field({ label, value }: { label: string; value: ReactNode }) {
  return (
    <Box mb={1.5}>
      <Typography variant="caption" color="text.secondary" sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}>
        {label}
      </Typography>
      <Typography variant="body2" component="div" sx={{ mt: 0.25, wordBreak: "break-word" }}>
        {value ?? "—"}
      </Typography>
    </Box>
  );
}

