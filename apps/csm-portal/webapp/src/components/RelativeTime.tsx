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

import { Box, Button, Tooltip } from "@wso2/oxygen-ui";
import { Check, Link2 } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useLayoutEffect, useRef, useState, type JSX } from "react";
import { formatRelativeTime } from "@features/csm-dashboard/utils/abtDashboard";
import { formatAbsoluteForUser, parseBackendTimestamp } from "@utils/dateTime";

/**
 * The granularity `formatRelativeTime` displays at, by how far `abs` (the
 * distance between the tracked timestamp and "now") currently is. Mirrors
 * that function's own bucketing (`diffMin` while `abs < 1h`, `diffHr` while
 * `abs < 24h`, `diffDay` beyond that) so the scheduler wakes up exactly when
 * the displayed text would actually change, not on a fixed cadence.
 */
const MINUTE_MS = 60_000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;

function currentGranularityMs(absMs: number): number {
  if (absMs < HOUR_MS) return MINUTE_MS;
  if (absMs < DAY_MS) return HOUR_MS;
  return DAY_MS;
}

/**
 * How long from `nowMs` until the text for a timestamp at `timestampMs`
 * would next change. Handles both directions `formatRelativeTime` supports:
 *
 * - Past ("Xm ago"): `abs` grows as real time passes, so the next change is
 *   when `abs` reaches the *next* multiple of the current granularity.
 * - Future ("Xm from now"): `abs` shrinks as real time passes (converging on
 *   the timestamp), so the next change is when `abs` drops to the *current*
 *   (lower) multiple of the granularity.
 *
 * Clamped to a 1s floor purely to avoid a zero-delay `setTimeout` re-firing
 * immediately due to integer rounding at an exact boundary — sub-second
 * precision isn't meaningful here since the coarsest display unit is a
 * minute.
 */
function nextChangeDelayMs(timestampMs: number, nowMs: number): number {
  const diffMs = nowMs - timestampMs;
  const absMs = Math.abs(diffMs);
  const granularity = currentGranularityMs(absMs);
  const bucket = Math.floor(absMs / granularity);
  const delay =
    diffMs >= 0
      ? (bucket + 1) * granularity - absMs // past: wait for abs to grow into the next bucket
      : absMs - bucket * granularity; // future: wait for abs to shrink out of this bucket
  return Math.max(delay, 1000);
}

type TickListener = () => void;

interface Registration {
  timestampMs: number;
  listener: TickListener;
}

/** Module-level registry backing a single shared, adaptively-scheduled
 * timer, so a page with many relative timestamps mounted at once (e.g. a
 * `CasesList` table, or a long comment thread) re-renders off one timer
 * instead of one per instance — and that one timer only wakes when some
 * registered timestamp's displayed text would actually change, instead of
 * polling on a fixed cadence. Keyed by a per-hook-instance object identity
 * rather than the timestamp value, since two mounted instances can track
 * the same timestamp. */
const registrations = new Map<object, Registration>();
let timeoutId: ReturnType<typeof setTimeout> | null = null;

function fireTick(): void {
  timeoutId = null;
  registrations.forEach(({ listener }) => listener());
  reschedule();
}

/** Recomputes the minimum "next change" delay across every currently
 * registered timestamp and (re)schedules a single `setTimeout` for it.
 * Always clears any existing timer first — called both when the timer
 * itself fires and whenever the registered set changes (mount, unmount, or
 * a tracked timestamp changing), so the scheduled delay never goes stale. */
function reschedule(): void {
  if (timeoutId !== null) {
    clearTimeout(timeoutId);
    timeoutId = null;
  }
  if (registrations.size === 0) return;
  const nowMs = Date.now();
  let minDelay: number | null = null;
  registrations.forEach(({ timestampMs }) => {
    const delay = nextChangeDelayMs(timestampMs, nowMs);
    if (minDelay === null || delay < minDelay) minDelay = delay;
  });
  if (minDelay === null) return;
  timeoutId = setTimeout(fireTick, minDelay);
}

/**
 * Subscribes the calling component to the shared adaptive scheduler and
 * returns the timestamp (ms) as of the last update, so it re-renders — and
 * recomputes whatever it derives from "now" — exactly when its own tracked
 * timestamp's displayed text would change (not on a fixed cadence shared
 * with every other timestamp on the page). Used by {@link RelativeTime}
 * itself, and by the two "Last refreshed …" labels (`CaseSlaTable`,
 * `RefreshButton`) that format a relative string outside this component.
 *
 * `trackedMs` is the epoch-ms of the timestamp this caller displays —
 * falsy (`undefined`/`null`/`0`/`NaN`) means "nothing to display yet"
 * (matches the callers' own `updatedAt ? … : null` guards), so the hook
 * simply doesn't register with the scheduler in that case.
 *
 * Deliberately returns the timestamp itself, not an opaque counter: this
 * webapp runs the React Compiler, which auto-memoizes calls like
 * `formatRelativeTime(iso)` against the reactive values they visibly read.
 * A counter that's merely in scope wouldn't be picked up as a dependency —
 * the caller must pass this value in as the explicit `now` argument (see
 * `formatRelativeTime`'s second parameter) for the compiler to know the
 * result needs recomputing on every update.
 */
// eslint-disable-next-line react-refresh/only-export-components -- hook is colocated with the shared scheduler it manages, and reused by the two other relative-time call sites (fast-refresh DX only)
export function useRelativeTimeTick(trackedMs?: number | null): number {
  const [now, setNow] = useState(() => Date.now());
  const keyRef = useRef<object>({});

  // Corrects `now` the instant `trackedMs` changes to an actual tracked
  // value, via `useLayoutEffect` rather than the registration `useEffect`
  // below — layout effects flush synchronously before the browser paints,
  // so this `setNow` lands before anything is visible to the user, with no
  // separate committed frame showing the stale value in between. Without
  // this, a component whose `now` state went stale while mounted (nothing
  // tracked, or its previous tracked timestamp hadn't ticked yet) would
  // compare a freshly-set `trackedMs` (e.g. "right now" from a refresh
  // click) against that stale `now`, producing a transient negative diff
  // ("Xm from now") until the next scheduled tick self-corrects it.
  //
  // Gated the same way the registration effect below is (nothing to do
  // when `trackedMs` is absent) — an earlier version of this ran
  // unconditionally on every `trackedMs` change including "became
  // untracked," which set state (and forced an extra render) on every
  // mount even for a component with nothing tracked at all. That extra
  // render happened to run before `useElementVisibleOnce`'s own mount
  // effect (see that hook's `[ref.current]` dependency) captured a stable
  // `ref.current`, so it re-observed a second time — caught by
  // `DashboardWidgetGrid.realisticViewport.test.tsx` asserting exactly one
  // `IntersectionObserver` instance per widget, not this task's own
  // authored test.
  //
  // React's own docs describe adjusting state synchronously during the
  // render body itself for this exact "correct state when a prop changes"
  // shape (https://react.dev/learn/you-might-not-need-an-effect#adjusting-some-state-when-a-prop-changes),
  // which would avoid this hook needing an effect for the correction at
  // all — but this app runs the React Compiler with its purity rule
  // enforced (`react-hooks/purity`), which rejects `Date.now()` (impure)
  // called from a render body outright, so that specific shape isn't
  // available here; a layout effect is the closest equivalent this
  // codebase's lint config allows.
  useLayoutEffect(() => {
    if (trackedMs == null || !Number.isFinite(trackedMs)) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect -- syncs `now` to the external shared-scheduler registry the instant this instance's tracked timestamp changes, so it's never rendered against a stale `now` left over from before this timestamp existed; see this effect's own comment above for why it's a layout effect specifically
    setNow(Date.now());
  }, [trackedMs]);

  useEffect(() => {
    const key = keyRef.current;
    if (trackedMs == null || !Number.isFinite(trackedMs)) {
      registrations.delete(key);
      reschedule();
      return;
    }
    registrations.set(key, {
      timestampMs: trackedMs,
      listener: () => setNow(Date.now()),
    });
    reschedule();
    return () => {
      registrations.delete(key);
      reschedule();
    };
  }, [trackedMs]);

  return now;
}

interface RelativeTimeProps {
  /** Backend timestamp string (assumed UTC if no zone is present). */
  iso: string | null | undefined;
  /**
   * Optional permalink target. When provided, the time renders as an anchor
   * (Twitter/Facebook pattern: time = permalink to the entry). May be a hash
   * fragment (e.g. `#cmt-1001-2`) or a route.
   */
  href?: string;
  /** Optional className passthrough for layout tweaks. */
  className?: string;
}

/**
 * Small icon-button that copies the full absolute permalink URL for `href`
 * to the clipboard, so the permalink affordance is discoverable even for
 * someone who doesn't notice the timestamp itself is clickable. Follows the
 * same Copy/Check + 2-second-reset pattern as {@link QueryErrorState}'s
 * tracking-ID copy button, but rests on a chain-link icon to signal
 * "permalink" rather than a generic copy action.
 */
function CopyPermalinkButton({ href }: { href: string }): JSX.Element {
  const [copied, setCopied] = useState(false);

  const handleCopy = (): void => {
    const url = `${window.location.origin}${window.location.pathname}${href}`;
    if (!navigator.clipboard) return;
    navigator.clipboard.writeText(url).then(
      () => {
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      },
      () => {
        // swallow — no toast surface available from this small button
      },
    );
  };

  const label = copied ? "Copied" : "Copy link to this entry";

  return (
    <Tooltip title={label} placement="top">
      <Button
        size="small"
        variant="text"
        color="inherit"
        onClick={handleCopy}
        sx={{ minWidth: 0, p: 0.5, color: "text.disabled" }}
        aria-label={label}
      >
        {copied ? <Check size={13} /> : <Link2 size={13} />}
      </Button>
    </Tooltip>
  );
}

/**
 * Renders a relative timestamp ("7h ago") with the full absolute datetime
 * (in the user's preferred zone) shown on hover. If `href` is provided, the
 * text becomes a permalink to that entry, and a copy-link icon-button follows
 * it so the permalink affordance doesn't rely on someone noticing the
 * timestamp itself is clickable.
 */
export default function RelativeTime({
  iso,
  href,
  className,
}: RelativeTimeProps): JSX.Element {
  // Resolve the epoch this instance tracks, so the shared scheduler can
  // compute exactly when *this* timestamp's display text would next change
  // — mirrors the parsing `formatRelativeTime` itself does below.
  const parsed = iso ? parseBackendTimestamp(iso) : null;
  const trackedMs = parsed ? parsed.getTime() : iso ? new Date(iso).getTime() : null;
  const now = useRelativeTimeTick(Number.isNaN(trackedMs) ? null : trackedMs);
  const relative = formatRelativeTime(iso, now);
  const absolute = formatAbsoluteForUser(iso) ?? "Unknown time";

  const inner = href ? (
    <a
      href={href}
      className={className}
      style={{
        color: "inherit",
        textDecoration: "none",
        whiteSpace: "nowrap",
        cursor: "pointer",
      }}
      onMouseEnter={(e) => {
        (e.currentTarget as HTMLAnchorElement).style.textDecoration =
          "underline";
      }}
      onMouseLeave={(e) => {
        (e.currentTarget as HTMLAnchorElement).style.textDecoration = "none";
      }}
    >
      {relative}
    </a>
  ) : (
    <span className={className} style={{ whiteSpace: "nowrap" }}>
      {relative}
    </span>
  );

  if (!href) {
    return (
      <Tooltip title={absolute} placement="top" arrow>
        {inner}
      </Tooltip>
    );
  }

  return (
    <Box component="span" sx={{ display: "inline-flex", alignItems: "center", gap: 0.25 }}>
      <Tooltip title={absolute} placement="top" arrow>
        {inner}
      </Tooltip>
      <CopyPermalinkButton href={href} />
    </Box>
  );
}
