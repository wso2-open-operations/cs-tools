-- Renames the local "users" table to match the real shared-DB schema's
-- "user" (singular, quoted -- reserved word), same gap category as Step 1's
-- KB tables (migration 000027): entity-service's code was updated during
-- the Sep 20 merge to match the real DB, but the local users table was
-- never migrated to match since local login testing wasn't revisited then.
-- FK constraints referencing users(id) are preserved automatically across
-- the rename (case_attachments, case_comments, cases, deployments,
-- accounts) -- Postgres keeps them intact, just repoints internally.
ALTER TABLE users RENAME TO "user";
ALTER TABLE "user" RENAME COLUMN created_at TO created_on;
ALTER TABLE "user" RENAME COLUMN updated_at TO updated_on;
