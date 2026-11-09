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


-- Only internal callers may change a service request's severity.
--
-- Row-level security works on rows, not columns: service_request_update
-- (100012) lets a project member update their own service request, and that
-- cannot be narrowed to "every column but severity". The application refuses
-- an external caller's severity change first (caseService.UpdateCase); this
-- trigger is the database-level backstop for any writer that skips that check.
-- It reads app.is_internal_staff, set per transaction from the caller's
-- resolved scope (see rls.go, scoped.go): true only for an internal caller that
-- did not arrive through the customer portal's backend. app.is_internal is not
-- used because it stays true for staff acting through that portal, whom the
-- service treats as customers for this field.
--
-- UPDATE only. A customer's own create goes through insertAsCheckedProjectMember,
-- which marks the insert transaction internal (so the extension policy can see
-- the new work item); the create path therefore ignores an external caller's
-- severity in Go and lets the column default apply. A writer with no
-- app.is_internal_staff setting at all (csm-sync-service) never changes
-- severity, so it is unaffected: the check fires only when the value actually
-- changes. Deploy order: the application must be running the build that sets
-- app.is_internal_staff before this migration is applied, or an internal
-- severity change is refused until it is.
--
-- CREATE OR REPLACE and DROP TRIGGER IF EXISTS make this safe to re-run.
BEGIN;

CREATE OR REPLACE FUNCTION service_request_severity_internal_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.severity IS DISTINCT FROM OLD.severity
       AND COALESCE(current_setting('app.is_internal_staff', true), '') <> 'true' THEN
        RAISE EXCEPTION 'service request severity can only be changed by internal users'
            USING ERRCODE = '42501';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS service_request_severity_internal_only ON service_request;
CREATE TRIGGER service_request_severity_internal_only
    BEFORE UPDATE OF severity ON service_request
    FOR EACH ROW EXECUTE FUNCTION service_request_severity_internal_only();

COMMIT;
