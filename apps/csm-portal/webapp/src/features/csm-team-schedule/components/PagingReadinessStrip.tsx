/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useState, type JSX } from "react";
import type { PagingReadinessChain, PagingReadinessGap } from "../types";
import { ADD_TO_PROFILE_HINT } from "../utils/pagingPhone";
import { InlineSkeleton } from "./ScheduleSkeleton";

/** An inline action on a phone gap, where the reader may act on that person. */
export interface PhoneGapAction {
  label: string;
  onClick: () => void;
  /** A test call already placed: shown, not clickable again. */
  pending?: boolean;
}

/** How many gaps show before "Show all". */
export const READINESS_COLLAPSED_GAPS = 6;

interface Props {
  /** The chain's readiness; absent while loading, on a failed read, or when
   *  the check does not cover this chain yet. */
  chain?: PagingReadinessChain;
  /** The label to use when there is no chain to take one from. */
  label: string;
  days: number;
  isLoading: boolean;
  isError: boolean;
  /** A responders gap: open that team's row for editing. Offered only where
   *  the responders are set on this tab (CRE). */
  onFixResponders?: (teamKey?: string) => void;
  /** A rota gap: open the week view on the rota (and day) it is in. */
  onOpenRota?: (gap: PagingReadinessGap) => void;
  /** A phone gap's action -- "Add number" or "Test call" -- where the reader
   *  may edit that person's paging number; null keeps the profile hint. */
  phoneActionFor?: (gap: PagingReadinessGap) => PhoneGapAction | null;
}

/** Errors before warnings, each keeping the server's order. */
function ordered(gaps: readonly PagingReadinessGap[]): PagingReadinessGap[] {
  return [...gaps.filter((g) => g.severity === "error"), ...gaps.filter((g) => g.severity !== "error")];
}

/**
 * Whether a alert chain has somebody to page on every day ahead, and what is
 * missing where it does not. Read-only: each gap names the one thing that
 * closes it -- an edit on this tab, a day on the rota, a profile, or an admin.
 *
 * Status is in the words (ready / gaps, Error / Warning), never the colour
 * alone.
 */
export default function PagingReadinessStrip({
  chain,
  label,
  days,
  isLoading,
  isError,
  onFixResponders,
  onOpenRota,
  phoneActionFor,
}: Props): JSX.Element {
  const [showAll, setShowAll] = useState(false);
  const name = chain?.label || label;

  if (isLoading) {
    return (
      <section className="cp-ready" aria-label={`${name} alert readiness`}>
        <p className="cp-ready-head">
          <InlineSkeleton label={`Checking the ${name} alert chain…`} w="16em" />
        </p>
      </section>
    );
  }
  if (isError || !chain) {
    return (
      <section className="cp-ready" aria-label={`${name} alert readiness`}>
        <p className="cp-ready-head">
          {isError
            ? `Could not check whether the ${name} alert chain is ready.`
            : `No readiness check for the ${name} alert chain yet.`}
        </p>
      </section>
    );
  }

  const gaps = ordered(chain.gaps ?? []);
  const errors = gaps.filter((g) => g.severity === "error").length;
  const warnings = gaps.length - errors;
  const shown = showAll ? gaps : gaps.slice(0, READINESS_COLLAPSED_GAPS);

  const head = chain.ready
    ? `✓ ready for the next ${days} days${warnings > 0 ? ` · ${warnings} warning${warnings === 1 ? "" : "s"}` : ""}`
    : gaps.length > 0
      ? `⚠ ${gaps.length} gap${gaps.length === 1 ? "" : "s"}`
      : "⚠ not ready";

  const action = (g: PagingReadinessGap): JSX.Element | null => {
    switch (g.fix) {
      case "responders":
        return onFixResponders ? (
          <button type="button" className="btn ghost sm" onClick={() => onFixResponders(g.teamKey)}>
            Set responders
          </button>
        ) : null;
      case "rota":
        return onOpenRota ? (
          <button type="button" className="btn ghost sm" onClick={() => onOpenRota(g)}>
            Open the rota{g.date ? ` on ${fmtDay(g.date)}` : ""}
          </button>
        ) : null;
      case "profile": {
        const phone = g.userId ? phoneActionFor?.(g) : null;
        return phone ? (
          <button
            type="button"
            className="btn ghost sm"
            aria-disabled={phone.pending || undefined}
            onClick={() => {
              if (!phone.pending) phone.onClick();
            }}
          >
            {phone.label}
          </button>
        ) : (
          <span className="cp-fixnote">{ADD_TO_PROFILE_HINT}</span>
        );
      }
      default:
        return <span className="cp-fixnote">Needs an admin</span>;
    }
  };

  return (
    <section className={`cp-ready ${chain.ready ? "ok" : "gap"}`} aria-label={`${name} alert readiness`}>
      <p className="cp-ready-head">
        <b>{name}</b> — {head}
      </p>
      {gaps.length > 0 ? (
        <ul className="cp-gaps" aria-label={`${name} gaps`}>
          {shown.map((g, i) => (
            <li key={`${g.code}-${i}`} className={`cp-gap ${g.severity}`}>
              <span className="cp-sev">{g.severity === "error" ? "Error" : "Warning"}</span>
              <span className="cp-msg">{g.message}</span>
              <span className="cp-fix">{action(g)}</span>
            </li>
          ))}
        </ul>
      ) : null}
      {gaps.length > READINESS_COLLAPSED_GAPS ? (
        <button
          type="button"
          className="btn ghost sm cp-more"
          aria-expanded={showAll}
          onClick={() => setShowAll((v) => !v)}
        >
          {showAll ? "Show fewer" : `Show all ${gaps.length}`}
        </button>
      ) : null}
    </section>
  );
}

/** "Sat 10 Oct" for a YYYY-MM-DD day, read as local midnight. */
function fmtDay(iso: string): string {
  const [y, m, d] = iso.split("-").map(Number);
  if (!y || !m || !d) return iso;
  return new Date(y, m - 1, d).toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short" });
}
