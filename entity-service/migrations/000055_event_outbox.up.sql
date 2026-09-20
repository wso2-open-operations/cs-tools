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

-- Transactional outbox for row changes that drive notifications.
--
-- WHY A TRIGGER AND NOT APPLICATION CODE. The notices this feeds are
-- record-triggered: "change request updated WHERE state changes to ...". That
-- needs a before/after diff, and no writer here can produce one -- csm-sync
-- upserts blindly from ServiceNow, so it knows the new row but never the old.
-- An AFTER UPDATE trigger is the only place both versions exist at once.
--
-- The table is deliberately generic (entity_type / entity_id) even though only
-- change_request writes to it today: the drainer filters by type, so adding a
-- second producer is a trigger and nothing else.
--
-- This DDL was applied by hand to staging while the notification chain was
-- being built and never committed. It is reproduced here from the live
-- definition so a rebuilt environment gets the same thing.
CREATE TABLE IF NOT EXISTS event_outbox (
    id           BIGSERIAL PRIMARY KEY,
    entity_type  TEXT NOT NULL,
    entity_id    UUID NOT NULL,
    -- {"column": {"from": ..., "to": ...}} for the columns that differ.
    changes      JSONB NOT NULL,
    -- to_jsonb(NEW): the row as it now stands, so a consumer can read a field
    -- that did not change without going back to the table.
    snapshot     JSONB NOT NULL,
    occurred_on  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_on TIMESTAMPTZ
);

-- The drainer only ever asks for unpublished rows, and this stays small even
-- when the table does not.
CREATE INDEX IF NOT EXISTS idx_event_outbox_unpublished
    ON event_outbox (id) WHERE published_on IS NULL;

CREATE OR REPLACE FUNCTION trg_event_outbox()
RETURNS TRIGGER AS $$
DECLARE
    diff JSONB := '{}'::jsonb;
    col  TEXT;
    oldv JSONB := to_jsonb(OLD);
    newv JSONB := to_jsonb(NEW);
BEGIN
    FOR col IN SELECT jsonb_object_keys(newv) LOOP
        -- updated_on moves on every upsert regardless of content, so treating
        -- it as a change would defeat the no-op guard entirely.
        IF col <> 'updated_on' AND oldv -> col IS DISTINCT FROM newv -> col THEN
            diff := diff || jsonb_build_object(col, jsonb_build_object('from', oldv -> col, 'to', newv -> col));
        END IF;
    END LOOP;

    -- A sync pass that rewrote the row with identical values is not an event.
    IF diff = '{}'::jsonb THEN
        RETURN NULL;
    END IF;

    INSERT INTO event_outbox (entity_type, entity_id, changes, snapshot)
    VALUES (TG_TABLE_NAME, NEW.id, diff, newv);

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- THE TRIGGER IS OWNED BY THE TABLE. If change_request is ever dropped and
-- recreated, this goes with it and the notices stop firing with no error
-- anywhere -- re-run this migration's CREATE TRIGGER after any migration that
-- replaces the table.
DROP TRIGGER IF EXISTS change_request_outbox ON change_request;
CREATE TRIGGER change_request_outbox
    AFTER UPDATE ON change_request
    FOR EACH ROW EXECUTE FUNCTION trg_event_outbox();
