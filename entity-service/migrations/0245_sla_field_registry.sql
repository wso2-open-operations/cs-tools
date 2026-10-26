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

-- The SLA module's field registry (SLA SPEC.md R5, R16): which work item
-- types may carry SLAs, and which of their fields an admin may reference in
-- a predicate condition (the "all"/"any"/"not" trees sla_definition stores --
-- see migration 0247). Three tables, two different owners:
--
--   sla_work_item_type     developer-seeded (via this migration), and
--   sla_field_catalog_entry  admin-read-only through the API: "which types
--                          and fields CAN be used" is a catalog of what this
--                          service's own code knows how to read (R16's "a
--                          catalog of where each type stores its field
--                          values... the UI only enables entries from that
--                          catalog"), not something an admin may invent.
--
--   sla_field_enablement   the one admin-writable row per catalog entry:
--                          "which of the catalog's fields are actually
--                          offered to the rule builder" (R16).
--
-- work_item_type reuses work_item_type_enum (migration 0021) rather than a
-- new vocabulary, so "case" here can never drift from what work_item.type
-- itself allows -- a new work item type becomes SLA-capable by adding one
-- sla_work_item_type row and a handful of sla_field_catalog_entry rows
-- (plus a resolver function in entity-service's own Go code, which is the
-- "storage mapping" R16 says the UI never defines), never by changing the
-- generic matching/lifecycle code (R1, R16).
--
-- storage_kind/storage_ref: most fields are a 'direct_column' read straight
-- off the type's own table (storage_ref is a resolver-registry key such as
-- '"case".severity'); a few are a 'derived_signal' the application computes
-- and keeps current on work_item_sla_signal (migration 0250) *before* any
-- condition is evaluated, so a condition can react to something like "the
-- author of the last comment" (R6/R18-R21's "fields of the work item
-- itself... related-record lookups are not supported" -- a derived signal
-- field is a field of the work item, not a join). lastCommentAuthorRole is
-- exactly this: reuses the same support-engineer-authorship check
-- case_service.go/sn_case_service.go already compute for the old SLA
-- engine's CompleteResponseClock, just surfaced as an ordinary enum field a
-- stop condition can use with the changed_to operator (R7), e.g.
-- {"field": "lastCommentAuthorRole", "op": "changed_to", "value":
-- "SUPPORT_ENGINEER"} to express "the first engineer reply stops the
-- response clock" with no special-cased condition type.
BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS sla_work_item_type (
    work_item_type               work_item_type_enum PRIMARY KEY,
    enabled                      BOOLEAN NOT NULL DEFAULT FALSE,
    -- Which catalog entry is "the state field" (R5: "registers its states
    -- and the state field"). Nullable here, set below once the catalog rows
    -- it references exist -- the two tables reference each other.
    state_field_catalog_entry_id UUID,
    created_on                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_on                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by                   VARCHAR(255) NOT NULL DEFAULT 'migration:0245',
    updated_by                   VARCHAR(255) NOT NULL DEFAULT 'migration:0245'
);

COMMENT ON TABLE sla_work_item_type IS
    'Which work item types may carry SLAs (SLA SPEC.md R1, R3, R16). Developer-seeded; enabled is the only field an admin may later flip.';

CREATE TABLE IF NOT EXISTS sla_field_catalog_entry (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    work_item_type work_item_type_enum NOT NULL REFERENCES sla_work_item_type(work_item_type),
    field_key      VARCHAR(100) NOT NULL,
    data_type      VARCHAR(20) NOT NULL,
    -- Allowed values for an 'enum' field, as the rule builder's own value
    -- picker should offer them -- not validated against another enum type
    -- at the DB level, since several fields here draw from different real
    -- Postgres enums (case_severity_enum, incident_state_enum, ...) and a
    -- few (the derived signals) have no backing Postgres enum at all.
    enum_values    JSONB,
    display_label  VARCHAR(255) NOT NULL,
    storage_kind   VARCHAR(20) NOT NULL,
    -- A key into entity-service's own Go resolver registry
    -- (internal/service/sla_field_resolvers.go) -- never a raw column name a
    -- predicate could turn into SQL directly; the resolver is what actually
    -- reads the value, so a storage_ref with no matching Go entry is a
    -- startup-time or publish-time error, never a runtime SQL injection
    -- surface.
    storage_ref    VARCHAR(255) NOT NULL,
    is_state_field BOOLEAN NOT NULL DEFAULT FALSE,
    created_on     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_on     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT sla_field_catalog_entry_data_type_check
        CHECK (data_type IN ('string', 'enum', 'boolean', 'datetime', 'integer')),
    CONSTRAINT sla_field_catalog_entry_storage_kind_check
        CHECK (storage_kind IN ('direct_column', 'derived_signal')),
    CONSTRAINT sla_field_catalog_entry_work_item_type_field_key_uq
        UNIQUE (work_item_type, field_key)
);

COMMENT ON TABLE sla_field_catalog_entry IS
    'Developer-seeded catalog of SLA-condition-eligible fields per work item type (R5, R16). Never admin-writable -- only sla_field_enablement is.';

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sla_work_item_type_state_field_fkey') THEN
        ALTER TABLE sla_work_item_type
            ADD CONSTRAINT sla_work_item_type_state_field_fkey
            FOREIGN KEY (state_field_catalog_entry_id) REFERENCES sla_field_catalog_entry(id);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS sla_field_enablement (
    field_catalog_entry_id UUID PRIMARY KEY REFERENCES sla_field_catalog_entry(id),
    enabled                BOOLEAN NOT NULL DEFAULT FALSE,
    updated_on             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by             VARCHAR(255) NOT NULL DEFAULT 'migration:0245'
);

COMMENT ON TABLE sla_field_enablement IS
    'Admin on/off switch per sla_field_catalog_entry row -- "which of the catalog''s fields are offered to the rule builder" (R16). The only admin-writable table of the three in this file.';

-- Seed: enable case and incident for SLAs (R3's v1 scope).
INSERT INTO sla_work_item_type (work_item_type, enabled)
VALUES ('CASE', TRUE), ('INCIDENT', TRUE)
ON CONFLICT (work_item_type) DO NOTHING;

-- Seed: the catalog entries a first, real SLA definition needs. This is a
-- starting set, not exhaustive -- a developer adds a row here (plus, for a
-- derived_signal, a resolver-registry entry) whenever a new field needs to
-- be reachable from a condition; nothing about the engine itself changes.
INSERT INTO sla_field_catalog_entry
    (work_item_type, field_key, data_type, enum_values, display_label, storage_kind, storage_ref, is_state_field)
VALUES
    ('CASE', 'state', 'enum',
     '["OPEN","WORK_IN_PROGRESS","AWAITING_INFO","SOLUTION_PROPOSED","WAITING_ON_WSO2","REOPENED","CLOSED"]',
     'State', 'direct_column', 'case.state', TRUE),
    ('CASE', 'severity', 'enum', '["S0","S1","S2","S3","S4"]',
     'Severity', 'direct_column', 'case.severity', FALSE),
    ('CASE', 'issueType', 'enum',
     '["TOTAL_OUTAGE","PARTIAL_OUTAGE","PERFORMANCE_DEGRADATION","QUESTION","SECURITY_OR_COMPLIANCE","ERROR"]',
     'Issue type', 'direct_column', 'case.issueType', FALSE),
    ('CASE', 'workState', 'enum', '["ONGOING","PAUSED"]',
     'Work state', 'direct_column', 'case.workState', FALSE),
    ('CASE', 'escalationLevel', 'enum', '["EL0","EL1","EL2","EL3","EL4","EL5"]',
     'Escalation level', 'direct_column', 'case.escalationLevel', FALSE),
    ('CASE', 'workaroundProvided', 'boolean', NULL,
     'Workaround provided', 'direct_column', 'case.workaroundProvided', FALSE),
    ('CASE', 'lastCommentAuthorRole', 'enum', '["SUPPORT_ENGINEER","CUSTOMER","SYSTEM"]',
     'Last comment author role', 'derived_signal', 'shared.lastCommentAuthorRole', FALSE),
    ('CASE', 'isAwaitingCustomerResponse', 'boolean', NULL,
     'Awaiting customer response', 'derived_signal', 'shared.isAwaitingCustomerResponse', FALSE),
    ('INCIDENT', 'state', 'enum',
     '["NEW","IN_PROGRESS","ON_HOLD","RESOLVED","CLOSED","CANCELED"]',
     'State', 'direct_column', 'incident.state', TRUE),
    ('INCIDENT', 'priority', 'enum', '["CRITICAL","HIGH","MODERATE","LOW"]',
     'Priority', 'direct_column', 'incident.priority', FALSE),
    ('INCIDENT', 'lastCommentAuthorRole', 'enum', '["SUPPORT_ENGINEER","CUSTOMER","SYSTEM"]',
     'Last comment author role', 'derived_signal', 'shared.lastCommentAuthorRole', FALSE),
    ('INCIDENT', 'isAwaitingCustomerResponse', 'boolean', NULL,
     'Awaiting customer response', 'derived_signal', 'shared.isAwaitingCustomerResponse', FALSE)
ON CONFLICT (work_item_type, field_key) DO NOTHING;

-- Every seeded field starts enabled -- the seed set above is exactly what a
-- first real policy set needs, so there is nothing to withhold by default;
-- an admin narrows it later from the UI if some of it should not be
-- condition-eligible after all.
INSERT INTO sla_field_enablement (field_catalog_entry_id, enabled)
SELECT id, TRUE FROM sla_field_catalog_entry
ON CONFLICT (field_catalog_entry_id) DO NOTHING;

-- Point each enabled work item type at its own state field, now that the
-- catalog rows it references exist.
UPDATE sla_work_item_type wt
SET state_field_catalog_entry_id = fc.id,
    updated_on = now(),
    updated_by = 'migration:0245'
FROM sla_field_catalog_entry fc
WHERE fc.work_item_type = wt.work_item_type
  AND fc.is_state_field
  AND wt.state_field_catalog_entry_id IS NULL;

COMMIT;
