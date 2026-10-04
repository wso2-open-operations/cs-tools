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

-- Record-triggered cloud status: notice outage changes as they happen.
--
-- Replaces the sweep's "look at everything and diff it" with the mechanism
-- 0051 established for change_request -- an AFTER-change trigger into
-- event_outbox, drained in process. That table was built for this:
-- "the drainer filters by type, so adding a second producer is a trigger and
-- nothing else." This is that second producer, and it needs no new plumbing.
--
-- WHY THIS WORKS EVEN THOUGH csm-sync-service WRITES THESE TABLES. A trigger
-- does not care who writes; it fires on the write. 0051 makes exactly this
-- point about change_request, which is also sync-written: the sync "upserts
-- blindly ... so it knows the new row but never the old. An AFTER UPDATE
-- trigger is the only place both versions exist at once." The same is true
-- here, and more so -- the sync cannot tell us an outage just ended, because
-- it does not know what the row said before.
--
-- WHAT THIS RECOVERS THAT SWEEPING LOST. A sweep reads current state, so an
-- outage that begins and ends between two runs is only ever seen finished:
-- it yields an end event, no begin event, and never appears on the public
-- status page. The trigger sees both writes, so short outages are reported
-- like any other.
--
-- INSERT AS WELL AS UPDATE, unlike change_request's. An outage arrives
-- already carrying its begin time, so the row's creation IS the declaration.
-- 0051's function returns NULL when nothing changed, which on INSERT would
-- mean comparing against a non-existent OLD -- handled below by giving the
-- insert arm its own statement rather than reusing the diff path.

-- The diff function from 0051 handles updates for any table unchanged
-- (TG_TABLE_NAME / NEW.id), so updates need only the trigger.
DROP TRIGGER IF EXISTS outage_outbox ON outage;
CREATE TRIGGER outage_outbox
    AFTER UPDATE ON outage
    FOR EACH ROW EXECUTE FUNCTION trg_event_outbox();

-- Inserts have no OLD to diff against. Recorded with an empty change set and
-- the full snapshot: the drainer reads the snapshot to decide, and an empty
-- `changes` is how it tells "this row is new" from "these columns moved".
CREATE OR REPLACE FUNCTION trg_event_outbox_insert()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO event_outbox (entity_type, entity_id, changes, snapshot)
    VALUES (TG_TABLE_NAME, NEW.id, '{}'::jsonb, to_jsonb(NEW));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS outage_outbox_insert ON outage;
CREATE TRIGGER outage_outbox_insert
    AFTER INSERT ON outage
    FOR EACH ROW EXECUTE FUNCTION trg_event_outbox_insert();

-- The affected-CI flow's own trigger table. Adding a CI to a running outage
-- is a declaration for that CI's cloud, so it must be noticed the same way.
DROP TRIGGER IF EXISTS outage_affected_ci_outbox ON outage_affected_ci;
CREATE TRIGGER outage_affected_ci_outbox
    AFTER INSERT ON outage_affected_ci
    FOR EACH ROW EXECUTE FUNCTION trg_event_outbox_insert();

DROP TRIGGER IF EXISTS outage_affected_ci_outbox_update ON outage_affected_ci;
CREATE TRIGGER outage_affected_ci_outbox_update
    AFTER UPDATE ON outage_affected_ci
    FOR EACH ROW EXECUTE FUNCTION trg_event_outbox();

-- *** THE TRIGGERS ARE OWNED BY THEIR TABLES. *** Same warning 0051 carries,
-- and it is not theoretical -- it has already happened once, to
-- change_request on staging. While csm-sync-service still owns these tables,
-- any migration there that drops and recreates `outage` or
-- `outage_affected_ci` takes these triggers with it, and cloud status
-- notifications stop with no error anywhere. Re-run this migration after any
-- such change, and keep the reconciliation sweep (which needs no trigger) as
-- the thing that notices if they have gone.
