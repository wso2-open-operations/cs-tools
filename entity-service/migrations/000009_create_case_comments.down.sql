DROP TABLE IF EXISTS case_comments;
DROP FUNCTION IF EXISTS check_activity_creator();
DROP FUNCTION IF EXISTS check_work_note_creator();
DROP FUNCTION IF EXISTS assert_user_type(TEXT, user_type_enum[], TEXT);
DROP TYPE IF EXISTS comment_type_enum;
