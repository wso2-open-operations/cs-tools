-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.

-- Stop at the first error instead of carrying on with a half-applied script.
\set ON_ERROR_STOP on

-- STEP 1 of the separate-application-role setup (docs/rls-database-roles.md): give the APPLICATION its own login role.
-- Run ONCE per database, as a role that can create roles (e.g. the server admin). Safe to re-run: an existing
-- role keeps its password; grants are re-applied.
--
--   psql -X -v app_password="$APP_DB_PASSWORD" -v schema=csmpd_stg_user -v owner_role=csmpd_stg_user \
--        -v app_role=csm_entity_app -f scripts/rls/create_app_role.sql
--
-- Why: row-level security binds a role that is NOT the table owner, so the application (entity-service) should
-- connect as this role. People, the sync/migration services and DBeaver keep the owner login. Nothing changes for
-- anyone until entity-service is switched to this role AND scripts/rls/owner_exempt_no_force.sql is run.
-- This script does not change any table, policy or existing role.
\if :{?app_password}\else \echo 'ERROR: pass -v app_password=...'  \quit \endif
\if :{?schema}\else \set schema csmpd_stg_user \endif
\if :{?owner_role}\else \set owner_role csmpd_stg_user \endif
\if :{?app_role}\else \set app_role csm_entity_app \endif
SELECT current_database() AS dbname \gset

SELECT format('CREATE ROLE %I LOGIN PASSWORD %L NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE', :'app_role', :'app_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'app_role') \gexec

GRANT CONNECT ON DATABASE :"dbname" TO :"app_role";
GRANT USAGE ON SCHEMA :"schema" TO :"app_role";
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA :"schema" TO :"app_role";
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA :"schema" TO :"app_role";
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA :"schema" TO :"app_role";
-- tables, sequences and functions the owner creates later (new migrations) are granted automatically
ALTER DEFAULT PRIVILEGES FOR ROLE :"owner_role" IN SCHEMA :"schema" GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO :"app_role";
ALTER DEFAULT PRIVILEGES FOR ROLE :"owner_role" IN SCHEMA :"schema" GRANT USAGE, SELECT ON SEQUENCES TO :"app_role";
ALTER DEFAULT PRIVILEGES FOR ROLE :"owner_role" IN SCHEMA :"schema" GRANT EXECUTE ON FUNCTIONS TO :"app_role";
-- the app relies on its schema being on the search path; the default "$user" would point at a schema named after the role
ALTER ROLE :"app_role" SET search_path = :"schema", public;

SELECT rolname, rolsuper, rolbypassrls, rolcanlogin FROM pg_roles WHERE rolname = :'app_role';
