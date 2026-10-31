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
Turn a team rotation import workbook (team_rotation_import.xlsx: an
Assignments sheet and a Paging Contacts sheet) into SQL for the team_schedule_*
and paging_contact tables.

The SQL is data only: no DDL beyond its own temp tables, so it loads into a
database on whatever schema version that database already runs, without a
migration or a redeploy. Where the schema is older it adapts rather than fails
(see RETIRED_SHIFTS; phones are skipped without paging_contact). Run it with
--rollback first and read the report. --add-missing-people and --clear-span
are for a local database: elsewhere accounts come from the ServiceNow sync, and
a clear would delete turns leads placed by hand.

    python3 import-team-rotation.py <workbook.xlsx> <out.sql> [options]

    --mapping grid_mapping.json  team names for the sheet's team keys (the
                                 converter's mapping); the workbook's own
                                 Teams sheet is read too
    --clear-span                 first take every turn off the sheet's teams
                                 on the dates the sheet covers, whoever put it
                                 there, so the rota shows only the sheet
    --add-missing-people         create an account for an unknown email, on
                                 the sheet's team, instead of reporting and
                                 skipping it
    --rollback                   end the SQL with ROLLBACK: run it to see the
                                 report without writing anything

It writes SQL rather than connecting to a database, so the output can be read
before it is applied, and prints a report of everything it could not place.

One Assignments row is what one POST /team-schedule/assignments/apply does,
and the SQL does it the same way, day by day in sheet order: a turn replaces
the turns that person already holds that day on that team in the same zone or
overlapping it in time, and leaves their regular hours alone. Days the window
is not worked on are skipped. Re-running the same file gives the same result.
Paging Contacts sets a number only for someone who has none, as
PUT /team-schedule/paging-contacts does when the phone import runs.

The workbook may name windows from an older catalogue. A window the database
still runs is used as named; one it has retired falls back through
RETIRED_SHIFTS, the way migration 0154 moved existing turns: the old weekend
TZ2 (18:00-06:00) is TZ3, whose window now runs every day. A fallen-back row
still covers only the days its own window was worked on.

The workbook holds real staff names and phone numbers. Keep it, and the SQL
this writes, out of the repository.
"""

import argparse
import datetime
import json
import os
import re
import sys
from collections import Counter, defaultdict

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from xlsx_reader import Book  # noqa: E402  (a sibling module, not a package)

IMPORT_TAG = "import:team-rotation"
INTERNAL_ROLE_ID = "00000000-0000-0000-0000-000000000101"

# Old window -> the window that does its job in the current catalogue.
RETIRED_SHIFTS = {"SRE_WE_TZ2": "SRE_TZ3"}

ASSIGNMENT_HEADER = ["team key", "shift code", "tier", "from", "to", "email", "note"]
PHONE_HEADER = ["email", "phone"]
EMAIL = re.compile(r"^[^@\s]+@[^@\s]+\.[^@\s]+$")
E164 = re.compile(r"^\+[1-9][0-9]{6,14}$")
# Country code -> national number lengths the paging service accepts.
PHONE_LENGTHS = {"94": {9}, "91": {10}, "55": {10, 11}}


def rows(book, name):
    """The sheet as a list of rows, header first, each cell a string or None."""
    g = book.grid(name)
    if not g:
        return []
    last_row = max(r for r, _ in g)
    last_col = max(c for _, c in g)
    return [[g.get((r, c), (None,))[0] for c in range(1, last_col + 1)] for r in range(1, last_row + 1)]


def as_date(v):
    """An Excel serial or YYYY-MM-DD, as a date; None if it is neither."""
    if v is None:
        return None
    s = str(v).strip()
    try:
        return datetime.date.fromisoformat(s[:10])
    except ValueError:
        pass
    try:
        return datetime.date(1899, 12, 30) + datetime.timedelta(days=int(float(s)))
    except ValueError:
        return None


def day_scope(days):
    """The Windows sheet's Days column as the catalogue's day_scope."""
    d = (days or "").strip().lower()
    if d.startswith("weekday"):
        return "WEEKDAY"
    if d.startswith("weekend"):
        return "WEEKEND"
    return "ANY"


def worked(scope, d):
    weekend = d.weekday() >= 5
    return not ((scope == "WEEKDAY" and weekend) or (scope == "WEEKEND" and not weekend))


def sql(v):
    if v is None:
        return "NULL"
    return "'" + str(v).replace("'", "''") + "'"


def check_header(got, want, sheet):
    norm = [(c or "").strip().lower() for c in got[:len(want)]]
    if norm != want:
        sys.exit(f"{sheet}: expected columns {want}, found {norm}")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("workbook")
    ap.add_argument("out_sql")
    ap.add_argument("--mapping")
    ap.add_argument("--clear-span", action="store_true")
    ap.add_argument("--add-missing-people", action="store_true")
    ap.add_argument("--rollback", action="store_true")
    args = ap.parse_args()

    book = Book(args.workbook)
    issues = []

    # ── Teams: the sheet's key, and the name to find the team by where the
    #    database knows it under another key (local keys are short: "apollo").
    team_names = {}
    if "Teams" in book.sheets:
        t = rows(book, "Teams")
        hdr = [(c or "").strip().lower() for c in t[0]]
        if "team key" in hdr and "old sheet" in hdr:
            ki, ni = hdr.index("team key"), hdr.index("old sheet")
            for r in t[1:]:
                if r[ki]:
                    team_names[r[ki].strip()] = (r[ni] or "").strip()
    skipped_teams = set()
    if args.mapping:
        with open(args.mapping) as fh:
            mapping = json.load(fh)
        for name, key in mapping.get("teams", {}).items():
            if key and key != "skip":
                team_names.setdefault(key, name)
            elif key == "skip":
                skipped_teams.add(name)

    # ── Windows: which days each window is worked on, by the sheet's own rule.
    scopes = {}
    if "Windows" in book.sheets:
        w = rows(book, "Windows")
        hdr = [(c or "").strip().lower() for c in w[0]]
        ci, di = hdr.index("shift code"), hdr.index("days")
        for r in w[1:]:
            if r[ci]:
                scopes[r[ci].strip()] = day_scope(r[di])

    # ── Assignments, expanded to one row per worked day, in sheet order.
    a = rows(book, "Assignments")
    check_header(a[0], ASSIGNMENT_HEADER, "Assignments")
    days = []          # (seq, email, team_key, target_shift, tier, date, note, sheet_row)
    translated = Counter()
    off_days = 0
    for n, r in enumerate(a[1:], start=2):
        r = (r + [None] * 7)[:7]
        if not any(r):
            continue
        team, shift, tier, f, t, email, note = [(c.strip() if isinstance(c, str) else c) for c in r]
        where = f"Assignments row {n}"
        email = (email or "").lower()
        tier = (tier or "").upper() or None
        bad = []
        if not team:
            bad.append("no team key")
        if not shift:
            bad.append("no shift code")
        if tier not in (None, "L1", "L2", "L3"):
            bad.append(f"tier {tier!r} is not L1, L2 or L3")
        if not EMAIL.match(email):
            bad.append(f"email {email!r} is not an email")
        fd, td = as_date(f), as_date(t)
        if fd is None or td is None:
            bad.append(f"dates {f!r}..{t!r} are not dates")
        if bad:
            issues.append(f"{where}: skipped -- " + "; ".join(bad))
            continue
        if td < fd:
            fd, td = td, fd
        if (td - fd).days > 366:
            issues.append(f"{where}: skipped -- {fd}..{td} is longer than a year")
            continue
        if shift.endswith("_L1") and tier not in (None, "L1"):
            issues.append(f"{where}: skipped -- {shift} holds L1 only, not {tier}")
            continue
        if shift in ("SRE_TZ1", "SRE_TZ2") and tier == "L1":
            issues.append(f"{where}: note -- L1 on {shift}; the sheet says use {shift}_L1 for L1")
        if shift not in scopes:
            issues.append(f"{where}: note -- {shift} is not on the Windows sheet; the catalogue decides its days")
        alt = RETIRED_SHIFTS.get(shift)
        if alt:
            translated[(shift, alt)] += 1
        scope = scopes.get(shift, "ANY")
        d = fd
        while d <= td:
            if worked(scope, d):
                days.append((len(days) + 1, email, team, shift, alt, tier, d, note, n))
            else:
                off_days += 1
            d += datetime.timedelta(days=1)

    # ── Paging contacts.
    phones = []
    if "Paging Contacts" in book.sheets:
        p = rows(book, "Paging Contacts")
        check_header(p[0], PHONE_HEADER, "Paging Contacts")
        seen = {}
        for n, r in enumerate(p[1:], start=2):
            r = (r + [None, None])[:2]
            email = (r[0] or "").strip().lower()
            phone = re.sub(r"[\s\-()]", "", str(r[1] or ""))
            if not email and not phone:
                continue
            if phone and not phone.startswith("+"):
                phone = "+" + phone
            where = f"Paging Contacts row {n}"
            if not EMAIL.match(email):
                issues.append(f"{where}: skipped -- email {email!r} is not an email")
                continue
            if not E164.match(phone):
                issues.append(f"{where}: skipped -- phone for {email} is not +<country><number>")
                continue
            cc = next((c for c in PHONE_LENGTHS if phone[1:].startswith(c)), None)
            if cc is None or len(phone) - 1 - len(cc) not in PHONE_LENGTHS[cc]:
                issues.append(f"{where}: note -- {email}'s number is not a +94/+91/+55 number of the usual length")
            if email in seen:
                issues.append(f"{where}: skipped -- {email} already has a number on row {seen[email]}")
                continue
            seen[email] = n
            phones.append((email, phone))

    teams_used = sorted({k for _, _, k, *_ in days})
    write_sql(args, days, phones, {k: team_names.get(k) for k in teams_used})

    # ── Report.
    print(f"Assignments: {len(a) - 1} sheet rows -> {len(days)} person-days "
          f"({off_days} days skipped: the window is not worked that day)")
    for (old, new), c in sorted(translated.items()):
        print(f"  {c} row(s) on {old}: written as {new} where the database no longer runs {old} (migration 0154)")
    per = Counter((k, s) for _, _, k, s, *_ in days)
    for (k, s), c in sorted(per.items()):
        print(f"  {k:24} {s:12} {c}")
    if days:
        print(f"  dates {min(x[6] for x in days)} .. {max(x[6] for x in days)}, "
              f"{len({x[1] for x in days})} people")
    for k in teams_used:
        print(f"  team {k}: found by key, else by name {team_names.get(k)!r}")
    if skipped_teams:
        print(f"  teams the mapping skips: {', '.join(sorted(skipped_teams))}")
    print(f"Paging contacts: {len(phones)} numbers")
    for i in issues:
        print("  " + i)
    print(f"Wrote {args.out_sql}. Apply it with psql; its last SELECT says what landed and what did not.")


def write_sql(args, days, phones, team_names):
    t = sql(IMPORT_TAG)
    w = [f"-- Generated by import-team-rotation.py from {os.path.basename(args.workbook)}. Data only, no schema change.",
         "-- Real staff data: do not commit this file.",
         "BEGIN;",
         f"SELECT set_config('app.actor', {t}, true);",
         "CREATE TEMP TABLE _rot_team (sheet_key TEXT, team_name TEXT) ON COMMIT DROP;",
         "CREATE TEMP TABLE _rot_day (seq INT, email TEXT, sheet_key TEXT, shift TEXT, alt TEXT, tier TEXT, d DATE, note TEXT, sheet_row INT) ON COMMIT DROP;",
         "CREATE TEMP TABLE _rot_phone (email TEXT, phone TEXT) ON COMMIT DROP;"]
    if team_names:
        w.append("INSERT INTO _rot_team VALUES\n" + ",\n".join(
            f"  ({sql(k)}, {sql(n)})" for k, n in team_names.items()) + ";")
    for i in range(0, len(days), 1000):
        w.append("INSERT INTO _rot_day VALUES\n" + ",\n".join(
            f"  ({s},{sql(e)},{sql(k)},{sql(sh)},{sql(al)},{sql(ti)},'{d}',{sql(no)},{n})"
            for s, e, k, sh, al, ti, d, no, n in days[i:i + 1000]) + ";")
    if phones:
        w.append("INSERT INTO _rot_phone VALUES\n" + ",\n".join(f"  ({sql(e)}, {sql(p)})" for e, p in phones) + ";")

    w.append(f"""
-- The sheet's team key as this database's team: the key itself, else the
-- team's name (a local database keys SRE SaaS as "apollo", DEV as
-- "apollo_abt_sre_team").
CREATE TEMP TABLE _rot_team_id ON COMMIT DROP AS
SELECT r.sheet_key,
       COALESCE((SELECT tm.id FROM team tm WHERE lower(tm.key) = lower(r.sheet_key)),
                (SELECT tm.id FROM team tm WHERE lower(tm.name) = lower(r.team_name)
                  ORDER BY tm.created_on LIMIT 1)) AS team_id
  FROM _rot_team r;
""")
    if args.add_missing_people:
        w.append(f"""
-- --add-missing-people: an email the database does not know becomes an
-- internal account, an engineer on the team the sheet puts them on. Someone
-- who already has an account is never added to another team here: that is a
-- lead's call, and is reported as "not on the team" instead.
INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by,
                    user_name, name, first_name, email, is_active, is_system_user, user_type)
SELECT md5({t} || '-user-' || e.email)::uuid, NOW(), NOW(), {t}, {t},
       e.email, split_part(e.email, '@', 1), split_part(e.email, '@', 1), e.email, TRUE, FALSE, 'INTERNAL'
  FROM (SELECT DISTINCT email FROM _rot_day UNION SELECT email FROM _rot_phone) e
 WHERE NOT EXISTS (SELECT 1 FROM "user" u WHERE lower(u.email) = e.email OR lower(u.user_name) = e.email)
ON CONFLICT (id) DO NOTHING;
""")

    w.append(f"""
-- One account per email and team. An email can belong to more than one
-- account (an old guest record beside the real one): the active account that
-- is on the team wins, then any active one, then the oldest.
CREATE TEMP TABLE _rot_uid ON COMMIT DROP AS
SELECT p.email, p.sheet_key, ti.team_id,
       (SELECT u.id FROM "user" u
         WHERE lower(u.email) = p.email OR lower(u.user_name) = p.email
         ORDER BY u.is_active DESC,
                  EXISTS (SELECT 1 FROM team_member m WHERE m.user_id = u.id AND m.team_id = ti.team_id) DESC,
                  u.created_on
         LIMIT 1) AS user_id
  FROM (SELECT DISTINCT email, sheet_key FROM _rot_day) p
  JOIN _rot_team_id ti ON ti.sheet_key = p.sheet_key;
""")
    if args.add_missing_people:
        w.append(f"""
INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, role)
SELECT md5({t} || '-tm-' || u.user_id::text || '-' || u.team_id::text)::uuid, NOW(), NOW(), {t}, {t},
       u.team_id, u.user_id, 'engineer'
  FROM _rot_uid u
  JOIN "user" nu ON nu.id = u.user_id AND nu.created_by = {t}
 WHERE u.team_id IS NOT NULL
   AND NOT EXISTS (SELECT 1 FROM team_member m WHERE m.user_id = u.user_id AND m.team_id = u.team_id)
ON CONFLICT (id) DO NOTHING;

-- The internal role is what lets entity-service admit them to the schedule
-- (recompute_user_type derives user_type from roles); see import-abt-roster.py.
INSERT INTO user_role (id, created_on, updated_on, created_by, updated_by, user_id, role_id)
SELECT md5({t} || '-ur-' || u.id::text)::uuid, NOW(), NOW(), {t}, {t}, u.id, '{INTERNAL_ROLE_ID}'::uuid
  FROM "user" u
 WHERE u.created_by = {t}
   AND NOT EXISTS (SELECT 1 FROM user_role ur WHERE ur.user_id = u.id AND ur.role_id = '{INTERNAL_ROLE_ID}'::uuid)
ON CONFLICT (id) DO NOTHING;
""")

    w.append(f"""
-- Who a span has moved onto another team for some days (Americas cover).
-- Read only where the schema has moves (migration 0202); an older schema has
-- none, and is loaded as it is.
CREATE TEMP TABLE _rot_moved (user_id UUID, team_key TEXT, starts_on DATE, ends_on DATE) ON COMMIT DROP;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_name = 'team_schedule_absence_kind' AND column_name = 'moves_to_team_key') THEN
        EXECUTE $q$
            INSERT INTO _rot_moved
            SELECT ab.user_id, lower(ab.team_key), ab.starts_on, ab.ends_on
              FROM team_schedule_absence ab
              JOIN team_schedule_absence_kind k ON k.id = ab.kind_id
             WHERE k.moves_to_team_key IS NOT NULL$q$;
    END IF;
END $$;

-- Every person-day, resolved, with why it cannot be placed if it cannot. The
-- service refuses someone not on the team, unless a span has moved them there
-- for those dates; so does this.
--
-- The window is the sheet's own code where this database still runs it, else
-- its replacement (RETIRED_SHIFTS): a database on the older catalogue keeps
-- its weekend TZ2, one on the current catalogue gets TZ3.
CREATE TEMP TABLE _rot_plan ON COMMIT DROP AS
SELECT d.*, u.user_id, u.team_id, tm.key AS team_key, s.id AS shift_id, s.code AS shift_code,
       CASE
         WHEN u.team_id IS NULL THEN 'no such team'
         WHEN u.user_id IS NULL THEN 'unknown email'
         WHEN s.id IS NULL THEN 'no such window'
         WHEN NOT s.is_active THEN 'window is retired'
         WHEN (s.day_scope = 'WEEKDAY' AND extract(isodow FROM d.d) >= 6)
           OR (s.day_scope = 'WEEKEND' AND extract(isodow FROM d.d) < 6) THEN 'window not worked that day'
         WHEN d.tier IS NULL AND s.is_escalation AND s.tier IS NULL THEN 'window needs a tier'
         WHEN d.tier IS NOT NULL AND NOT s.is_escalation THEN 'window holds no tier'
         WHEN s.tier IS NOT NULL AND d.tier IS NOT NULL AND s.tier::text <> d.tier THEN 'tier does not fit the window'
         WHEN NOT EXISTS (SELECT 1 FROM team_member m WHERE m.user_id = u.user_id AND m.team_id = u.team_id)
          AND NOT EXISTS (SELECT 1 FROM _rot_moved mv
                           WHERE mv.user_id = u.user_id AND mv.team_key = lower(tm.key)
                             AND d.d BETWEEN mv.starts_on AND COALESCE(mv.ends_on, d.d)) THEN 'not on the team'
       END AS problem
  FROM _rot_day d
  JOIN _rot_uid u ON u.email = d.email AND u.sheet_key = d.sheet_key
  LEFT JOIN team tm ON tm.id = u.team_id
  LEFT JOIN team_schedule_shift s ON s.id = COALESCE(
       (SELECT x.id FROM team_schedule_shift x WHERE x.code = d.shift AND x.is_active),
       (SELECT x.id FROM team_schedule_shift x WHERE x.code = d.alt AND x.is_active),
       (SELECT x.id FROM team_schedule_shift x WHERE x.code = d.shift));
""")
    if args.clear_span:
        w.append(f"""
-- --clear-span: the sheet is the whole rota for its teams over its dates, so
-- every other turn on those team-days goes first, recorded as a deletion.
-- Regular hours stay; so do days outside each team's first..last date.
CREATE TEMP TABLE _rot_cleared ON COMMIT DROP AS
SELECT a.id FROM team_schedule_assignment a
  JOIN (SELECT team_key, min(d) AS lo, max(d) AS hi FROM _rot_plan WHERE team_key IS NOT NULL GROUP BY team_key) sp
    ON lower(a.team_key) = lower(sp.team_key) AND a.rota_date BETWEEN sp.lo AND sp.hi
 WHERE a.is_rotation;
INSERT INTO team_schedule_assignment_activity
  (id, created_on, created_by, assignment_id, user_id, team_key, rota_date, shift_code, action, actor_email, note)
SELECT gen_random_uuid(), now(), {t}, a.id, a.user_id, a.team_key, a.rota_date, s.code, 'DELETED', {t},
       'cleared before the rotation import'
  FROM team_schedule_assignment a JOIN team_schedule_shift s ON s.id = a.shift_id
 WHERE a.id IN (SELECT id FROM _rot_cleared);
DELETE FROM team_schedule_assignment WHERE id IN (SELECT id FROM _rot_cleared);
""")

    w.append(f"""
-- Each placeable person-day as POST /team-schedule/assignments/apply does it:
-- take off that person's turns on that team that day in the same zone or
-- overlapping the new window (recording each), then put the new turn on.
CREATE TEMP TABLE _rot_result (seq INT, displaced INT, assignment_id UUID, problem TEXT) ON COMMIT DROP;
DO $$
DECLARE
    p RECORD;
    old RECORD;
    n INT;
    new_id UUID;
BEGIN
    FOR p IN SELECT * FROM _rot_plan WHERE problem IS NULL ORDER BY seq LOOP
      -- A sub-block, so a day that clashes is undone alone -- its displaced
      -- turns come back -- and reported, instead of failing the whole import.
      BEGIN
        n := 0;
        FOR old IN
            SELECT a.id, a.user_id, a.team_key, a.rota_date, s.code
              FROM team_schedule_assignment a
              JOIN team_schedule_shift s ON s.id = a.shift_id
              JOIN team_schedule_shift ns ON ns.id = p.shift_id
             WHERE a.user_id = p.user_id AND a.rota_date = p.d AND lower(a.team_key) = lower(p.team_key)
               AND CASE
                     WHEN ns.is_escalation AND ns.zone_id IS NOT NULL THEN
                          a.is_rotation AND (a.zone_id = ns.zone_id OR tstzrange(a.starts_at, a.ends_at, '[)') && tstzrange(
                              (p.d::timestamp + make_interval(mins => ns.start_minute)) AT TIME ZONE ns.authoring_time_zone,
                              (p.d::timestamp + make_interval(mins => ns.end_minute))   AT TIME ZONE ns.authoring_time_zone, '[)'))
                     WHEN ns.zone_id IS NOT NULL THEN NOT a.is_rotation
                     ELSE TRUE
                   END
        LOOP
            INSERT INTO team_schedule_assignment_activity
              (id, created_on, created_by, assignment_id, user_id, team_key, rota_date, shift_code, action, actor_email, note)
            VALUES (gen_random_uuid(), now(), {t}, old.id, old.user_id, old.team_key, old.rota_date, old.code, 'DELETED', {t}, p.note);
            DELETE FROM team_schedule_assignment WHERE id = old.id;
            n := n + 1;
        END LOOP;

        INSERT INTO team_schedule_assignment
          (id, created_on, updated_on, created_by, updated_by, user_id, team_id, team_key,
           shift_id, zone_id, tier, rota_date, starts_at, ends_at, is_on_call, source, note)
        SELECT gen_random_uuid(), now(), now(), {t}, {t}, p.user_id, p.team_id, p.team_key,
               s.id, s.zone_id, COALESCE(p.tier::team_schedule_tier_enum, s.tier), p.d,
               (p.d::timestamp + make_interval(mins => s.start_minute)) AT TIME ZONE s.authoring_time_zone,
               (p.d::timestamp + make_interval(mins => s.end_minute))   AT TIME ZONE s.authoring_time_zone,
               s.is_on_call, 'IMPORTED', p.note
          FROM team_schedule_shift s WHERE s.id = p.shift_id
        RETURNING id INTO new_id;

        INSERT INTO team_schedule_assignment_activity
          (id, created_on, created_by, assignment_id, user_id, team_key, rota_date, shift_code, action, actor_email, note)
        VALUES (gen_random_uuid(), now(), {t}, new_id, p.user_id, p.team_key, p.d, p.shift_code, 'CREATED', {t}, p.note);
        INSERT INTO _rot_result VALUES (p.seq, n, new_id, NULL);
      EXCEPTION WHEN exclusion_violation OR unique_violation THEN
        INSERT INTO _rot_result VALUES (p.seq, 0, NULL, 'clashes with another turn they hold (another team?)');
      END;
    END LOOP;
END $$;

-- A number only for someone who has none: a number already set in the portal
-- is theirs.
CREATE TEMP TABLE _rot_phone_uid ON COMMIT DROP AS
SELECT p.email, p.phone,
       (SELECT u.id FROM "user" u WHERE lower(u.email) = p.email OR lower(u.user_name) = p.email
         ORDER BY u.is_active DESC, u.created_on LIMIT 1) AS user_id
  FROM _rot_phone p;
-- Only where the database has paging_contact (migration 0216): an older
-- schema is loaded as it is, never changed, and its phones are reported.
CREATE TEMP TABLE _rot_phone_set (user_id UUID) ON COMMIT DROP;
CREATE TEMP TABLE _rot_has_paging ON COMMIT DROP AS
SELECT to_regclass('paging_contact') IS NOT NULL AS yes;
DO $$
BEGIN
    IF (SELECT yes FROM _rot_has_paging) THEN
        EXECUTE $q$
            WITH ins AS (
              INSERT INTO paging_contact (user_id, phone, set_by)
              SELECT user_id, phone, {t} FROM _rot_phone_uid WHERE user_id IS NOT NULL
              ON CONFLICT (user_id) DO NOTHING
              RETURNING user_id)
            INSERT INTO _rot_phone_set SELECT user_id FROM ins$q$;
    END IF;
END $$;

-- What landed, and what did not and why, so nothing is dropped silently.
SELECT 'person-days placed' AS what, count(*)::text AS n FROM _rot_result WHERE problem IS NULL
UNION ALL SELECT 'turns replaced by them', COALESCE(sum(displaced), 0)::text FROM _rot_result
UNION ALL SELECT 'not placed: ' || r.problem || ' (' || p.email || ' ' || p.d || ' ' || p.shift_code || ')', '1'
  FROM _rot_result r JOIN _rot_plan p ON p.seq = r.seq WHERE r.problem IS NOT NULL
UNION ALL SELECT 'not placed: ' || problem || ' (' || email || ')', count(*)::text
  FROM _rot_plan WHERE problem IS NOT NULL GROUP BY problem, email
UNION ALL SELECT 'not placed: placed, then replaced later in the sheet', count(*)::text
  FROM _rot_result r WHERE r.problem IS NULL
   AND NOT EXISTS (SELECT 1 FROM team_schedule_assignment a WHERE a.id = r.assignment_id)
UNION ALL SELECT 'paging numbers set', count(*)::text FROM _rot_phone_set
UNION ALL SELECT 'paging numbers kept (already set)', count(*)::text
  FROM _rot_phone_uid p WHERE (SELECT yes FROM _rot_has_paging)
   AND p.user_id IS NOT NULL AND p.user_id NOT IN (SELECT user_id FROM _rot_phone_set)
UNION ALL SELECT 'paging numbers not set: no paging_contact table here', count(*)::text
  FROM _rot_phone_uid WHERE NOT (SELECT yes FROM _rot_has_paging)
UNION ALL SELECT 'paging numbers not set: unknown email (' || email || ')', '1'
  FROM _rot_phone_uid WHERE user_id IS NULL;
""")
    if args.clear_span:
        w.append("SELECT 'other turns cleared first' AS what, count(*) FROM _rot_cleared;")
    if args.add_missing_people:
        w.append(f"SELECT 'accounts created' AS what, count(*) FROM \"user\" WHERE created_by = {t}\n"
                 f"UNION ALL SELECT 'team memberships added', count(*) FROM team_member WHERE created_by = {t};")
    w.append("ROLLBACK;" if args.rollback else "COMMIT;")
    with open(args.out_sql, "w") as fh:
        fh.write("\n".join(w) + "\n")


if __name__ == "__main__":
    main()
