ALTER TABLE kb_managers DROP CONSTRAINT IF EXISTS uq_kb_managers_kb_group;
ALTER TABLE kb_managers DROP CONSTRAINT IF EXISTS chk_kb_managers_user_xor_group;

ALTER TABLE kb_managers DROP COLUMN group_id;

-- Precondition: only safe if no kb_managers row has a NULL user_id at the
-- time this rollback runs (i.e. no group-only rows exist).
ALTER TABLE kb_managers ALTER COLUMN user_id SET NOT NULL;
