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

import {
  Box,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, type JSX } from "react";
import type { GeneratedRotaTurn, GenerateRotaResult } from "../api/useGenerateRota";

/** The zone a generated turn is on. The weekend day window covers TZ1 and
 *  TZ2: SRE_WE_TZ1 (06:00-21:00), or SRE_WE_DAY (06:00-18:00) once the weekend
 *  is Day/Night, when SRE_WE_NIGHT (18:00-06:00) takes TZ3's place. */
const ZONE_OF: Record<string, "TZ1" | "TZ2" | "TZ3" | "WE"> = {
  SRE_TZ1_L1: "TZ1",
  SRE_TZ1: "TZ1",
  SRE_TZ2_L1: "TZ2",
  SRE_TZ2: "TZ2",
  SRE_TZ3: "TZ3",
  SRE_WE_TZ1: "WE",
  SRE_WE_DAY: "WE",
  SRE_WE_NIGHT: "TZ3",
};

/** The weekend Day/Night windows, whose cells are named Day and Night. */
const DAY_NIGHT = new Set(["SRE_WE_DAY", "SRE_WE_NIGHT"]);

const TIERS = ["L1", "L2", "L3"] as const;

interface DayRow {
  date: string;
  teamKey: string;
  zones: Partial<Record<"TZ1" | "TZ2" | "TZ3" | "WE", Partial<Record<(typeof TIERS)[number], string>>>>;
  /** A Day/Night weekend: its day cell reads "Day", its night cell "Night". */
  dayNight?: boolean;
}

function dayLabel(iso: string): string {
  const d = new Date(`${iso}T00:00:00Z`);
  return d.toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short", timeZone: "UTC" });
}

/** "L1 Ann · L2 Ben · L3 Cal", or a dash for a zone with nobody planned. */
function cellText(tiers: DayRow["zones"]["TZ1"]): string {
  const parts = TIERS.filter((t) => tiers?.[t]).map((t) => `${t} ${tiers?.[t]}`);
  return parts.length > 0 ? parts.join(" · ") : "—";
}

export interface GeneratedRotaPreviewProps {
  result: GenerateRotaResult;
  /** Team key -> display name. */
  teamNames?: Readonly<Record<string, string>>;
}

/**
 * Who the generator would put on each day: a row per date and team, with
 * L1, L2 and L3 per zone, then the lieu leave it would add. Regular hours
 * (SUP) are left out -- they follow the zone each person works -- and so are
 * slots set by hand, which stay as they are.
 */
export default function GeneratedRotaPreview({ result, teamNames = {} }: GeneratedRotaPreviewProps): JSX.Element {
  const rows = useMemo(() => {
    const byKey = new Map<string, DayRow>();
    const turns: GeneratedRotaTurn[] = result.turns.filter((t) => t.tier && ZONE_OF[t.shiftCode]);
    for (const t of turns) {
      const key = `${t.rotaDate}|${t.teamKey}`;
      let row = byKey.get(key);
      if (!row) {
        row = { date: t.rotaDate, teamKey: t.teamKey, zones: {} };
        byKey.set(key, row);
      }
      const zone = ZONE_OF[t.shiftCode];
      row.zones[zone] = { ...row.zones[zone], [t.tier]: t.name || t.userId };
      if (DAY_NIGHT.has(t.shiftCode)) row.dayNight = true;
    }
    return [...byKey.values()].sort((a, b) => a.date.localeCompare(b.date) || a.teamKey.localeCompare(b.teamKey));
  }, [result.turns]);

  const lieu = [...result.lieuLeave].sort((a, b) => a.startsOn.localeCompare(b.startsOn) || a.name.localeCompare(b.name));
  const team = (key: string): string => teamNames[key] ?? key;

  return (
    <Box sx={{ mt: 1 }}>
      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer sx={{ maxHeight: 320 }}>
          <Table size="small" stickyHeader aria-label="Who works when" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ "& th": { bgcolor: "action.hover" } }}>
                <TableCell>Date</TableCell>
                <TableCell>Team</TableCell>
                <TableCell>TZ1</TableCell>
                <TableCell>TZ2</TableCell>
                <TableCell>TZ3</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {rows.map((r) => (
                <TableRow key={`${r.date}|${r.teamKey}`}>
                  <TableCell sx={{ whiteSpace: "nowrap" }}>{dayLabel(r.date)}</TableCell>
                  <TableCell>{team(r.teamKey)}</TableCell>
                  {r.zones.WE ? (
                    <TableCell colSpan={2}>
                      {r.dayNight ? "Day" : "Weekend day"}: {cellText(r.zones.WE)}
                    </TableCell>
                  ) : (
                    <>
                      <TableCell>{cellText(r.zones.TZ1)}</TableCell>
                      <TableCell>{cellText(r.zones.TZ2)}</TableCell>
                    </>
                  )}
                  <TableCell>
                    {r.dayNight && r.zones.TZ3 ? "Night: " : ""}
                    {cellText(r.zones.TZ3)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Box>
      <Typography variant="caption" color="text.secondary" component="p" sx={{ mt: 0.5 }}>
        Regular hours (SUP) follow the zone each person works and are not listed. Slots set by hand are not
        listed either; they stay as they are.
      </Typography>
      {lieu.length > 0 ? (
        <Box sx={{ mt: 1 }}>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
            Lieu leave
          </Typography>
          <Box component="ul" sx={{ m: 0, pl: 2 }}>
            {lieu.map((l) => (
              <li key={`${l.userId}|${l.startsOn}`}>
                <Typography variant="body2">
                  {l.name || l.userId} ({team(l.teamKey)}): {dayLabel(l.startsOn)}
                  {l.endsOn !== l.startsOn ? ` – ${dayLabel(l.endsOn)}` : ""}
                </Typography>
              </li>
            ))}
          </Box>
        </Box>
      ) : null}
    </Box>
  );
}
