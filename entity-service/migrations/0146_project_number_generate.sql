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

-- Gives every project a number (CSPRJ0008211 format). A project ServiceNow
-- sends without one gets the next number from a sequence that continues after
-- the highest CSPRJ number already stored. The attachment folders are named
-- after project.number, so a project must not be left without one.

CREATE SEQUENCE IF NOT EXISTS project_number_seq;

-- Continue after the highest existing CSPRJ number (never move backwards).
DO $$
DECLARE
    highest bigint;
    current bigint;
BEGIN
    SELECT max(substring(number FROM 6)::bigint) INTO highest
    FROM project WHERE number ~ '^CSPRJ[0-9]{1,15}$';
    SELECT CASE WHEN is_called THEN last_value ELSE 0 END INTO current FROM project_number_seq;
    highest := GREATEST(COALESCE(highest, 0), current);
    IF highest > 0 THEN
        PERFORM setval('project_number_seq', highest, true);
    END IF;
END $$;

CREATE OR REPLACE FUNCTION project_number_next() RETURNS text
    LANGUAGE sql AS
$$ SELECT 'CSPRJ' || lpad(nextval('project_number_seq')::text, 7, '0') $$;

-- The project mapping lists number, so a sync of a project that ServiceNow
-- sends without one would write NULL over the number we generated. This keeps
-- the stored number, and numbers a project that has none.
CREATE OR REPLACE FUNCTION project_number_fill() RETURNS trigger
    LANGUAGE plpgsql AS
$$
DECLARE
    existing text;
BEGIN
    IF NEW.number IS NOT NULL AND btrim(NEW.number) <> '' THEN
        RETURN NEW;
    END IF;
    IF TG_OP = 'UPDATE' THEN
        existing := OLD.number;
    ELSE
        -- An upsert reaches this trigger as an insert before it turns into an update.
        SELECT p.number INTO existing FROM project p WHERE p.id = NEW.id;
    END IF;
    NEW.number := COALESCE(NULLIF(btrim(existing), ''), project_number_next());
    RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS trg_project_number_fill ON project;
CREATE TRIGGER trg_project_number_fill
    BEFORE INSERT OR UPDATE ON project
    FOR EACH ROW EXECUTE FUNCTION project_number_fill();

-- Number the projects that have none, oldest first (the order is best effort).
UPDATE project SET number = NULL WHERE btrim(number) = '';
UPDATE project SET number = project_number_next()
WHERE id IN (SELECT id FROM project WHERE number IS NULL ORDER BY created_on, id);
