DROP TABLE IF EXISTS cases;
DROP TYPE IF EXISTS case_state_enum;
DROP TYPE IF EXISTS case_priority_enum;
DROP TYPE IF EXISTS case_issue_type_enum;
DROP FUNCTION IF EXISTS check_case_deployment_belongs_to_project();
DROP FUNCTION IF EXISTS check_case_deployed_product_belongs_to_deployment();
DROP FUNCTION IF EXISTS check_case_catastrophic_priority();
DROP EXTENSION IF EXISTS pg_trgm;
