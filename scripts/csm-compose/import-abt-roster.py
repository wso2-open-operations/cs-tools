#!/usr/bin/env python3
# Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

"""
Turn a year tab of the CS ABT Roster Plan workbook into SQL for the
schedule_* tables. LOCAL DEVELOPMENT ONLY.

    python3 import-abt-roster.py "CS ABT Roster Plan.xlsx" "2026 - New" out.sql

It writes SQL rather than connecting to a database, so the output can be read
before it is applied, and prints a report of what it found -- every code it
could not place, and every cell it skipped -- so nothing is dropped silently.

The workbook holds real staff names and leave. Keep it, and the SQL this
writes, out of the repository.

Stdlib only: it reads the .xlsx as the zip of XML it is, because it needs the
cell fill colours as well as the values -- some cells in the sheet carry their
meaning in colour alone, which a CSV export loses.

Re-running is safe. Every row it writes is marked created_by = IMPORT_TAG and
the SQL deletes those first, so an import replaces the previous import and
touches nothing else: seeded data, and edits made in the portal, are left alone.
"""

import datetime
import os
import re
import sys
from collections import Counter, defaultdict

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from xlsx_reader import Book  # noqa: E402  (a sibling module, not a package)

IMPORT_TAG = "import:abt-roster"


# ── What each sheet code means ──────────────────────────────────────────────
# A cell code maps to a rota window (team_schedule_shift.code) or to a whole-day
# absence (team_schedule_absence_kind.code, plus who an allocation is for).
SHIFTS = {
    "lk": "CRE_REGULAR",
    "ind": "CRE_REGULAR_IND",
    # Allo-IND is the India region shift written as an allocation, not an
    # allocation to anybody.
    "allo-ind": "CRE_REGULAR_IND",
    "6-9am": "CRE_MORNING",
    "6-9am-oc": "CRE_MORNING_OC",
    "6-9pm": "CRE_EVENING",
    # The sheet writes a few evening, weekend and non-LK cells as "-OC", but
    # there is no second on-call role behind those windows: the person is
    # simply on the window.
    "6-9pm-oc": "CRE_EVENING",
    "nlk": "CRE_AMERICAS",
    "nlk-oc": "CRE_AMERICAS",
    "we": "CRE_WEEKEND",
    "we-oc": "CRE_WEEKEND",
    "nlk-we": "CRE_WEEKEND_NIGHT",
    "nlk-we-oc": "CRE_WEEKEND_NIGHT_OC",
}

ABSENCES = {
    "al": ("ANNUAL_LEAVE", None),
    "ll": ("LIEU_LEAVE", None),
    "l": ("LIEU_LEAVE", None),  # one cell in 2026; LL with the second L missing
    "sl": ("SICK_LEAVE", None),
    # ML on the sheet is Maternity leave, confirmed by the people who keep the
    # sheet. It was mapped to SICK_LEAVE for a while on the reading that it
    # meant Medical leave; that was wrong, and it filed real maternity leave as
    # sick leave.
    #
    # The sheet itself says so twice over, which is worth recording because the
    # short-code coincidence is genuinely confusing: SL above is already sick
    # leave, so a second code for the same thing would be redundant, and PL
    # directly below is paternity. ML and PL are a pair.
    "ml": ("MATERNITY_LEAVE", None),
    "pl": ("PATERNITY_LEAVE", None),
    "mig": ("MIGRATION", None),
    # Lent to the Migration team from their ABT: away from the ABT's rota
    # exactly as a migration allocation is.
    "mig-rota": ("MIGRATION", None),
    "on-boarding": ("ONBOARDING", None),
    "exclude": ("EXCLUDED", None),
    # Allo-EXT and Allo-INT are the catalogue's own external / internal
    # allocation kinds. They used to be filed as a customer allocation and as
    # RnD; both were retired from CRE's list (RnD is SRE's), so they land on
    # the kinds a lead now picks for the same thing.
    "allo-ext": ("ALLO_EXT", None),
    "allo-int": ("ALLO_INT", None),
    "allo-br": ("ALLO_BR", None),
}

# Anything else written Allo-<X> is time with customer X. The sheet does not
# say on site or off, and off site is the common case.
# The teams the seed creates, which are the only keys the rest of the stack
# knows. A workbook heading that does not land on one of these is a heading
# this importer has not been taught, not a new team: the seed's own clean-up
# looks teams up by key, team_member rows are inserted by key, and an
# assignment written under an unknown key gets team_id NULL and is invisible
# to both. Better to stop and say which heading than to import silently.
CANONICAL_TEAM_KEYS = {
    "castor", "draco", "vega", "sirius", "atlas", "phoenix", "rigel",
    "americas", "migration", "apollo", "artemis",
}

CUSTOMER_PREFIX = "allo-"
# A named customer ("Allo-Acme") is external work, with the customer kept as
# allocated_to -- the same kind 0154 moves older customer allocations onto.
CUSTOMER_KIND = "ALLO_EXT"

# What follows Allo- when it names the Brazil rotation rather than a customer.
# tokens() has already stripped the spaces and lowercased by this point, so
# "Allo-BR Rotation" arrives as "allo-brrotation".
BRAZIL_ROTATION = re.compile(r"(br|brazil|brasil)(rotation|rota)?")

# Kinds whose span may run across a weekend with nothing written on it. An
# allocation or an exclusion does not stop for a Saturday; leave does -- a
# Friday and a Monday off are two separate absences, not four days.
BRIDGES_WEEKENDS = {"MIGRATION", "ONBOARDING", "EXCLUDED", "ALLO_EXT", "ALLO_INT", "RND", "ALLO_BR"}

# Cells whose meaning is their colour alone -- no text, the fill of a code.
# Only these fills, and only on an otherwise empty cell.
COLOUR_ONLY = {
    "FFEAD1DC": "lk",
    "FFFCE5CD": "mig",
    "FFFFD966": "we",
    "FFB45F06": "exclude",
    "FF1155CC": "6-9pm",
    "FFB6D7A8": "nlk",
    "FFB4A7D6": "nlk-we-oc",
}

# Column headings that are not engineers.
NOT_PEOPLE = {"rota lead"}


def as_date(v):
    try:
        return datetime.date(1899, 12, 30) + datetime.timedelta(days=int(float(v)))
    except (TypeError, ValueError):
        return None


def tokens(text):
    """Split a cell into its codes: '6-9pm/6-9am-OC', 'LK - Allo-ACME',
    'LL- 6-9am-OC', 'WE - OC' (one code, spaced out)."""
    parts = re.split(r"\s*/\s*|\n|\s-\s|(?<=^ll)-\s", text.strip(), flags=re.I)
    out = []
    for p in parts:
        k = re.sub(r"\s+", "", p).lower()
        if not k:
            continue
        if k == "oc" and out:  # 'WE - OC' split on its spaced dash
            out[-1] += "-oc"
        else:
            out.append(k)
    return out


def sql(v):
    return "NULL" if v is None else "'" + str(v).replace("'", "''") + "'"


# ── The sheet → rows ────────────────────────────────────────────────────────
def main(xlsx, sheet, out_path):
    g = Book(xlsx).grid(sheet)
    cell = lambda r, c: g.get((r, c), (None, None))
    maxr = max(r for r, _ in g)
    maxc = max(c for _, c in g)

    team_row = next(
        r for r in range(1, 30)
        if any(isinstance(cell(r, c)[0], str) and re.match(r"^\w+ \(.+\)$", cell(r, c)[0]) for c in range(1, maxc + 1))
    )
    name_row = team_row + 1
    day_rows = [r for r in range(name_row + 1, maxr + 1) if as_date(cell(r, 2)[0])]

    # Teams run left to right; a team owns every column up to the next heading.
    heads = [(c, cell(team_row, c)[0]) for c in range(1, maxc + 1)
             if isinstance(cell(team_row, c)[0], str) and re.match(r"^\w+ \(.+\)$", cell(team_row, c)[0])]
    columns = []  # (col, team_key, nickname)
    leads = {}    # team_key -> lead nickname
    for i, (c0, head) in enumerate(heads):
        team, lead = re.match(r"^(\w+) \((.+)\)$", head).groups()
        key = team.lower()
        if key not in CANONICAL_TEAM_KEYS:
            raise SystemExit(
                f"sheet heading {head!r} gives team key {key!r}, which is not one the "
                f"stack knows ({', '.join(sorted(CANONICAL_TEAM_KEYS))}).\n"
                "Add it to the seed and to CANONICAL_TEAM_KEYS, or fix the heading."
            )
        leads[key] = lead.strip()
        c1 = heads[i + 1][0] if i + 1 < len(heads) else maxc + 1
        for c in range(c0, c1):
            raw = cell(name_row, c)[0]
            if not raw:
                continue
            nick = re.sub(r"\s*\(.*\)\s*$", "", raw).strip()  # 'Jane (R&D )'
            if nick.lower() in NOT_PEOPLE:
                continue
            columns.append((c, key, nick))

    email = lambda nick: re.sub(r"\s+", "", nick).lower() + "@wso2.com"

    # One person can have a column under two teams: they moved, and the sheet
    # kept their old column until the move. On a day both columns carry, the
    # later team wins; their membership is the team of their last column.
    by_person = defaultdict(list)
    for c, key, nick in columns:
        by_person[email(nick)].append((c, key, nick))

    report = Counter()
    unknown = Counter()
    assignments = []           # (email, team_key, date, shift_code)
    days_by_person = defaultdict(dict)  # email -> date -> (team_key, [(kind, allocated_to)])
    # The days each person is actually rostered, so an absence span cannot be
    # bridged across one. Kept alongside rather than derived from
    # `assignments`, which is a flat list and would need scanning per gap.
    on_rota = defaultdict(set)  # email -> {date}

    for mail, cols in by_person.items():
        for r in day_rows:
            d = as_date(cell(r, 2)[0])
            weekend = d.weekday() >= 5
            chosen = None
            for c, key, _ in reversed(cols):  # later team first
                v, f = cell(r, c)
                if v or (f in COLOUR_ONLY):
                    chosen = (key, v, f)
                    break
            if not chosen:
                continue
            key, v, f = chosen
            if v:
                codes = tokens(v)
            else:
                code = COLOUR_ONLY[f]
                if code == "lk" and weekend:
                    continue
                codes = [code]
                report["colour-only cells read by fill"] += 1
            for code in codes:
                if code in SHIFTS:
                    assignments.append((mail, key, d, SHIFTS[code]))
                    on_rota[mail].add(d)
                elif code in ABSENCES:
                    kind, to = ABSENCES[code]
                    days_by_person[mail].setdefault(d, (key, []))[1].append((kind, to))
                elif code.startswith(CUSTOMER_PREFIX) and BRAZIL_ROTATION.fullmatch(
                    code[len(CUSTOMER_PREFIX):]
                ):
                    # Brazil is a rotation the team takes a turn at, not a
                    # customer somebody is allocated to. Only the exact
                    # 'Allo-BR' reached ABSENCES above, so every other way the
                    # sheet writes it -- Allo-Brazil, Allo-BR Rotation, which
                    # loses its space in tokens() -- fell through to the
                    # customer branch and became an off-site allocation to a
                    # customer named Brazil.
                    days_by_person[mail].setdefault(d, (key, []))[1].append(("ALLO_BR", None))
                elif code.startswith(CUSTOMER_PREFIX):
                    # From the original cell, so 'Allo-Acme' keeps the casing
                    # the sheet wrote -- but matched against THIS code, not the
                    # first Allo- in the cell. 'Allo-IND / Allo-Acme' holds two,
                    # and searching the whole cell gave the second one the
                    # first one's customer.
                    want = re.escape(code[len(CUSTOMER_PREFIX):])
                    m = re.search(rf"allo-\s*({want})\b", v or "", re.I)
                    customer = m.group(1) if m else code[len(CUSTOMER_PREFIX):].upper()
                    days_by_person[mail].setdefault(d, (key, []))[1].append((CUSTOMER_KIND, customer))
                else:
                    unknown[code] += 1

    # Consecutive days of the same absence become one span.
    absences = []  # (email, team_key, kind, allocated_to, starts_on, ends_on)
    for mail, days in days_by_person.items():
        open_spans = {}  # (kind, to) -> [team_key, start, end]
        for d in sorted(days):
            key, items = days[d]
            seen = set()
            for kind, to in items:
                k = (kind, to)
                seen.add(k)
                span = open_spans.get(k)
                if span:
                    gap = [span[2] + datetime.timedelta(days=i) for i in range(1, (d - span[2]).days)]
                    # `days` holds this person's absence days only, so a
                    # weekend they are rostered on looked empty and the span
                    # swallowed it. OnDutyAt now hides anyone whose assignment
                    # overlaps an absence, so bridging a worked Saturday takes
                    # that engineer off the weekend entirely.
                    bridged = kind in BRIDGES_WEEKENDS and all(
                        x.weekday() >= 5 and x not in days and x not in on_rota[mail]
                        for x in gap)
                    if (d - span[2]).days == 1 or bridged:
                        span[2] = d
                        continue
                    absences.append((mail, span[0], kind, to, span[1], span[2]))
                open_spans[k] = [key, d, d]
        for (kind, to), (key, s, e) in open_spans.items():
            absences.append((mail, key, kind, to, s, e))

    people = []
    for mail, cols in by_person.items():
        c, key, nick = cols[-1]
        people.append((mail, nick, key, leads.get(key, "").lower() == nick.lower()))
    # Leads named in a heading but with no column of their own still lead it.
    for key, lead in leads.items():
        if lead.lower() in NOT_PEOPLE or email(lead) in by_person:
            continue
        people.append((email(lead), lead, key, True))

    write_sql(out_path, sheet, people, assignments, absences)

    first, last = as_date(cell(day_rows[0], 2)[0]), as_date(cell(day_rows[-1], 2)[0])
    print(f"sheet            {sheet!r}: {first} -> {last}, {len(day_rows)} days")
    print(f"engineers        {len(by_person)} (from {len(columns)} columns)")
    print(f"movers           {', '.join(cols[-1][2] + ': ' + ' -> '.join(k for _, k, _ in cols) for cols in by_person.values() if len(cols) > 1) or 'none'}")
    print(f"leads            {', '.join(f'{k}={v}' for k, v in leads.items())}")
    print(f"assignments      {len(assignments)}  {dict(Counter(s for *_, s in assignments).most_common())}")
    print(f"absence spans    {len(absences)}  {dict(Counter(k for _, _, k, *_ in absences).most_common())}")
    print(f"customers        {sorted({to for *_, to, _, _ in absences if to})}")
    for k, v in report.items():
        print(f"{k:16} {v}")
    print(f"UNPLACED codes   {dict(unknown) or 'none'}")
    print(f"wrote            {out_path}")


def write_sql(path, sheet, people, assignments, absences):
    t = sql(IMPORT_TAG)
    w = []
    w.append(f"-- Generated by import-abt-roster.py from sheet {sheet!r}. LOCAL DEVELOPMENT ONLY.")
    w.append("-- Real staff data: do not commit this file.")
    w.append("BEGIN;")
    w.append("CREATE TEMP TABLE _imp_person (email TEXT, nick TEXT, team_key TEXT, is_lead BOOLEAN) ON COMMIT DROP;")
    # Written only when there are rows: `INSERT ... VALUES` with nothing after
    # it is a syntax error, and it aborts the whole transaction -- taking the
    # assignment import down with it.
    if people:
        w.append("INSERT INTO _imp_person VALUES")
        w.append(",\n".join(
            f"  ({sql(e)}, {sql(n)}, {sql(k)}, {'TRUE' if l else 'FALSE'})" for e, n, k, l in people) + ";")

    # People: reuse an existing account by email; create the rest as internal
    # staff, which is what lets entity-service's access check admit them.
    w.append(f"""
INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by,
                    user_name, name, first_name, email, is_active, is_system_user, user_type)
SELECT md5('{IMPORT_TAG}-user-' || p.email)::uuid, NOW(), NOW(), {t}, {t},
       p.email, p.nick, p.nick, p.email, TRUE, FALSE, 'INTERNAL'
  FROM _imp_person p
 WHERE NOT EXISTS (SELECT 1 FROM "user" u WHERE lower(u.email) = p.email OR lower(u.user_name) = p.email);

CREATE TEMP TABLE _imp_uid ON COMMIT DROP AS
SELECT p.email,
       (SELECT u.id FROM "user" u WHERE lower(u.email) = p.email OR lower(u.user_name) = p.email
         ORDER BY u.created_on LIMIT 1) AS user_id
  FROM _imp_person p;

-- The imported teams are the real ones now: the seed's stand-in engineers in
-- them are taken off, so the roster is not half real people and half fakes.
DELETE FROM team_schedule_assignment WHERE created_by = 'seed' AND team_key IN (SELECT DISTINCT team_key FROM _imp_person);
DELETE FROM team_schedule_absence    WHERE created_by = 'seed' AND team_key IN (SELECT DISTINCT team_key FROM _imp_person);
DELETE FROM team_member m USING team t
 WHERE m.team_id = t.id AND m.created_by = 'seed' AND lower(t.name) IN (SELECT DISTINCT team_key FROM _imp_person);

DELETE FROM team_member WHERE created_by = {t};
INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, role)
SELECT md5('{IMPORT_TAG}-tm-' || p.email)::uuid, NOW(), NOW(), {t}, {t}, tm.id, u.user_id,
       CASE WHEN p.is_lead THEN 'lead' ELSE 'engineer' END
  FROM _imp_person p
  JOIN _imp_uid u ON u.email = p.email
  JOIN team tm ON lower(tm.name) = p.team_key
 WHERE NOT EXISTS (SELECT 1 FROM team_member m WHERE m.user_id = u.user_id AND m.team_id = tm.id);

-- Everyone on the sheet is WSO2 staff, so they get the internal role. Not
-- cosmetic: recompute_user_type() derives "user".user_type from roles, and the
-- schedule endpoints admit only an INTERNAL caller -- without this the people
-- the rota is about are refused it (see seed-team-schedule.sql, which does the
-- same for its own engineers). Tagged as this import's rows, so the seed's
-- clean-up of its own grants leaves them alone.
DELETE FROM user_role WHERE created_by = {t};
INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
SELECT md5('{IMPORT_TAG}-ur-' || u.user_id::text)::uuid, NOW(), NOW(), {t}, {t},
       u.user_id, '00000000-0000-0000-0000-000000000101'::uuid
  FROM _imp_uid u
 WHERE NOT EXISTS (SELECT 1 FROM user_role ur
                    WHERE ur.user_id = u.user_id
                      AND ur.role_id = '00000000-0000-0000-0000-000000000101'::uuid)
ON CONFLICT (id) DO NOTHING;
""")

    w.append("CREATE TEMP TABLE _imp_asg (email TEXT, team_key TEXT, d DATE, shift TEXT) ON COMMIT DROP;")
    for i in range(0, len(assignments), 1000):
        chunk = assignments[i:i + 1000]
        w.append("INSERT INTO _imp_asg VALUES\n" + ",\n".join(f"  ({sql(e)},{sql(k)},'{d}',{sql(s)})" for e, k, d, s in chunk) + ";")
    w.append(f"""
DELETE FROM team_schedule_assignment WHERE created_by = {t};
-- Instants resolved in the clock each window was written in, and stored, the
-- same as every other writer of this table.
INSERT INTO team_schedule_assignment
  (user_id, team_id, team_key, shift_id, zone_id, tier, rota_date, starts_at, ends_at,
   is_on_call, source, created_by, updated_by)
SELECT u.user_id, tm.id, a.team_key, s.id, s.zone_id, s.tier, a.d,
       (a.d::timestamp + make_interval(mins => s.start_minute)) AT TIME ZONE s.authoring_time_zone,
       (a.d::timestamp + make_interval(mins => s.end_minute))   AT TIME ZONE s.authoring_time_zone,
       s.is_on_call, 'IMPORTED', {t}, {t}
  FROM _imp_asg a
  JOIN _imp_uid u ON u.email = a.email
  JOIN team_schedule_shift s ON s.code = a.shift
  LEFT JOIN team tm ON lower(tm.name) = a.team_key
ON CONFLICT DO NOTHING;
""")

    w.append("CREATE TEMP TABLE _imp_abs (email TEXT, team_key TEXT, kind TEXT, allocated_to TEXT, s DATE, e DATE) ON COMMIT DROP;")
    if absences:
        w.append("INSERT INTO _imp_abs VALUES\n" + ",\n".join(
            f"  ({sql(m)},{sql(k)},{sql(kd)},{sql(to)},'{s}','{e}')" for m, k, kd, to, s, e in absences) + ";")
    w.append(f"""
DELETE FROM team_schedule_absence WHERE created_by = {t};
INSERT INTO team_schedule_absence (user_id, team_key, kind_id, allocated_to, starts_on, ends_on, note, created_by, updated_by)
SELECT u.user_id, a.team_key, k.id, a.allocated_to, a.s, a.e, 'imported from the ABT roster sheet', {t}, {t}
  FROM _imp_abs a
  JOIN _imp_uid u ON u.email = a.email
  JOIN team_schedule_absence_kind k ON k.code = a.kind;

-- What landed, so a short count is visible without a second query.
SELECT 'assignments' AS what, count(*) FROM team_schedule_assignment WHERE created_by = {t}
UNION ALL SELECT 'absences', count(*) FROM team_schedule_absence WHERE created_by = {t}
UNION ALL SELECT 'team members', count(*) FROM team_member WHERE created_by = {t}
UNION ALL SELECT 'assignment rows not placed', (SELECT count(*) FROM _imp_asg) - (SELECT count(*) FROM team_schedule_assignment WHERE created_by = {t})
UNION ALL SELECT 'absence rows not placed', (SELECT count(*) FROM _imp_abs) - (SELECT count(*) FROM team_schedule_absence WHERE created_by = {t});
COMMIT;
""")
    with open(path, "w") as fh:
        fh.write("\n".join(w))


if __name__ == "__main__":
    if len(sys.argv) != 4:
        sys.exit(__doc__)
    main(*sys.argv[1:])
