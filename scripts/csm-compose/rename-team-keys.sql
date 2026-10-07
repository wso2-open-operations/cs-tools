-- Rename the team catalogue keys to the long convention:
--   <name>_abt_cre_team / <name>_abt_sre_team for the ABTs,
--   <name>_cre_team     for the teams that hold no ABT rotation.
--
-- Safe to run against dev or a local stack, and safe to run twice: the
-- mapping is keyed on the OLD name, so a key that has already been renamed
-- matches nothing and is left alone. A database that never had one of these
-- teams simply reports zero for it.
--
-- Run it with ON_ERROR_STOP so a failed assertion actually aborts:
--   psql -v ON_ERROR_STOP=1 -f rename-team-keys.sql
--
-- Table names are unqualified on purpose, so the same file works against the
-- local stack (public) and dev (csmpd_stg_user). Connect as a role whose
-- search_path reaches the right schema; do not add a SET search_path here.
--
-- DO NOT TRUST ON UPDATE CASCADE TO CARRY THE ROTA ROWS.
--
-- An earlier version of this script claimed it would, and that was wrong in
-- two separate ways:
--
--   * team_schedule_assignment_activity and team_schedule_absence_activity
--     both store team_key and have NO foreign key to team(key) anywhere --
--     not on dev, not locally. A rename leaves them pointing at a key that
--     no longer exists. This is not hypothetical: it is what happened on
--     dev, which carried 23 orphaned activity rows on short keys while
--     every other table had moved to the long ones.
--   * team_schedule_assignment and team_schedule_absence DO have the
--     cascading FK on dev, but NOT on a local compose stack, whose
--     team_schedule_* tables are built by migrate-and-seed.sh's old
--     *.up.sql path rather than 0153_team_schedule_tables.sql. Against
--     local, the same rename orphans every rota row instead.
--
-- So this script now updates all four child tables explicitly, after the
-- parent. Where the cascade exists it has already done the work and the
-- explicit pass matches zero rows, which is harmless. Where it does not,
-- the explicit pass is the only thing that keeps the data consistent.
-- The legacy schedule_assignment / schedule_absence tables are left alone:
-- they carry their own cascading FK and are not the live rota.
--
-- Everything OUTSIDE the database that hardcodes a key still has to be
-- changed by hand in the same release: CSM_TEAM_REGISTRY, the compose seed,
-- the ABT importer, the escalation config. Note that CSM_TEAM_REGISTRY rows
-- are key|displayName|family|creGroupId|sreGroupId -- preserve the group id
-- fields when you rewrite the keys, or the case-search team filters break.

BEGIN;

CREATE TEMP TABLE _team_key_rename (old_key TEXT PRIMARY KEY, new_key TEXT NOT NULL)
ON COMMIT DROP;

INSERT INTO _team_key_rename (old_key, new_key) VALUES
    ('castor',   'castor_abt_cre_team'),
    ('draco',    'draco_abt_cre_team'),
    ('vega',     'vega_abt_cre_team'),
    ('sirius',   'sirius_abt_cre_team'),
    ('atlas',    'atlas_abt_cre_team'),
    ('phoenix',  'phoenix_abt_cre_team'),
    ('rigel',    'rigel_abt_cre_team'),
    ('apollo',   'apollo_abt_sre_team'),
    ('artemis',  'artemis_abt_sre_team'),
    ('americas', 'americas_cre_team'),
    ('australia','australia_cre_team'),
    ('migration','migration_cre_team');

-- The four live tables that carry team_key. Kept in one place so the
-- repair pass and the final assertion below cannot drift apart.
CREATE TEMP TABLE _child_tables (tbl TEXT PRIMARY KEY) ON COMMIT DROP;
INSERT INTO _child_tables VALUES
    ('team_schedule_assignment'),
    ('team_schedule_absence'),
    ('team_schedule_assignment_activity'),
    ('team_schedule_absence_activity');

-- Which child tables actually have a foreign key on team_key, so the
-- operator can see which ones depended on an explicit repair rather than a
-- cascade. A table listed with has_fk = false is one the old script broke.
SELECT c.tbl,
       to_regclass(c.tbl) IS NOT NULL AS present,
       EXISTS (
           SELECT 1
             FROM information_schema.table_constraints tc
             JOIN information_schema.key_column_usage kcu
               ON kcu.constraint_name = tc.constraint_name
              AND kcu.table_name = tc.table_name
            WHERE tc.constraint_type = 'FOREIGN KEY'
              AND tc.table_name = c.tbl
              AND kcu.column_name = 'team_key'
       ) AS has_team_key_fk
  FROM _child_tables c ORDER BY c.tbl;

-- What is about to change, and what each key carries with it.
SELECT r.old_key,
       r.new_key,
       (SELECT count(*) FROM team t WHERE lower(t.key) = r.old_key) AS team_rows
  FROM _team_key_rename r
 ORDER BY r.old_key;

-- A destination key that another team already holds is not something this
-- script can resolve: whether those two teams are the same team under two
-- names, or two genuinely different ones, is a decision about the catalogue,
-- not a rename. Such a mapping is skipped and reported rather than aborting
-- the run, so the unambiguous renames still land and the one real question
-- is left visible instead of buried in a failed transaction.
CREATE TEMP TABLE _blocked ON COMMIT DROP AS
SELECT r.old_key, r.new_key, t.id AS held_by_team_id, t.name AS held_by_team
  FROM _team_key_rename r
  JOIN team t ON lower(t.key) = r.new_key
 WHERE EXISTS (SELECT 1 FROM team s WHERE lower(s.key) = r.old_key)
   AND NOT EXISTS (SELECT 1 FROM team o WHERE lower(o.key) = r.old_key AND o.id = t.id);

SELECT old_key AS not_renamed, new_key AS because_this_key_is_held_by, held_by_team
  FROM _blocked ORDER BY old_key;

-- 1. The parent. Where a cascading FK exists this also moves the rota rows.
UPDATE team t
   SET key = r.new_key,
       updated_on = NOW(),
       updated_by = 'rename-team-keys'
  FROM _team_key_rename r
 WHERE lower(t.key) = r.old_key
   AND NOT EXISTS (SELECT 1 FROM _blocked b WHERE b.old_key = r.old_key);

-- 2. The children, explicitly. A no-op wherever the cascade already ran;
--    the only thing that repairs the rest. This also repairs a database
--    renamed by the earlier, broken version of this script -- the parent
--    update above matches nothing there, and this pass still fixes the
--    rows it left behind.
DO $repair$
DECLARE
    t      TEXT;
    n      BIGINT;
    total  BIGINT := 0;
BEGIN
    FOR t IN SELECT tbl FROM _child_tables ORDER BY tbl LOOP
        IF to_regclass(t) IS NULL THEN
            RAISE NOTICE 'skip % -- not present in this database', t;
            CONTINUE;
        END IF;
        EXECUTE format(
            'UPDATE %I c SET team_key = r.new_key
               FROM _team_key_rename r
              WHERE lower(c.team_key) = r.old_key
                AND NOT EXISTS (SELECT 1 FROM _blocked b WHERE b.old_key = r.old_key)', t);
        GET DIAGNOSTICS n = ROW_COUNT;
        total := total + n;
        RAISE NOTICE 'repaired % row(s) in %', n, t;
    END LOOP;
    RAISE NOTICE 'child rows repaired in total: %', total;
END
$repair$;

-- 3. Display names to title case: 'draco' -> 'Draco', 'americas' -> 'Americas'.
--
-- This is the name, not the key: the rota, the registry and the escalation
-- config all resolve teams by key, which stays lower case, so nothing that
-- schedules anybody is affected by this.
--
-- It is also a correction rather than a preference. POST /users/search
-- matches team.name exactly (user_repo.go), and the portal feeds it the
-- DISPLAY name out of CSM_TEAM_REGISTRY -- which has always been 'Draco'.
-- Against a catalogue holding 'draco' that filter silently returns nobody.
--
-- Only a name that is a single all-lower-case word is touched. That takes the
-- ABTs, americas and australia, and deliberately leaves 'Devops Approval' and
-- 'CAB Approval' alone -- they are already correct, and GroupMemberEmails
-- looks them up by this exact string. Slug-style names like
-- customer_onboarding_team are left too: initcap would make
-- 'Customer_Onboarding_Team' of them, which is not an improvement.
UPDATE team
   SET name = initcap(name),
       updated_on = NOW(),
       updated_by = 'rename-team-keys'
 WHERE name ~ '^[a-z]+$'
   AND (lower(type) LIKE 'cre%' OR lower(type) LIKE 'sre%')
   -- only the teams this script renames, including ones renamed on an earlier
   -- run: an unrelated team with a one-word lower-case name is not ours to
   -- touch, and its registry entry may match it by that exact name
   AND EXISTS (SELECT 1 FROM _team_key_rename r
                WHERE lower(team.key) IN (r.old_key, r.new_key));

-- 4. Assert before committing. The failure this guards against is a child
--    row left on a key the mapping was supposed to move -- exactly the state
--    the old script committed silently. Rolling back is the right outcome:
--    a half-renamed catalogue resolves nothing and is harder to diagnose
--    than a rename that refused to happen.
DO $verify$
DECLARE
    t         TEXT;
    stranded  BIGINT;
    bad       BIGINT := 0;
BEGIN
    FOR t IN SELECT tbl FROM _child_tables ORDER BY tbl LOOP
        IF to_regclass(t) IS NULL THEN CONTINUE; END IF;
        -- A blocked mapping was skipped on purpose (step 2), so its rows are
        -- expected to stay put: reported, never a reason to roll back the
        -- unambiguous renames.
        EXECUTE format(
            'SELECT count(*) FROM %I c
               JOIN _team_key_rename r ON lower(c.team_key) = r.old_key
              WHERE NOT EXISTS (SELECT 1 FROM _blocked b WHERE b.old_key = r.old_key)', t)
           INTO stranded;
        IF stranded > 0 THEN
            RAISE WARNING '% still holds % row(s) on an old key', t, stranded;
            bad := bad + stranded;
        END IF;
        EXECUTE format(
            'SELECT count(*) FROM %I c
               JOIN _blocked b ON lower(c.team_key) = b.old_key', t)
           INTO stranded;
        IF stranded > 0 THEN
            RAISE NOTICE '% keeps % row(s) on a blocked key (not renamed, see above)', t, stranded;
        END IF;
    END LOOP;
    IF bad > 0 THEN
        RAISE EXCEPTION 'rename incomplete: % child row(s) still on old keys', bad;
    END IF;
END
$verify$;

-- 5. The catalogue as it now stands, with the live rota counts beside it.
--    Guarded like the repair and the assertion: a schedule table this
--    database does not have is reported as NULL, rather than a query that
--    aborts the transaction and rolls the renames back.
CREATE TEMP TABLE _rota_counts (team_key TEXT, assignments BIGINT, absences BIGINT) ON COMMIT DROP;
INSERT INTO _rota_counts (team_key) SELECT key FROM team;
DO $counts$
BEGIN
    IF to_regclass('team_schedule_assignment') IS NOT NULL THEN
        UPDATE _rota_counts r SET assignments =
            (SELECT count(*) FROM team_schedule_assignment a WHERE a.team_key = r.team_key);
    END IF;
    IF to_regclass('team_schedule_absence') IS NOT NULL THEN
        UPDATE _rota_counts r SET absences =
            (SELECT count(*) FROM team_schedule_absence b WHERE b.team_key = r.team_key);
    END IF;
END
$counts$;

SELECT t.key,
       t.name,
       t.type,
       c.assignments,
       c.absences
  FROM team t
  LEFT JOIN _rota_counts c ON c.team_key = t.key
 WHERE lower(t.type) LIKE 'cre%' OR lower(t.type) LIKE 'sre%'
 ORDER BY t.type, t.key;

COMMIT;
