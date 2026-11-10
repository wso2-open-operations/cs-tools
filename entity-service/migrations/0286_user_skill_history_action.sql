-- Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
--
-- WSO2 LLC. licenses this file to you under the Apache License,
-- Version 2.0 (the "License"); you may not use this file except
-- in compliance with the License.
-- You may obtain a copy of the License at
--
-- http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing,
-- software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
-- KIND, either express or implied.  See the License for the
-- specific language governing permissions and limitations
-- under the License.

-- Skill management revamp (M4): history says what happened (ADDED,
-- LEVEL_CHANGED, REMOVED, READDED) instead of leaving it to be inferred from
-- which level is NULL, keeps the skill's name and category path as they were,
-- links a change made by an approved request, survives the skill being
-- deleted (RESTRICT instead of CASCADE), and cannot be edited.
-- Run scripts/skill_data_checks.sh (B4, B5) first: a row with neither level stays
-- unclassified (action NULL).

DO $$ BEGIN
    CREATE TYPE skill_history_action_enum AS ENUM ('ADDED', 'LEVEL_CHANGED', 'REMOVED', 'READDED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE user_skill_history
    ADD COLUMN IF NOT EXISTS action                 skill_history_action_enum,
    ADD COLUMN IF NOT EXISTS skill_name_snapshot    VARCHAR(255),
    ADD COLUMN IF NOT EXISTS category_path_snapshot VARCHAR(1000),
    ADD COLUMN IF NOT EXISTS skill_request_id       UUID REFERENCES skill_request(id);

-- Backfill. An add with an earlier row for the same pair is a re-add.
WITH h AS (
    SELECT id, level_from_id, level_to_id,
           row_number() OVER (PARTITION BY user_id, skill_id ORDER BY created_on, id) AS seq
      FROM user_skill_history
     WHERE action IS NULL
)
UPDATE user_skill_history u
   SET action = CASE
         WHEN h.level_from_id IS NULL AND h.level_to_id IS NULL THEN NULL
         WHEN h.level_from_id IS NULL AND h.seq = 1           THEN 'ADDED'::skill_history_action_enum
         WHEN h.level_from_id IS NULL                          THEN 'READDED'::skill_history_action_enum
         WHEN h.level_to_id IS NULL                            THEN 'REMOVED'::skill_history_action_enum
         ELSE 'LEVEL_CHANGED'::skill_history_action_enum END
  FROM h
 WHERE u.id = h.id;

-- Today's names are the best available for the old rows.
UPDATE user_skill_history u
   SET skill_name_snapshot = s.name,
       category_path_snapshot = (
         SELECT string_agg(COALESCE(c.category_path, c.name), '; ' ORDER BY COALESCE(c.category_path, c.name))
           FROM skill_category_link l JOIN skill_category c ON c.id = l.category_id
          WHERE l.skill_id = s.id)
  FROM skill s
 WHERE s.id = u.skill_id AND u.skill_name_snapshot IS NULL;

ALTER TABLE user_skill_history DROP CONSTRAINT IF EXISTS user_skill_history_skill_id_fkey;
ALTER TABLE user_skill_history ADD CONSTRAINT user_skill_history_skill_id_fkey
    FOREIGN KEY (skill_id) REFERENCES skill(id) ON DELETE RESTRICT;
ALTER TABLE user_skill_history DROP CONSTRAINT IF EXISTS user_skill_history_level_from_id_fkey;
ALTER TABLE user_skill_history ADD CONSTRAINT user_skill_history_level_from_id_fkey
    FOREIGN KEY (level_from_id) REFERENCES skill_level(id) ON DELETE RESTRICT;
ALTER TABLE user_skill_history DROP CONSTRAINT IF EXISTS user_skill_history_level_to_id_fkey;
ALTER TABLE user_skill_history ADD CONSTRAINT user_skill_history_level_to_id_fkey
    FOREIGN KEY (level_to_id) REFERENCES skill_level(id) ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_user_skill_history_user_skill_on
    ON user_skill_history (user_id, skill_id, created_on DESC);

-- Append-only. The one delete allowed is the cascade from a deleted user: the
-- RI action runs after the user row is gone, so it is no longer visible.
CREATE OR REPLACE FUNCTION user_skill_history_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' AND NOT EXISTS (SELECT 1 FROM "user" WHERE id = OLD.user_id) THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'user_skill_history is append-only' USING ERRCODE = 'insufficient_privilege';
END $$;

DROP TRIGGER IF EXISTS trg_user_skill_history_append_only ON user_skill_history;
CREATE TRIGGER trg_user_skill_history_append_only
    BEFORE UPDATE OR DELETE ON user_skill_history
    FOR EACH ROW EXECUTE FUNCTION user_skill_history_append_only();
