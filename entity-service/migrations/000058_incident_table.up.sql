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

-- work_item type extension for INCIDENT, same shared-primary-key pattern as
-- case/change_request (000018_case_table.up.sql, 000047_change_request_table.up.sql):
-- id IS work_item.id, ON DELETE CASCADE, no audit columns - those live on
-- work_item and are reachable via join.
--
-- parent_incident_id and problem_id are deliberately left out for now: both
-- need a backfill pass once the tables they reference (incident itself, and
-- the not-yet-migrated problem child table) are populated. Added in
-- 000060_incident_add_parent_and_problem.up.sql.
DO $$ BEGIN
    CREATE TYPE incident_priority_enum AS ENUM ('CRITICAL', 'HIGH', 'MODERATE', 'LOW');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE incident_state_enum AS ENUM (
        'NEW', 'IN_PROGRESS', 'ON_HOLD', 'RESOLVED', 'CLOSED', 'CANCELED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Covers every category referenced by incident_subcategory below, not just
-- the smaller set the top-level "category" choice list surfaces on its own.
DO $$ BEGIN
    CREATE TYPE incident_category_enum AS ENUM (
        'NETWORK', 'DATABASE', 'HARDWARE', 'SOFTWARE', 'INQUIRY', 'SERVICE_INTERRUPTION', 'SECURITY'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE incident_impact_enum AS ENUM ('HIGH', 'MEDIUM', 'LOW');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE incident_urgency_enum AS ENUM ('HIGH', 'MEDIUM', 'LOW');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE incident_contact_type_enum AS ENUM (
        'EMAIL', 'PHONE', 'AZURE', 'SITE_24_7', 'VIRTUAL_AGENT', 'SELF_SERVICE', 'SENTINEL',
        'WALK_IN', 'EMAIL_EXTERNAL', 'EMAIL_INTERNAL', 'CHAT', 'DIRECT'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE incident_resolution_code_enum AS ENUM (
        'SOLVED_WORK_AROUND', 'SOLVED_PERMANENTLY', 'NOT_SOLVED_NOT_REPRODUCIBLE',
        'FALSE_ALARM', 'NOT_ACTIONABLE_ALERT', 'DUPLICATE'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- subcategory is a ServiceNow dependent choice list (subcategory valid values
-- depend on category) - a native enum can't express that, so it's modeled as
-- a lookup table instead, with its own surrogate id (this data isn't synced
-- from ServiceNow, so gen_random_uuid() stands in for a resolved sysid).
-- value is the raw ServiceNow choice value (lookup-by-name match target);
-- label is the display text.
CREATE TABLE IF NOT EXISTS incident_subcategory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category incident_category_enum NOT NULL,
    value TEXT NOT NULL,
    label TEXT NOT NULL,
    UNIQUE (category, value),
    -- Composite target for incident's (category, subcategory_id) FK below, so
    -- a subcategory can never be attached to an incident whose own category
    -- doesn't match the subcategory's category.
    UNIQUE (category, id)
);

INSERT INTO incident_subcategory (category, value, label) VALUES
    ('NETWORK', 'dhcp', 'DHCP'),
    ('NETWORK', 'ip address', 'IP Address'),
    ('NETWORK', 'dns', 'DNS'),
    ('NETWORK', 'vpn', 'VPN'),
    ('NETWORK', 'wireless', 'Wireless'),
    ('DATABASE', 'oracle', 'Oracle'),
    ('DATABASE', 'sql server', 'MS SQL Server'),
    ('DATABASE', 'db2', 'DB2'),
    ('HARDWARE', 'cpu', 'CPU'),
    ('HARDWARE', 'keyboard', 'Keyboard'),
    ('HARDWARE', 'memory', 'Memory'),
    ('HARDWARE', 'mouse', 'Mouse'),
    ('HARDWARE', 'disk', 'Disk'),
    ('HARDWARE', 'monitor', 'Monitor'),
    ('SOFTWARE', 'os', 'Operating System'),
    ('SOFTWARE', 'email', 'Email'),
    ('INQUIRY', 'Config Change Request', 'Config Change Request'),
    ('INQUIRY', 'Information Request', 'Information Request'),
    ('SERVICE_INTERRUPTION', 'Full Outage', 'Full Outage'),
    ('SERVICE_INTERRUPTION', 'Slowness', 'Slowness'),
    ('SERVICE_INTERRUPTION', 'Partial Outage', 'Partial Outage'),
    ('SECURITY', 'DOS/ DDOS', 'DOS/ DDOS'),
    ('SECURITY', 'Privilege escalations', 'Privilege escalations'),
    ('SECURITY', 'Threat intelligence', 'Threat intelligence'),
    ('SECURITY', 'Scans and Probes', 'Scans and Probes'),
    ('SECURITY', 'Application Security', 'Application Security'),
    ('SECURITY', 'Privacy', 'Privacy'),
    ('SECURITY', 'Data Breach', 'Data Breach'),
    ('SECURITY', 'System Compromises', 'System Compromises'),
    ('SECURITY', 'Malware', 'Malware'),
    ('SECURITY', 'Vulnerability', 'Vulnerability'),
    ('SECURITY', 'Unauthorized Access', 'Unauthorized Access'),
    ('SECURITY', 'Identity Protection', 'Identity Protection'),
    ('SECURITY', 'Phishing', 'Phishing'),
    ('SECURITY', 'Improper configuration', 'Improper configuration')
ON CONFLICT (category, value) DO NOTHING;

CREATE TABLE IF NOT EXISTS incident (
    id UUID PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    priority incident_priority_enum,
    -- Defaults mirror ServiceNow's own field defaults for a new incident.
    state incident_state_enum NOT NULL DEFAULT 'NEW',
    service_id UUID REFERENCES service(id) ON DELETE SET NULL,
    is_sla_met BOOLEAN,
    opened_on TIMESTAMPTZ,
    caller_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    category incident_category_enum,
    subcategory_id UUID,
    impact incident_impact_enum NOT NULL DEFAULT 'LOW',
    urgency incident_urgency_enum NOT NULL DEFAULT 'LOW',
    service_offering_id UUID REFERENCES service_offering(id) ON DELETE SET NULL,
    contact_type incident_contact_type_enum,
    change_request_id UUID REFERENCES change_request(id) ON DELETE SET NULL,
    caused_by_id UUID REFERENCES change_request(id) ON DELETE SET NULL,
    resolution_code incident_resolution_code_enum,
    close_notes TEXT,
    resolved_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    resolved_on TIMESTAMPTZ,
    incident_report TEXT,
    -- Both columns stay individually nullable (an incident may have no
    -- subcategory at all), but a non-null subcategory_id must carry a
    -- matching category - MATCH SIMPLE alone wouldn't catch a NULL
    -- category paired with a non-null subcategory_id, hence the CHECK.
    CONSTRAINT incident_category_subcategory_fkey FOREIGN KEY (category, subcategory_id)
        REFERENCES incident_subcategory (category, id) ON DELETE SET NULL (subcategory_id),
    CONSTRAINT incident_subcategory_requires_category
        CHECK (subcategory_id IS NULL OR category IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_incident_service_id ON incident (service_id);
CREATE INDEX IF NOT EXISTS idx_incident_subcategory_id ON incident (subcategory_id);
CREATE INDEX IF NOT EXISTS idx_incident_caller_id ON incident (caller_id);
CREATE INDEX IF NOT EXISTS idx_incident_service_offering_id ON incident (service_offering_id);
CREATE INDEX IF NOT EXISTS idx_incident_change_request_id ON incident (change_request_id);
CREATE INDEX IF NOT EXISTS idx_incident_caused_by_id ON incident (caused_by_id);
CREATE INDEX IF NOT EXISTS idx_incident_resolved_by_id ON incident (resolved_by_id);
