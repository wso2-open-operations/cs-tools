-- Allow a group (external groups concept -- no local table to reference, so
-- no FK) to hold the same review/approve/reject/edit capability a
-- kb_managers user-row already grants. Exactly one of user_id/group_id must
-- be set per row; both users and groups remain many-to-many against a KB.
ALTER TABLE kb_managers ALTER COLUMN user_id DROP NOT NULL;

ALTER TABLE kb_managers ADD COLUMN group_id TEXT NULL;

ALTER TABLE kb_managers ADD CONSTRAINT chk_kb_managers_user_xor_group
  CHECK (
    (user_id IS NOT NULL AND group_id IS NULL)
    OR (user_id IS NULL AND group_id IS NOT NULL)
  );

ALTER TABLE kb_managers ADD CONSTRAINT uq_kb_managers_kb_group UNIQUE (knowledge_base_id, group_id);

-- This migration does not populate any group rows; group_id stays NULL for
-- every row until a future, separate effort backfills it.
