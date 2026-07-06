CREATE TYPE comment_type_enum AS ENUM (
  'work_note',
  'comment',
  'activity'
);

CREATE TABLE case_comments (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  case_id    UUID NOT NULL REFERENCES cases(id),
  type       comment_type_enum NOT NULL,
  content    TEXT NOT NULL,
  created_by TEXT NOT NULL REFERENCES users(id),
  created_at TIMESTAMP DEFAULT NOW()
);

-- This function checks if a user has one of the allowed user types. It can be used in triggers or other functions to enforce access control based on user type.

CREATE OR REPLACE FUNCTION assert_user_type_enum(
  p_user_id   TEXT,
  p_allowed   user_type_enum[],
  p_context   TEXT
)
RETURNS VOID AS $$
DECLARE
  v_user_type_enum user_type_enum;
BEGIN
  SELECT user_type_enum INTO v_user_type_enum
  FROM users WHERE id = p_user_id;

  IF v_user_type_enum != ALL(p_allowed) THEN
    RAISE EXCEPTION '% requires user type to be one of (%). user_id % has type %.',
      p_context,
      array_to_string(p_allowed::text[], ', '),
      p_user_id,
      v_user_type_enum;
  END IF;
END;
$$ LANGUAGE plpgsql;

-- Triggers to enforce that only internal users can create work notes, and only internal or system users can create activity comments.

CREATE OR REPLACE FUNCTION check_work_note_creator()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.type = 'work_note' THEN
    PERFORM assert_user_type_enum(
      NEW.created_by,
      ARRAY['internal']::user_type_enum[],
      'work_note comment'
    );
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION check_activity_creator()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.type = 'activity' THEN
    PERFORM assert_user_type_enum(
      NEW.created_by,
      ARRAY['internal', 'system']::user_type_enum[],
      'activity comment'
    );
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers to enforce the creator restrictions on case_comments.

CREATE TRIGGER trg_check_work_note_creator
  BEFORE INSERT OR UPDATE ON case_comments
  FOR EACH ROW EXECUTE FUNCTION check_work_note_creator();

CREATE TRIGGER trg_check_activity_creator
  BEFORE INSERT OR UPDATE ON case_comments
  FOR EACH ROW EXECUTE FUNCTION check_activity_creator();

-- Indexes to optimize queries on case_comments by case_id, created_by, comment_type, and combinations of these fields.

CREATE INDEX idx_case_comments_case_id    ON case_comments(case_id);
CREATE INDEX idx_case_comments_created_by ON case_comments(created_by);
CREATE INDEX idx_case_comments_type       ON case_comments(type);
CREATE INDEX idx_case_comments_case_time  ON case_comments(case_id, created_at DESC);
CREATE INDEX idx_case_comments_case_type  ON case_comments(case_id, type);
