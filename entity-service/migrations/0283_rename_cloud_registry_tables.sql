-- 0283: rename the cloud registry tables created by 0282.
--
--   cloud_status_cloud            -> cloud_registry
--   cloud_status_cloud_region     -> cloud_registry_region
--   cloud_status_offering_weight  -> cloud_registry_offering_weight
--
-- cloud_status_cloud repeated itself and the three tables did not share a
-- prefix. The constraints and their backing indexes are renamed with them so
-- the old name does not survive in \d output or error messages.
--
-- Data, columns and foreign keys are untouched: ALTER ... RENAME only changes
-- the catalog name. An environment seeded before this migration keeps its rows.
-- Seed scripts and queries written against the old names must switch to the new
-- ones.
--
-- Idempotent: each rename is skipped when the old name is gone or the new name
-- is taken.
-- One transaction with a short lock timeout, as the sibling migrations do.

BEGIN;
SET LOCAL lock_timeout = '5s';

DO $$
DECLARE
    t RECORD;
    c RECORD;
BEGIN
    FOR t IN
        SELECT * FROM (VALUES
            ('cloud_status_cloud',           'cloud_registry'),
            ('cloud_status_cloud_region',    'cloud_registry_region'),
            ('cloud_status_offering_weight', 'cloud_registry_offering_weight')
        ) AS v(old_name, new_name)
    LOOP
        IF to_regclass(t.old_name) IS NOT NULL AND to_regclass(t.new_name) IS NULL THEN
            EXECUTE format('ALTER TABLE %I RENAME TO %I', t.old_name, t.new_name);

            -- Constraints keep their old names after a table rename. Rename every
            -- one that carries the old table name as its prefix; renaming a
            -- primary-key or unique constraint renames its index too.
            FOR c IN
                SELECT conname
                  FROM pg_constraint
                 WHERE conrelid = to_regclass(t.new_name)
                   AND conname LIKE t.old_name || '\_%' ESCAPE '\'
            LOOP
                EXECUTE format('ALTER TABLE %I RENAME CONSTRAINT %I TO %I',
                    t.new_name, c.conname,
                    t.new_name || substr(c.conname, length(t.old_name) + 1));
            END LOOP;
        END IF;
    END LOOP;
END $$;

COMMIT;
