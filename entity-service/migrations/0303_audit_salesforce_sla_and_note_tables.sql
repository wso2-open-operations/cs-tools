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

-- Start auditing the Salesforce opportunity and invoice tables, sla_policy and
-- plg_note_revision (see 0295 to 0302). Each is registered with its primary key
-- as the record id and every column recorded, except sync_time_stamp and
-- updated_on on the four Salesforce tables: the Salesforce ingest rewrites both on
-- every run, so recording them would add two rows per record per run and no
-- business change (the same rule as account.sync_time_stamp in 0298).
--
-- The Salesforce tables are written by the Salesforce ingest, so a bulk run pays
-- the audit cost for every row it changes; the actor on those rows is whoever the
-- ingest connects as, not a person.
--
-- A table is registered only when it has no audit_config row yet, so a
-- registration made by hand, or a table an admin turned off with
-- unregister_audit, is left as it is. A table missing from the schema is skipped
-- with a notice.
DO $$
DECLARE
    t RECORD;
BEGIN
    -- Re-creating a trigger locks its table: give up after 5s rather than queue
    -- behind its writers. One transaction, so a timeout changes nothing and the
    -- migration can be run again.
    PERFORM set_config('lock_timeout', '5s', true);
    FOR t IN
        SELECT * FROM (VALUES
            ('sf_invoice',             ARRAY['sync_time_stamp', 'updated_on']),
            ('sf_opportunity',         ARRAY['sync_time_stamp', 'updated_on']),
            ('sf_opportunity_link',    ARRAY['sync_time_stamp', 'updated_on']),
            ('sf_opportunity_product', ARRAY['sync_time_stamp', 'updated_on']),
            ('sla_policy',             '{}'::text[]),
            ('plg_note_revision',      '{}'::text[])
        ) AS v(table_name, excluded)
    LOOP
        IF to_regclass(format('%I.%I', current_schema(), t.table_name)) IS NULL THEN
            RAISE NOTICE 'audit: % does not exist in schema %, not registered', t.table_name, current_schema();
        ELSIF NOT EXISTS (SELECT 1 FROM audit_config
                           WHERE schema_name = current_schema() AND table_name = t.table_name) THEN
            PERFORM register_audit(current_schema(), t.table_name, NULL, t.excluded, '{}');
        END IF;
    END LOOP;
END $$;
