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

import { type JSX } from "react";

/**
 * What a Team Schedule view looks like before its data arrives.
 *
 * A line of "Loading the rota…" on an empty card said nothing about what was
 * coming, and the whole card jumped when the grid replaced it. Each skeleton
 * is drawn in the shape of the view it stands in for -- the roster's rows of
 * day cells, the week's seven columns, the day's ladder lanes, My week's day
 * cards -- so the page holds still and the reader's eye is already where the
 * content will land.
 *
 * Purely visual: nothing here reads data or changes when a view loads. The
 * message is still said, as a short caption and to a screen reader through
 * role="status", and the shapes themselves are hidden from assistive tech.
 * The skeleton fades in after a moment, so a read that answers at once does
 * not flash one, and it holds still for anyone who asks for reduced motion.
 */
export type ScheduleSkeletonVariant = "mine" | "today" | "week" | "roster" | "list";

interface Props {
  variant: ScheduleSkeletonVariant;
  /** What is loading, in words: shown as the caption and announced. */
  label: string;
  /** Rows to draw, where the variant has rows. */
  rows?: number;
}

/** Widths that read as names and labels of different lengths, rather than a
 *  column of identical bars. Fixed, so the skeleton never shifts on render. */
const WIDTHS = [72, 58, 84, 64, 46, 78, 52, 68, 88, 60];
const width = (i: number): string => `${WIDTHS[i % WIDTHS.length]}%`;

const range = (n: number): number[] => Array.from({ length: n }, (_, i) => i);

function Bar({ w, h, round }: { w?: string; h?: number; round?: boolean }): JSX.Element {
  return <span className={`sk-bar${round ? " round" : ""}`} style={{ width: w, height: h }} />;
}

function Roster({ rows }: { rows: number }): JSX.Element {
  const days = 31;
  return (
    <div className="sk-roster">
      <div className="sk-rhead">
        <Bar w="60%" h={10} />
        {range(days).map((d) => (
          <span key={d} className="sk-dayhd">
            <Bar w="70%" h={8} />
            <Bar w="45%" h={10} />
          </span>
        ))}
      </div>
      {range(rows).map((r) => (
        <div key={r} className="sk-rrow">
          <span className="sk-who">
            <Bar w="22px" h={22} round />
            <Bar w={width(r)} h={10} />
          </span>
          {range(days).map((d) => (
            // Weekends sit quieter on the real roster; so they do here.
            <span key={d} className={`sk-cell${d % 7 === 5 || d % 7 === 6 ? " we" : ""}`} />
          ))}
        </div>
      ))}
    </div>
  );
}

function Week({ rows }: { rows: number }): JSX.Element {
  return (
    <div className="sk-week">
      <span />
      {range(7).map((d) => (
        <span key={d} className="sk-weekhd">
          <Bar w="55%" h={10} />
        </span>
      ))}
      {range(rows).map((r) => (
        <div key={r} className="sk-weekrow">
          <span className="sk-who">
            <Bar w={width(r + 3)} h={10} />
          </span>
          {range(7).map((d) => (
            <span key={d} className="sk-weekcell">
              <Bar w={width(r + d)} h={18} />
              {(r + d) % 3 === 0 ? <Bar w={width(r + d + 4)} h={18} /> : null}
            </span>
          ))}
        </div>
      ))}
    </div>
  );
}

function Today(): JSX.Element {
  return (
    <div className="sk-ladder">
      {range(4).map((lane) => (
        <div key={lane} className="sk-lane">
          <Bar w="50%" h={12} />
          {range(lane % 2 === 0 ? 4 : 3).map((p) => (
            <span key={p} className="sk-person">
              <Bar w="26px" h={26} round />
              <span className="sk-lines">
                <Bar w={width(lane + p)} h={10} />
                <Bar w={width(lane + p + 5)} h={8} />
              </span>
            </span>
          ))}
        </div>
      ))}
    </div>
  );
}

function Mine(): JSX.Element {
  return (
    <div className="sk-mine">
      <div className="sk-hero">
        <Bar w="18%" h={10} />
        <Bar w="42%" h={18} />
        <Bar w="30%" h={10} />
      </div>
      <div className="sk-days">
        {range(7).map((d) => (
          <span key={d} className="sk-daycard">
            <Bar w="45%" h={10} />
            <Bar w="70%" h={20} />
            <Bar w={width(d)} h={8} />
          </span>
        ))}
      </div>
    </div>
  );
}

function List({ rows }: { rows: number }): JSX.Element {
  return (
    <div className="sk-list">
      {range(rows).map((r) => (
        <span key={r} className="sk-item">
          <Bar w="28px" h={28} round />
          <span className="sk-lines">
            <Bar w={width(r)} h={10} />
            <Bar w={width(r + 4)} h={8} />
          </span>
        </span>
      ))}
    </div>
  );
}

export default function ScheduleSkeleton({ variant, label, rows }: Props): JSX.Element {
  return (
    // No aria-busy here: on a live region it holds the announcement until busy
    // turns false, and this is unmounted before it ever would be.
    <div className={`sk sk-${variant}-wrap`} role="status" aria-live="polite">
      <p className="sk-caption">
        <span className="sk-spin" aria-hidden="true" />
        {label}
      </p>
      <div className="sk-body" aria-hidden="true">
        {variant === "roster" ? (
          <Roster rows={rows ?? 10} />
        ) : variant === "week" ? (
          <Week rows={rows ?? 6} />
        ) : variant === "today" ? (
          <Today />
        ) : variant === "mine" ? (
          <Mine />
        ) : (
          <List rows={rows ?? 4} />
        )}
      </div>
    </div>
  );
}

/** The view tabs while it is not yet known which of them the reader gets:
 *  placeholders the size of the real ones, so the strip is not an empty pill
 *  and the row does not jump when the tabs arrive. */
export function TabStripSkeleton({ count = 5 }: { count?: number }): JSX.Element {
  return (
    <>
      {range(count).map((i) => (
        <span key={i} className="tab sk-tab" aria-hidden="true">
          <Bar w={`${56 + ((i * 13) % 30)}%`} h={11} />
          <Bar w={`${40 + ((i * 17) % 34)}%`} h={8} />
        </span>
      ))}
    </>
  );
}

/** One line of text still loading, inline: a short shimmer where the words
 *  will be, with the words themselves kept for a screen reader. */
export function InlineSkeleton({ label, w = "9em" }: { label: string; w?: string }): JSX.Element {
  return (
    <span className="sk-inline" role="status" aria-live="polite">
      <Bar w={w} h={10} />
      <span className="sk-sr">{label}</span>
    </span>
  );
}
